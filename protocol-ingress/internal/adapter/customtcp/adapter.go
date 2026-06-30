package customtcp

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/nhirsama/Goster-IoT/proto/gen/goster/ingress/v1"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/adapter"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/config"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/coreclient"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/normalizer"
	"github.com/nhirsama/Goster-IoT/protocol-ingress/internal/protocol/gosterwy"
	"google.golang.org/protobuf/types/known/timestamppb"
)

type Adapter struct {
	cfg            config.CustomTCPConfig
	sourceInstance string
	logger         *slog.Logger
	core           coreclient.Client
	normalizer     normalizer.Normalizer
	codec          gosterwy.ProtocolCodec
	privateKey     *ecdh.PrivateKey
	connSeq        atomic.Uint64
	shutdown       atomic.Bool
	connMu         sync.Mutex
	conns          map[net.Conn]struct{}
}

func New(cfg config.CustomTCPConfig, logger *slog.Logger, deps ...Option) *Adapter {
	if logger == nil {
		logger = slog.Default()
	}
	if cfg.ReadTimeout <= 0 {
		cfg.ReadTimeout = 60 * time.Second
	}
	if cfg.RegisterAckGraceDelay <= 0 {
		cfg.RegisterAckGraceDelay = 300 * time.Millisecond
	}
	if cfg.DownlinkMaxBatch <= 0 {
		cfg.DownlinkMaxBatch = 1
	}
	privKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		panic(fmt.Sprintf("custom_tcp 生成 X25519 密钥失败: %v", err))
	}
	a := &Adapter{
		cfg:            cfg,
		sourceInstance: "protocol-ingress",
		logger:         logger,
		codec:          gosterwy.NewCodec(),
		privateKey:     privKey,
		conns:          make(map[net.Conn]struct{}),
	}
	for _, opt := range deps {
		opt(a)
	}
	logger.Info("custom_tcp X25519 密钥初始化成功", "pub_key", hex.EncodeToString(privKey.PublicKey().Bytes()))
	return a
}

type Option func(*Adapter)

func WithCoreClient(core coreclient.Client) Option {
	return func(a *Adapter) { a.core = core }
}

func WithNormalizer(n normalizer.Normalizer) Option {
	return func(a *Adapter) { a.normalizer = n }
}

func WithCodec(codec gosterwy.ProtocolCodec) Option {
	return func(a *Adapter) {
		if codec != nil {
			a.codec = codec
		}
	}
}

func WithSourceInstance(instanceID string) Option {
	return func(a *Adapter) {
		if instanceID != "" {
			a.sourceInstance = instanceID
		}
	}
}

func WithPrivateKey(key *ecdh.PrivateKey) Option {
	return func(a *Adapter) {
		if key != nil {
			a.privateKey = key
		}
	}
}

func (a *Adapter) Name() string { return "custom_tcp" }

func (a *Adapter) Start(ctx context.Context) error {
	if !a.cfg.Enabled {
		a.logger.Info("custom_tcp adapter 未启用")
		return nil
	}
	listener, err := net.Listen("tcp", a.cfg.ListenAddr)
	if err != nil {
		return fmt.Errorf("custom_tcp 监听失败: %w", err)
	}
	return a.Serve(ctx, listener)
}

func (a *Adapter) Serve(ctx context.Context, listener net.Listener) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if listener == nil {
		return errors.New("custom_tcp listener 不能为空")
	}
	defer listener.Close()

	a.shutdown.Store(false)
	a.logger.Info("custom_tcp adapter 已启动", "addr", listener.Addr().String())

	stopShutdown := make(chan struct{})
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		select {
		case <-ctx.Done():
			a.shutdown.Store(true)
			_ = listener.Close()
			a.closeActiveConnections()
		case <-stopShutdown:
		}
	}()
	defer close(stopShutdown)

	for {
		conn, err := listener.Accept()
		if err != nil {
			if ctx.Err() != nil || errors.Is(err, net.ErrClosed) {
				<-shutdownDone
				return nil
			}
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			return err
		}
		if a.shutdown.Load() {
			_ = conn.Close()
			<-shutdownDone
			return nil
		}
		a.trackConnection(conn)
		go func() {
			defer a.untrackConnection(conn)
			a.handleConnection(ctx, conn)
		}()
	}
}

