package iot_gateway

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"time"

	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/logger"
	"github.com/nhirsama/Goster-IoT/src/protocol"
)

// gatewayService 是当前进程内的 IoT Gateway 实现。
// 它只负责网络接入、协议循环与会话管理。
type gatewayService struct {
	backend    inter.GatewayBackend
	logger     inter.Logger
	protocol   inter.ProtocolCodec
	privateKey *ecdh.PrivateKey
	connSeq    atomic.Uint64
	shutdown   atomic.Bool
	connMu     sync.Mutex
	conns      map[net.Conn]struct{}
	config     appcfg.APIConfig
}

// NewGatewayWithConfig 基于已抽象的后端接口创建网络层服务。
func NewGatewayWithConfig(backend inter.GatewayBackend, l inter.Logger, cfg appcfg.APIConfig) inter.IoTGateway {
	cfg = appcfg.NormalizeAPIConfig(cfg)
	if l == nil {
		l = logger.Default()
	}
	if backend == nil {
		panic("iot gateway backend is required")
	}

	privKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		l.Error("IoT Gateway 生成 X25519 密钥失败", inter.Err(err))
		panic(err)
	}

	l.Info("IoT Gateway X25519 密钥初始化成功", inter.String("pub_key", hex.EncodeToString(privKey.PublicKey().Bytes())))

	return &gatewayService{
		backend:    backend,
		logger:     l,
		protocol:   protocol.NewGosterCodec(),
		privateKey: privKey,
		conns:      make(map[net.Conn]struct{}),
		config:     cfg,
	}
}

// NewGateway 创建网络层服务实例。
func NewGateway(backend inter.GatewayBackend, l inter.Logger) inter.IoTGateway {
	return NewGatewayWithConfig(backend, l, appcfg.DefaultAPIConfig())
}

// NewGatewayFromCoreWithConfig 使用当前单体核心依赖创建网络层服务。
// 这层适配器是未来切换到 gRPC backend 前的过渡实现。
func NewGatewayFromCoreWithConfig(registry inter.DeviceRegistry, presence inter.DevicePresence, telemetry inter.TelemetryIngestService, downlinkCommands inter.DownlinkCommandService, l inter.Logger, cfg appcfg.APIConfig) inter.IoTGateway {
	return NewGatewayWithConfig(newLocalBackend(registry, presence, telemetry, downlinkCommands), l, cfg)
}

// NewGatewayFromCore 使用默认配置创建网络层服务。
func NewGatewayFromCore(registry inter.DeviceRegistry, presence inter.DevicePresence, telemetry inter.TelemetryIngestService, downlinkCommands inter.DownlinkCommandService, l inter.Logger) inter.IoTGateway {
	return NewGatewayFromCoreWithConfig(registry, presence, telemetry, downlinkCommands, l, appcfg.DefaultAPIConfig())
}

// Start 启动独立的 TCP 服务监听，并在 ctx 取消后主动停止监听和连接。
func (g *gatewayService) Start(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	addr := g.config.TCPAddr
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		g.logger.Error("IoT Gateway 监听失败", inter.String("addr", addr), inter.Err(err))
		return err
	}
	defer listener.Close()

	g.shutdown.Store(false)
	g.logger.Info("IoT Gateway 已启动", inter.String("addr", listener.Addr().String()))

	stopShutdown := make(chan struct{})
	shutdownDone := make(chan struct{})
	go func() {
		defer close(shutdownDone)
		select {
		case <-ctx.Done():
			g.shutdown.Store(true)
			_ = listener.Close()
			g.closeActiveConnections()
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
			g.logger.Warn("IoT Gateway 接受连接失败", inter.Err(err))
			var netErr net.Error
			if errors.As(err, &netErr) && netErr.Timeout() {
				continue
			}
			return err
		}
		g.trackConnection(conn)
		go func() {
			defer g.untrackConnection(conn)
			g.handleConnection(conn)
		}()
	}
}

