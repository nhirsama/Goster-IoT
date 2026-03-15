package api

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"sync/atomic"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/logger"
	"github.com/nhirsama/Goster-IoT/src/protocol"
)

type apiImpl struct {
	dataStore     inter.DataStore
	deviceManager inter.DeviceManager
	logger        inter.Logger
	protocol      inter.ProtocolCodec
	privateKey    *ecdh.PrivateKey // X25519 私钥
	connSeq       atomic.Uint64
}

// NewApi 创建 API 服务实例
func NewApi(ds inter.DataStore, dm inter.DeviceManager, l inter.Logger) inter.Api {
	if l == nil {
		l = logger.Default()
	}
	privKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		l.Error("api generate x25519 key failed", inter.Err(err))
		panic(err)
	}

	l.Info("api x25519 key initialized", inter.String("pub_key", hex.EncodeToString(privKey.PublicKey().Bytes())))

	return &apiImpl{
		dataStore:     ds,
		deviceManager: dm,
		logger:        l,
		protocol:      protocol.NewGosterCodec(),
		privateKey:    privKey,
	}
}

// Start 启动独立的 TCP 服务监听
func (a *apiImpl) Start() {
	addr := ":8081"
	l, err := net.Listen("tcp", addr)
	if err != nil {
		a.logger.Error("api listen failed", inter.String("addr", addr), inter.Err(err))
		panic(err)
	}
	a.logger.Info("api server started", inter.String("addr", addr))

	for {
		conn, err := l.Accept()
		if err != nil {
			a.logger.Warn("api accept failed", inter.Err(err))
			continue
		}
		go a.handleConnection(conn)
	}
}

// negotiateSecret 使用对方公钥协商共享密钥 (ECDH)
func (a *apiImpl) negotiateSecret(peerPubKeyBytes []byte) ([]byte, error) {
	peerPubKey, err := ecdh.X25519().NewPublicKey(peerPubKeyBytes)
	if err != nil {
		return nil, fmt.Errorf("无效的对端公钥: %w", err)
	}
	return a.privateKey.ECDH(peerPubKey)
}