func (a *Adapter) handleConnection(ctx context.Context, conn net.Conn) {
	defer conn.Close()

	connID := a.connSeq.Add(1)
	logger := a.logger.With("remote_addr", conn.RemoteAddr().String(), "conn_id", connID)
	session := newSession(a, logger, conn)
	defer session.RequeueInflight(ctx)

	var sessionKey []byte
	var writeSeq uint64

	for {
		_ = conn.SetReadDeadline(time.Now().Add(a.cfg.ReadTimeout))
		packet, err := a.codec.Unpack(conn, sessionKey)
		if err != nil {
			if a.shutdown.Load() || ctx.Err() != nil {
				return
			}
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				logger.Warn("custom_tcp 解包失败", "error", err)
			}
			return
		}

		allowed := packet.CmdID == gosterwy.CmdHandshakeInit || packet.CmdID == gosterwy.CmdAuthVerify || packet.CmdID == gosterwy.CmdDeviceRegister
		if !session.IsAuthenticated() && !allowed {
			logger.Warn("未鉴权状态下收到非法指令", "cmd_id", packet.CmdID)
			return
		}

		switch packet.CmdID {
		case gosterwy.CmdHandshakeInit:
			if len(packet.Payload) != 32 {
				logger.Warn("握手失败：公钥长度无效", "payload_len", len(packet.Payload))
				return
			}
			secret, err := a.negotiateSecret(packet.Payload)
			if err != nil {
				logger.Warn("握手失败：共享密钥协商失败", "error", err)
				return
			}
			sessionKey = secret
			writeSeq++
			respBuf, err := a.codec.Pack(a.privateKey.PublicKey().Bytes(), gosterwy.CmdHandshakeResp, 0, nil, writeSeq, true)
			if err != nil || writeAll(conn, respBuf) != nil {
				logger.Warn("写入握手响应失败", "error", err)
				return
			}
			logger.Info("握手完成：已交换密钥")

		case gosterwy.CmdAuthVerify:
			status, respPayload, err := session.Authenticate(ctx, string(packet.Payload), packet)
			if err != nil {
				logger.Warn("鉴权失败", "error", err, "token_len", len(packet.Payload))
			}
			writeSeq++
			ackBuf, err := a.codec.Pack(append([]byte{status}, respPayload...), gosterwy.CmdAuthAck, 1, sessionKey, writeSeq, true)
			if err != nil || writeAll(conn, ackBuf) != nil {
				logger.Warn("写入鉴权响应失败", "error", err)
				return
			}
			if status != 0x00 {
				return
			}
			logger = logger.With("uuid", session.UUID())
			logger.Info("鉴权成功")

		case gosterwy.CmdDeviceRegister:
			status, respPayload, err := session.Register(ctx, string(packet.Payload), packet)
			if err != nil {
				logger.Warn("设备注册处理失败", "status", status, "error", err)
			}
			writeSeq++
			ackBuf, err := a.codec.Pack(append([]byte{status}, respPayload...), gosterwy.CmdAuthAck, 1, sessionKey, writeSeq, true)
			if err != nil || writeAll(conn, ackBuf) != nil {
				logger.Warn("写入注册响应失败", "error", err)
				return
			}
			if status != 0x00 {
				if status == byte(ingressv1.RegistrationStatus_REGISTRATION_STATUS_PENDING) {
					logger.Info("设备注册待审核")
				} else {
					logger.Warn("设备注册被拒绝", "status", status)
				}
				time.Sleep(a.cfg.RegisterAckGraceDelay)
				return
			}
			logger = logger.With("uuid", session.UUID())
			logger.Info("设备注册成功")

		case gosterwy.CmdMetricsReport:
			if err := session.HandleMetrics(ctx, packet); err != nil {
				logger.Warn("指标上报处理失败", "error", err)
			}
			writeSeq++
			if err := a.writeAck(conn, gosterwy.CmdMetricsReport, sessionKey, writeSeq); err != nil {
				return
			}

		case gosterwy.CmdLogReport:
			if err := session.HandleLog(ctx, packet); err != nil {
				logger.Warn("日志上报处理失败", "error", err)
			}
			writeSeq++
			if err := a.writeAck(conn, gosterwy.CmdLogReport, sessionKey, writeSeq); err != nil {
				return
			}

		case gosterwy.CmdEventReport:
			if err := session.HandleEvent(ctx, packet); err != nil {
				logger.Warn("事件上报处理失败", "error", err)
			}
			writeSeq++
			if err := a.writeAck(conn, gosterwy.CmdEventReport, sessionKey, writeSeq); err != nil {
				return
			}

		case gosterwy.CmdHeartbeat:
			if err := session.HandleHeartbeat(ctx, packet); err != nil {
				logger.Warn("心跳处理失败", "error", err)
			}
			writeSeq++
			if err := a.writeAck(conn, gosterwy.CmdHeartbeat, sessionKey, writeSeq); err != nil {
				return
			}

		case gosterwy.CmdKeyExchangeUplink:
			secret, err := a.negotiateSecret(packet.Payload)
			if err != nil {
				logger.Warn("密钥重协商失败", "error", err)
				return
			}
			writeSeq++
			respBuf, err := a.codec.Pack(a.privateKey.PublicKey().Bytes(), gosterwy.CmdKeyExchangeDownlink, 1, sessionKey, writeSeq, true)
			if err != nil || writeAll(conn, respBuf) != nil {
				logger.Warn("写入密钥重协商响应失败", "error", err)
				return
			}
			sessionKey = secret

		case gosterwy.CmdConfigPush, gosterwy.CmdOtaData, gosterwy.CmdActionExec, gosterwy.CmdScreenWy:
			if packet.IsAck {
				session.HandleDownlinkAck(ctx, packet.CmdID, packet)
			}

		case gosterwy.CmdErrorReport:
			session.HandleError(ctx, packet)
			return

		default:
			logger.Warn("未知指令", "cmd_id", packet.CmdID)
		}

		if session.IsAuthenticated() {
			for i := 0; i < a.cfg.DownlinkMaxBatch; i++ {
				msg, ok := session.PopCommand(ctx)
				if !ok {
					break
				}
				cmdID, ok := resolveDownlinkCmd(msg)
				if !ok {
					session.FailDownlink(ctx, msg, fmt.Errorf("无法映射下行操作: %s", msg.Operation))
					continue
				}
				writeSeq++
				downlinkBuf, err := a.codec.Pack(msg.Payload, cmdID, 1, sessionKey, writeSeq, false)
				if err != nil {
					session.FailDownlink(ctx, msg, err)
					logger.Warn("下行指令打包失败", "cmd_id", cmdID, "command_id", msg.CommandID, "error", err)
					continue
				}
				if err := writeAll(conn, downlinkBuf); err != nil {
					session.RequeueDownlink(ctx, msg, err)
					logger.Warn("下行指令发送失败", "cmd_id", cmdID, "command_id", msg.CommandID, "error", err)
					return
				}
				session.MarkDownlinkSent(ctx, msg, cmdID)
				logger.Info("下行指令已发送", "cmd_id", cmdID, "command_id", msg.CommandID)
			}
		}
	}
}

