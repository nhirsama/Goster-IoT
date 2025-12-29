package Api

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/protocol"
)

type apiImpl struct {
	dataStore       inter.DataStore
	deviceManager   inter.DeviceManager
	identityManager inter.IdentityManager
	protocol        inter.ProtocolCodec
	privateKey      *ecdh.PrivateKey // X25519 私钥
}

// NewApi 创建 API 服务实例
func NewApi(ds inter.DataStore, dm inter.DeviceManager, im inter.IdentityManager) inter.Api {
	privKey, err := ecdh.X25519().GenerateKey(rand.Reader)
	if err != nil {
		log.Fatalf("API: 生成 X25519 密钥对失败: %v", err)
	}

	log.Printf("API: 初始化 X25519 密钥成功, 公钥: %s", hex.EncodeToString(privKey.PublicKey().Bytes()))

	return &apiImpl{
		dataStore:       ds,
		deviceManager:   dm,
		identityManager: im,
		protocol:        protocol.NewGosterCodec(),
		privateKey:      privKey,
	}
}

// Start 启动独立的 TCP 服务监听
func (a *apiImpl) Start() {
	addr := ":8081"
	l, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatalf("API 服务无法监听端口 %s: %v", addr, err)
	}
	log.Printf("正在启动 API 服务 (Goster-WY) 于 %s", addr)

	for {
		conn, err := l.Accept()
		if err != nil {
			log.Printf("API 连接接收错误: %v", err)
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

	// 为当前会话创建独立的业务逻辑处理器 (Application Layer)
	handler := NewBusinessHandler(a.dataStore, a.deviceManager, a.identityManager)

	var sessionKey []byte
	var writeSeq uint64 = 0

	for {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// 解包 (根据是否有 sessionKey 自动处理加密/明文)
		packet, err := a.protocol.Unpack(conn, sessionKey)
		if err != nil {
			if err != io.EOF && err != io.ErrUnexpectedEOF {
				log.Printf("API: 解包失败 (Remote: %s): %v", conn.RemoteAddr(), err)
			}
			return
		}

		// 权限检查：未完成鉴权前，只允许握手、鉴权和注册指令
		allowed := packet.CmdID == inter.CmdHandshakeInit ||
			packet.CmdID == inter.CmdAuthVerify ||
			packet.CmdID == inter.CmdDeviceRegister

		if !handler.IsAuthenticated() && !allowed {
			log.Printf("API: 拒绝非法指令 0x%X (未鉴权)", packet.CmdID)
			return
		}

		switch packet.CmdID {
		case inter.CmdHandshakeInit:
			// 设备第一帧：上传其公钥 (32字节)
			if len(packet.Payload) != 32 {
				log.Printf("API: 握手失败，无效的公钥长度")
				return
			}
			secret, err := a.negotiateSecret(packet.Payload)
			if err != nil {
				log.Printf("API: 密钥协商失败: %v", err)
				return
			}
			sessionKey = secret

			// 回复服务端公钥 (明文)
			writeSeq++
			respBuf, _ := a.protocol.Pack(a.privateKey.PublicKey().Bytes(), inter.CmdHandshakeResp, 0, nil, writeSeq, true)
			conn.Write(respBuf)
			log.Printf("API: 已交换密钥 (Remote: %s)", conn.RemoteAddr())

		case inter.CmdAuthVerify:
			// Token 鉴权 (0x0003)
			token := string(packet.Payload)

			// 调用业务层鉴权
			status, respPayload, err := handler.Authenticate(token)

			if err != nil {
				log.Printf("API: 鉴权失败 (Token: %s): %v", token, err)
			}

			// 发送 CmdAuthAck (加密)
			ackPayload := append([]byte{status}, respPayload...)
			writeSeq++
			ackBuf, _ := a.protocol.Pack(ackPayload, inter.CmdAuthAck, 1, sessionKey, writeSeq, true)
			conn.Write(ackBuf)

			if status != 0x00 {
				return // 鉴权失败关闭连接
			}
			log.Printf("API: 身份鉴权通过 (UUID: %s)", handler.GetUUID())

		case inter.CmdDeviceRegister:
			// 设备注册 (0x0005)
			payloadStr := string(packet.Payload)

			// 调用业务层注册
			status, respPayload, err := handler.HandleRegistration(payloadStr)

			if err != nil {
				log.Printf("API: 注册处理异常 (Status: %d): %v", status, err)
			}

			// 发送 CmdAuthAck (加密)
			ackPayload := append([]byte{status}, respPayload...)
			writeSeq++
			ackBuf, _ := a.protocol.Pack(ackPayload, inter.CmdAuthAck, 1, sessionKey, writeSeq, true)
			conn.Write(ackBuf)

			if status != 0x00 {
				if status == 0x02 {
					log.Printf("API: 设备注册申请已提交 (Pending), 关闭连接")
				} else {
					log.Printf("API: 鉴权/注册被拒绝 (Status: %d), 关闭连接", status)
				}
				time.Sleep(100 * time.Millisecond) // 给客户端留出读取响应的时间
				return                             // 关闭连接
			}
			log.Printf("API: 设备注册成功并自动鉴权 (UUID: %s)", handler.GetUUID())

		case inter.CmdMetricsReport:
			if err := handler.HandleMetrics(packet.Payload); err != nil {
				log.Printf("API: Metrics error: %v", err)
			}
			// 发送通用 ACK
			writeSeq++
			ackBuf, _ := a.protocol.Pack(nil, inter.CmdMetricsReport, 1, sessionKey, writeSeq, true)
			conn.Write(ackBuf)

		case inter.CmdLogReport:
			if err := handler.HandleLog(packet.Payload); err != nil {
				log.Printf("API: Log error: %v", err)
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
				log.Printf("API: 密钥重协商失败: %v", err)
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
			log.Printf("API: 未知指令 0x%X", packet.CmdID)
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
					log.Printf("API: 下发指令 0x%X 到设备 %s", cmdID, handler.GetUUID())
				}
			}
		}
	}
}