// handleConnection 处理长连接协议循环
func (a *apiImpl) handleConnection(conn net.Conn) {
	defer conn.Close()

	connLogger := a.logger.With(inter.String("remote_addr", conn.RemoteAddr().String()))
	connID := a.connSeq.Add(1)
	connLogger = connLogger.With(inter.Int64("conn_id", int64(connID)))

	// 为当前会话创建独立的业务逻辑处理器 (Application Layer)
	handler := NewBusinessHandler(a.dataStore, a.deviceManager, connLogger)

	var sessionKey []byte
	var writeSeq uint64 = 0

	for {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// 解包 (根据是否有 sessionKey 自动处理加密/明文)
		packet, err := a.protocol.Unpack(conn, sessionKey)
		if err != nil {
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				connLogger.Warn("api unpack failed", inter.Err(err))
			}
			return
		}

		// 权限检查：未完成鉴权前，只允许握手、鉴权和注册指令
		allowed := packet.CmdID == inter.CmdHandshakeInit ||
			packet.CmdID == inter.CmdAuthVerify ||
			packet.CmdID == inter.CmdDeviceRegister

		if !handler.IsAuthenticated() && !allowed {
			connLogger.Warn("api command rejected before auth", inter.Int("cmd_id", int(packet.CmdID)))
			return
		}

		switch packet.CmdID {
		case inter.CmdHandshakeInit:
			// 设备第一帧：上传其公钥 (32字节)
			if len(packet.Payload) != 32 {
				connLogger.Warn("api handshake invalid pubkey length", inter.Int("payload_len", len(packet.Payload)))
				return
			}
			secret, err := a.negotiateSecret(packet.Payload)
			if err != nil {
				connLogger.Warn("api handshake negotiate secret failed", inter.Err(err))
				return
			}
			sessionKey = secret

			// 回复服务端公钥 (明文)
			writeSeq++
			respBuf, _ := a.protocol.Pack(a.privateKey.PublicKey().Bytes(), inter.CmdHandshakeResp, 0, nil, writeSeq, true)
			conn.Write(respBuf)
			connLogger.Info("api handshake exchanged key")

		case inter.CmdAuthVerify:
			// Token 鉴权 (0x0003)
			token := string(packet.Payload)

			// 调用业务层鉴权
			status, respPayload, err := handler.Authenticate(token)

			if err != nil {
				connLogger.Warn("api auth failed", inter.Err(err), inter.Int("token_len", len(token)))
			}

			// 发送 CmdAuthAck (加密)
			ackPayload := append([]byte{status}, respPayload...)
			writeSeq++
			ackBuf, _ := a.protocol.Pack(ackPayload, inter.CmdAuthAck, 1, sessionKey, writeSeq, true)
			conn.Write(ackBuf)

			if status != 0x00 {
				return // 鉴权失败关闭连接
			}
			connLogger = connLogger.With(inter.String("uuid", handler.GetUUID()))
			connLogger.Info("api auth success")

		case inter.CmdDeviceRegister:
			// 设备注册 (0x0005)
			payloadStr := string(packet.Payload)

			// 调用业务层注册
			status, respPayload, err := handler.HandleRegistration(payloadStr)

			if err != nil {
				connLogger.Warn("api register handler failed", inter.Int("status", int(status)), inter.Err(err))
			}

			// 发送 CmdAuthAck (加密)
			ackPayload := append([]byte{status}, respPayload...)
			writeSeq++
			ackBuf, _ := a.protocol.Pack(ackPayload, inter.CmdAuthAck, 1, sessionKey, writeSeq, true)
			conn.Write(ackBuf)

			if status != 0x00 {
				if status == 0x02 {
					connLogger.Info("api register pending")
				} else {
					connLogger.Warn("api register rejected", inter.Int("status", int(status)))
				}
				time.Sleep(100 * time.Millisecond) // 给客户端留出读取响应的时间
				return                             // 关闭连接
			}
			connLogger = connLogger.With(inter.String("uuid", handler.GetUUID()))
			connLogger.Info("api register success")

		case inter.CmdMetricsReport:
			if err := handler.HandleMetrics(packet.Payload); err != nil {
				connLogger.Warn("api metrics handle failed", inter.Err(err))
			}
			// 发送通用 ACK
			writeSeq++
			ackBuf, _ := a.protocol.Pack(nil, inter.CmdMetricsReport, 1, sessionKey, writeSeq, true)
			conn.Write(ackBuf)

		case inter.CmdLogReport:
			if err := handler.HandleLog(packet.Payload); err != nil {
				connLogger.Warn("api log handle failed", inter.Err(err))
			}
			// 发送通用 ACK
			writeSeq++
			ackBuf, _ := a.protocol.Pack(nil, inter.CmdLogReport, 1, sessionKey, writeSeq, true)
			conn.Write(ackBuf)

		case inter.CmdEventReport:
			handler.HandleEvent(packet.Payload)
			// 发送通用 ACK
			writeSeq++
			ackBuf, _ := a.protocol.Pack(nil, inter.CmdEventReport, 1, sessionKey, writeSeq, true)
			conn.Write(ackBuf)

		case inter.CmdKeyExchangeUplink:
			// 密钥重协商
			secret, err := a.negotiateSecret(packet.Payload)
			if err != nil {
				connLogger.Warn("api key re-exchange failed", inter.Err(err))
				return
			}
			sessionKey = secret
			writeSeq++
			respBuf, _ := a.protocol.Pack(a.privateKey.PublicKey().Bytes(), inter.CmdKeyExchangeDownlink, 1, sessionKey, writeSeq, true)
			conn.Write(respBuf)

		case inter.CmdConfigPush, inter.CmdOtaData, inter.CmdActionExec, inter.CmdScreenWy:
			if packet.IsAck {
				handler.HandleDownlinkAck(packet.CmdID)
			}
		case inter.CmdHeartbeat:
			handler.HandleHeartbeat()
			// 发送通用 ACK
			writeSeq++
			ackBuf, _ := a.protocol.Pack(nil, inter.CmdHeartbeat, 1, sessionKey, writeSeq, true)
			conn.Write(ackBuf)

		case inter.CmdErrorReport:
			handler.HandleError(packet.Payload)
			return

		default:
			connLogger.Warn("api unknown cmd", inter.Int("cmd_id", int(packet.CmdID)))
		}

		// --- 检查并处理下行消息 ---
		if handler.IsAuthenticated() {
			for {
				cmdID, downlinkPayload, ok := handler.PopMessage()
				if !ok {
					break
				}
				writeSeq++
				downlinkBuf, err := a.protocol.Pack(downlinkPayload, cmdID, 1, sessionKey, writeSeq, false)
				if err == nil {
					conn.Write(downlinkBuf)
					connLogger.Info("api downlink sent", inter.Int("cmd_id", int(cmdID)))
				}
			}
		}
	}
}