// negotiateSecret 使用对方公钥协商共享密钥 (ECDH)。
func (g *gatewayService) negotiateSecret(peerPubKeyBytes []byte) ([]byte, error) {
	peerPubKey, err := ecdh.X25519().NewPublicKey(peerPubKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("无效的对端公钥: %w", err)
	}
	return g.privateKey.ECDH(peerPubKey)
}

// handleConnection 处理长连接协议循环。
func (g *gatewayService) handleConnection(conn net.Conn) {
	defer conn.Close()

	connLogger := g.logger.With(inter.String("remote_addr", conn.RemoteAddr().String()))
	connID := g.connSeq.Add(1)
	connLogger = connLogger.With(inter.Int64("conn_id", int64(connID)))

	handler := NewSessionHandler(g.backend, connLogger)
	var sessionKey []byte
	var writeSeq uint64

	for {
		conn.SetReadDeadline(time.Now().Add(g.config.ReadTimeout))

		packet, err := g.protocol.Unpack(conn, sessionKey)
		if err != nil {
			if g.shutdown.Load() {
				return
			}
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				connLogger.Warn("IoT Gateway 解包失败", inter.Err(err))
			}
			return
		}

		allowed := packet.CmdID == inter.CmdHandshakeInit ||
			packet.CmdID == inter.CmdAuthVerify ||
			packet.CmdID == inter.CmdDeviceRegister

		if !handler.IsAuthenticated() && !allowed {
			connLogger.Warn("未鉴权状态下收到非法指令", inter.Int("cmd_id", int(packet.CmdID)))
			return
		}

		switch packet.CmdID {
		case inter.CmdHandshakeInit:
			if len(packet.Payload) != 32 {
				connLogger.Warn("握手失败：公钥长度无效", inter.Int("payload_len", len(packet.Payload)))
				return
			}
			secret, err := g.negotiateSecret(packet.Payload)
			if err != nil {
				connLogger.Warn("握手失败：共享密钥协商失败", inter.Err(err))
				return
			}
			sessionKey = secret

			writeSeq++
			respBuf, _ := g.protocol.Pack(g.privateKey.PublicKey().Bytes(), inter.CmdHandshakeResp, 0, nil, writeSeq, true)
			conn.Write(respBuf)
			connLogger.Info("握手完成：已交换密钥")

		case inter.CmdAuthVerify:
			status, respPayload, err := handler.Authenticate(string(packet.Payload))
			if err != nil {
				connLogger.Warn("鉴权失败", inter.Err(err), inter.Int("token_len", len(packet.Payload)))
			}
			writeSeq++
			ackBuf, _ := g.protocol.Pack(append([]byte{status}, respPayload...), inter.CmdAuthAck, 1, sessionKey, writeSeq, true)
			conn.Write(ackBuf)

			if status != 0x00 {
				return
			}
			connLogger = connLogger.With(inter.String("uuid", handler.GetUUID()))
			connLogger.Info("鉴权成功")

		case inter.CmdDeviceRegister:
			status, respPayload, err := handler.HandleRegistration(string(packet.Payload))
			if err != nil {
				connLogger.Warn("设备注册处理失败", inter.Int("status", int(status)), inter.Err(err))
			}
			writeSeq++
			ackBuf, _ := g.protocol.Pack(append([]byte{status}, respPayload...), inter.CmdAuthAck, 1, sessionKey, writeSeq, true)
			conn.Write(ackBuf)

			if status != 0x00 {
				if status == byte(inter.RegistrationPending) {
					connLogger.Info("设备注册待审核")
				} else {
					connLogger.Warn("设备注册被拒绝", inter.Int("status", int(status)))
				}
				time.Sleep(g.config.RegisterAckGraceDelay)
				return
			}
			connLogger = connLogger.With(inter.String("uuid", handler.GetUUID()))
			connLogger.Info("设备注册成功")

		case inter.CmdMetricsReport:
			if err := handler.HandleMetrics(packet.Payload); err != nil {
				connLogger.Warn("指标上报处理失败", inter.Err(err))
			}
			writeSeq++
			ackBuf, _ := g.protocol.Pack(nil, inter.CmdMetricsReport, 1, sessionKey, writeSeq, true)
			conn.Write(ackBuf)

		case inter.CmdLogReport:
			if err := handler.HandleLog(packet.Payload); err != nil {
				connLogger.Warn("日志上报处理失败", inter.Err(err))
			}
			writeSeq++
			ackBuf, _ := g.protocol.Pack(nil, inter.CmdLogReport, 1, sessionKey, writeSeq, true)
			conn.Write(ackBuf)

		case inter.CmdEventReport:
			if err := handler.HandleEvent(packet.Payload); err != nil {
				connLogger.Warn("事件上报处理失败", inter.Err(err))
			}
			writeSeq++
			ackBuf, _ := g.protocol.Pack(nil, inter.CmdEventReport, 1, sessionKey, writeSeq, true)
			conn.Write(ackBuf)

		case inter.CmdKeyExchangeUplink:
			secret, err := g.negotiateSecret(packet.Payload)
			if err != nil {
				connLogger.Warn("密钥重协商失败", inter.Err(err))
				return
			}
			writeSeq++
			// 重协商响应仍使用当前会话密钥发送，设备收到服务端公钥后再切换到新密钥。
			respBuf, _ := g.protocol.Pack(g.privateKey.PublicKey().Bytes(), inter.CmdKeyExchangeDownlink, 1, sessionKey, writeSeq, true)
			conn.Write(respBuf)
			sessionKey = secret

		case inter.CmdConfigPush, inter.CmdOtaData, inter.CmdActionExec, inter.CmdScreenWy:
			if packet.IsAck {
				handler.HandleDownlinkAck(packet.CmdID)
			}

		case inter.CmdHeartbeat:
			if err := handler.HandleHeartbeat(); err != nil {
				connLogger.Warn("心跳处理失败", inter.Err(err))
			}
			writeSeq++
			ackBuf, _ := g.protocol.Pack(nil, inter.CmdHeartbeat, 1, sessionKey, writeSeq, true)
			conn.Write(ackBuf)

		case inter.CmdErrorReport:
			handler.HandleError(packet.Payload)
			return

		default:
			connLogger.Warn("未知指令", inter.Int("cmd_id", int(packet.CmdID)))
		}

		if handler.IsAuthenticated() {
			for {
				msg, ok := handler.PopMessage()
				if !ok {
					break
				}
				writeSeq++
				downlinkBuf, err := g.protocol.Pack(msg.Payload, msg.CmdID, 1, sessionKey, writeSeq, false)
				if err != nil {
					handler.MarkDownlinkFailed(msg, err)
					connLogger.Warn("下行指令打包失败", inter.Int("cmd_id", int(msg.CmdID)), inter.Err(err))
					continue
				}
				if _, err := conn.Write(downlinkBuf); err != nil {
					handler.MarkDownlinkFailed(msg, err)
					connLogger.Warn("下行指令发送失败", inter.Int("cmd_id", int(msg.CmdID)), inter.Err(err))
					return
				}
				handler.MarkDownlinkSent(msg)
				connLogger.Info("下行指令已发送",
					inter.Int("cmd_id", int(msg.CmdID)),
					inter.Int64("command_id", msg.CommandID))
			}
		}
	}
}

func (g *gatewayService) trackConnection(conn net.Conn) {
	g.connMu.Lock()
	defer g.connMu.Unlock()
	g.conns[conn] = struct{}{}
}

func (g *gatewayService) untrackConnection(conn net.Conn) {
	g.connMu.Lock()
	defer g.connMu.Unlock()
	delete(g.conns, conn)
}

func (g *gatewayService) closeActiveConnections() {
	g.connMu.Lock()
	defer g.connMu.Unlock()
	for conn := range g.conns {
		_ = conn.Close()
	}
}
