package Api

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"math"
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

// handleDownlinkAck 处理下行指令的 ACK 确认
func (a *apiImpl) handleDownlinkAck(uuid string, cmd inter.CmdID) {
	log.Printf("API: 收到下行确认 (UUID: %s, Cmd: 0x%X)", uuid, cmd)
	// TODO: 调用相关的消息队列 ACK 接口
}

// handleConnection 处理长连接协议循环
func (a *apiImpl) handleConnection(conn net.Conn) {
	defer conn.Close()

	var sessionKey []byte
	var currentUUID string
	var authenticated bool = false
	var writeSeq uint64 = 0

	for {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		// 解包 (根据是否有 sessionKey 自动处理加密/明文)
		packet, err := a.protocol.Unpack(conn, sessionKey)
		if err != nil {
			if err != io.EOF {
				log.Printf("API: 解包失败 (Remote: %s): %v", conn.RemoteAddr(), err)
			}
			return
		}

		// 权限检查：未完成鉴权前，只允许握手和鉴权指令
		if !authenticated && packet.CmdID != inter.CmdHandshakeInit && packet.CmdID != inter.CmdAuthVerify {
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
			respBuf, _ := a.protocol.Pack(a.privateKey.PublicKey().Bytes(), inter.CmdHandshakeResp, 0, nil, writeSeq)
			conn.Write(respBuf)
			log.Printf("API: 已交换密钥 (Remote: %s)", conn.RemoteAddr())

		case inter.CmdAuthVerify:
			// 设备第二帧：发送 Token (加密)
			token := string(packet.Payload)

			// 直接调用接口进行鉴权
			authUUID, err := a.identityManager.Authenticate(token)

			if err != nil {
				log.Printf("API: %v", err)
				return
			}

			status := byte(0) // Success
			if err != nil {
				status = 1 // Fail
				log.Printf("API: 鉴权失败 (Token: %s): %v", token, err)
			}

			// 发送 CmdAuthAck (加密)
			writeSeq++
			ackBuf, _ := a.protocol.Pack([]byte{status}, inter.CmdAuthAck, 1, sessionKey, writeSeq)
			conn.Write(ackBuf)

			if err != nil {
				return // 鉴权失败关闭连接
			}

			currentUUID = authUUID
			authenticated = true
			log.Printf("API: 身份鉴权通过 (UUID: %s)", currentUUID)

		case inter.CmdMetricsReport:
			// 解析并上报指标
			if len(packet.Payload) < 17 { // 基础头部长度检查
				continue
			}
			data := inter.MetricsUploadData{
				StartTimestamp: int64(binary.LittleEndian.Uint64(packet.Payload[0:8])),
				SampleInterval: binary.LittleEndian.Uint32(packet.Payload[8:12]),
				DataType:       packet.Payload[12],
				Count:          binary.LittleEndian.Uint32(packet.Payload[13:17]),
				DataBlob:       packet.Payload[17:],
			}
			a.UploadMetrics(currentUUID, data)

		case inter.CmdLogReport:
			// 解析日志上报 Payload
			// 结构: [Timestamp(8B)] + [Level(1B)] + [MsgLen(2B)] + [Message(N)]
			if len(packet.Payload) < 11 { // 8+1+2
				log.Printf("API: 日志上报数据过短")
				continue
			}

			ts := int64(binary.LittleEndian.Uint64(packet.Payload[0:8]))
			levelVal := inter.LogLevel(packet.Payload[8])
			msgLen := int(binary.LittleEndian.Uint16(packet.Payload[9:11]))

			if len(packet.Payload) < 11+msgLen {
				log.Printf("API: 日志消息体长度不足")
				continue
			}

			message := string(packet.Payload[11 : 11+msgLen])

			// 转换日志级别字符串
			var levelStr string
			switch levelVal {
			case inter.LogLevelDebug:
				levelStr = "DEBUG"
			case inter.LogLevelInfo:
				levelStr = "INFO"
			case inter.LogLevelWarn:
				levelStr = "WARN"
			case inter.LogLevelError:
				levelStr = "ERROR"
			default:
				levelStr = "UNKNOWN"
			}

			// 格式化最终消息 (附加时间戳)
			finalMsg := fmt.Sprintf("[%s] %s", time.UnixMilli(ts).Format(time.DateTime), message)
			a.dataStore.WriteLog(currentUUID, levelStr, finalMsg)
		case inter.CmdEventReport:
			log.Printf("API: 收到事件上报 (UUID: %s)", currentUUID)

		case inter.CmdConfigPush, inter.CmdOtaData, inter.CmdActionExec, inter.CmdScreenWy:
			if packet.IsAck {
				a.handleDownlinkAck(currentUUID, packet.CmdID)
			}
		case inter.CmdHeartbeat:
			a.Heartbeat(currentUUID)
		case inter.CmdErrorReport:
			log.Printf("API: 收到设备错误: %s", string(packet.Payload))
			return

		default:
			log.Printf("API: 未知指令 0x%X", packet.CmdID)
		}
	}
}

// Heartbeat 心跳处理
func (a *apiImpl) Heartbeat(uuid string) bool {
	a.deviceManager.HandleHeartbeat(uuid)
	return a.deviceManager.QueueIsEmpty(uuid)
}

// UploadMetrics 指标上报处理
func (a *apiImpl) UploadMetrics(uuid string, data inter.MetricsUploadData) error {
	log.Printf("API: 解析采样数据 (UUID: %s, Count: %d)", uuid, data.Count)

	// DataType Check (0 = Float32)
	if data.DataType != 0 {
		return fmt.Errorf("API: 不支持的指标数据类型: %d", data.DataType)
	}

	// Check DataBlob length
	pointSize := 4 // float32
	expectedLen := int(data.Count) * pointSize
	if len(data.DataBlob) != expectedLen {
		return fmt.Errorf("API: 数据长度不匹配: 期望 %d, 实际 %d", expectedLen, len(data.DataBlob))
	}

	points := make([]inter.MetricPoint, 0, data.Count)
	startTime := data.StartTimestamp
	intervalUs := int64(data.SampleInterval)

	for i := 0; i < int(data.Count); i++ {
		// Parse float32 (Little Endian)
		bits := binary.LittleEndian.Uint32(data.DataBlob[i*pointSize : (i+1)*pointSize])
		val := math.Float32frombits(bits)

		// Calculate Timestamp
		offsetMs := (int64(i) * intervalUs) / 1000
		ts := startTime + offsetMs

		points = append(points, inter.MetricPoint{
			Timestamp: ts,
			Value:     val,
		})
	}

	return a.dataStore.BatchAppendMetrics(uuid, points)
}