func (a *Adapter) writeAck(conn net.Conn, cmd gosterwy.CmdID, sessionKey []byte, seq uint64) error {
	ackBuf, err := a.codec.Pack(nil, cmd, 1, sessionKey, seq, true)
	if err != nil {
		return err
	}
	return writeAll(conn, ackBuf)
}

func (a *Adapter) negotiateSecret(peerPubKeyBytes []byte) ([]byte, error) {
	peerPubKey, err := ecdh.X25519().NewPublicKey(peerPubKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("无效的对端公钥: %w", err)
	}
	return a.privateKey.ECDH(peerPubKey)
}

func (a *Adapter) trackConnection(conn net.Conn) {
	a.connMu.Lock()
	defer a.connMu.Unlock()
	a.conns[conn] = struct{}{}
}

func (a *Adapter) untrackConnection(conn net.Conn) {
	a.connMu.Lock()
	defer a.connMu.Unlock()
	delete(a.conns, conn)
}

func (a *Adapter) closeActiveConnections() {
	a.connMu.Lock()
	defer a.connMu.Unlock()
	for conn := range a.conns {
		_ = conn.Close()
	}
}

func writeAll(conn net.Conn, data []byte) error {
	for len(data) > 0 {
		n, err := conn.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

func resolveDownlinkCmd(command adapter.AdapterCommand) (gosterwy.CmdID, bool) {
	if command.ProtocolCommandCode != 0 {
		cmd := gosterwy.CmdID(command.ProtocolCommandCode)
		if gosterwy.IsDownlinkCommand(cmd) {
			return cmd, true
		}
	}
	if raw, ok := command.Options["command_code"]; ok {
		switch v := raw.(type) {
		case float64:
			cmd := gosterwy.CmdID(uint16(v))
			if gosterwy.IsDownlinkCommand(cmd) {
				return cmd, true
			}
		case int:
			cmd := gosterwy.CmdID(uint16(v))
			if gosterwy.IsDownlinkCommand(cmd) {
				return cmd, true
			}
		case string:
			if n, err := strconv.ParseUint(v, 0, 16); err == nil {
				cmd := gosterwy.CmdID(n)
				if gosterwy.IsDownlinkCommand(cmd) {
					return cmd, true
				}
			}
		}
	}
	return gosterwy.CommandByOperation(command.Operation)
}

func frameInfo(packet *gosterwy.Packet) adapter.FrameInfo {
	if packet == nil {
		return adapter.FrameInfo{}
	}
	return adapter.FrameInfo{
		MagicNumber:  uint32(gosterwy.MagicNumber),
		CommandCode:  uint32(packet.CmdID),
		KeyID:        packet.KeyID,
		Sequence:     packet.Sequence,
		IsAck:        packet.IsAck,
		IsEncrypted:  packet.IsEncrypted,
		IsCompressed: packet.IsCompressed,
		PayloadLen:   uint32(len(packet.Payload)),
	}
}

func (a *Adapter) ingressContext(conn net.Conn, packet *gosterwy.Packet, session *session) *ingressv1.IngressContext {
	ctx := &ingressv1.IngressContext{
		SourceInstance:  a.sourceInstance,
		AdapterId:       a.Name(),
		ProtocolName:    "goster-wy",
		ProtocolVersion: "1",
		Transport:       ingressv1.Transport_TRANSPORT_STREAM,
		ReceivedAt:      timestamppb.Now(),
		Network: &ingressv1.NetworkContext{
			RemoteAddr: conn.RemoteAddr().String(),
			LocalAddr:  conn.LocalAddr().String(),
		},
	}
	if packet != nil {
		ctx.Frame = &ingressv1.FrameContext{
			MagicNumber:  uint32(gosterwy.MagicNumber),
			CommandCode:  uint32(packet.CmdID),
			KeyId:        packet.KeyID,
			Sequence:     packet.Sequence,
			IsAck:        packet.IsAck,
			IsEncrypted:  packet.IsEncrypted,
			IsCompressed: packet.IsCompressed,
			PayloadLen:   uint32(len(packet.Payload)),
		}
	}
	if session != nil && session.tenantID != "" {
		ctx.TenantId = session.tenantID
	}
	return ctx
}

var _ adapter.Adapter = (*Adapter)(nil)
