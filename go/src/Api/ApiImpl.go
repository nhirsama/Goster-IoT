package Api

import (
	"encoding/binary"
	"io"
	"log"
	"net"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

type apiImpl struct {
	dataStore       inter.DataStore
	deviceManager   inter.DeviceManager
	identityManager inter.IdentityManager
}

// NewApi 创建 API 服务实例
func NewApi(ds inter.DataStore, dm inter.DeviceManager, im inter.IdentityManager) inter.Api {
	return &apiImpl{
		dataStore:       ds,
		deviceManager:   dm,
		identityManager: im,
	}
}

// Start 启动独立的 TCP 服务监听 (Goster-WY 协议)
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

// handleConnection 处理长连接协议循环
func (a *apiImpl) handleConnection(conn net.Conn) {
	defer conn.Close()

	// 临时保存当前连接的设备 UUID (握手后获得)
	var currentUUID string

	for {
		conn.SetReadDeadline(time.Now().Add(60 * time.Second))

		header := make([]byte, 11)
		if _, err := io.ReadFull(conn, header); err != nil {
			if err != io.EOF {
				log.Printf("API: 读取头部失败: %v", err)
			}
			return
		}

		if header[0] != 0x57 || header[1] != 0x59 {
			log.Printf("API: 无效魔数 %x %x", header[0], header[1])
			return
		}

		cmdID := header[4]
		length := binary.BigEndian.Uint32(header[7:11])

		var payload []byte
		if length > 0 {
			if length > 1024*1024 { // 限制 1MB 防止内存攻击
				log.Printf("API: 数据包过大 (%d 字节)", length)
				return
			}
			payload = make([]byte, length)
			if _, err := io.ReadFull(conn, payload); err != nil {
				log.Printf("API: 读取 Payload 失败: %v", err)
				return
			}
		}

		switch cmdID {
		case 0x01: // Heartbeat
			if currentUUID != "" {
				a.Heartbeat(currentUUID)
				// TODO: 发送响应响应头包含 Flags (PendingMsg)
			}
		case 0x02: // Metrics
			// TODO: 解析请求并发送到数据库
		case 0x03: // Log
			if currentUUID != "" {
				a.UploadLog(currentUUID, "INFO", string(payload))
			}
		case 0x00: // 握手
		default:
			log.Printf("API: 未知指令 0x%02x", cmdID)
		}
	}
}

// Handshake 实现身份鉴权
func (a *apiImpl) Handshake(uuid string, token string) (string, error) {
	authenticatedUUID, err := a.identityManager.Authenticate(token)
	if err != nil {
		log.Printf("API: 握手失败 (Token: %s): %v", token, err)
		return "", err
	}
	log.Printf("API: 设备握手成功 UUID=%s", authenticatedUUID)
	return authenticatedUUID, nil
}

// Heartbeat 更新设备状态并检查消息
func (a *apiImpl) Heartbeat(uuid string) (bool, error) {
	a.deviceManager.HandleHeartbeat(uuid)
	log.Printf("API: 收到心跳 UUID=%s", uuid)
	// TODO: 实际应检查消息队列返回 hasPending
	return false, nil
}

// UploadMetrics 持久化传感器数据
func (a *apiImpl) UploadMetrics(uuid string, data inter.MetricsUploadData) error {
	// 实际应解析 data.DataBlob 为 MetricPoint 数组
	log.Printf("API: 收到采样数据 UUID=%s, 长度=%d", uuid, len(data.DataBlob))
	return nil
}

// UploadLog 写入设备日志
func (a *apiImpl) UploadLog(uuid string, level string, message string) error {
	err := a.dataStore.WriteLog(uuid, level, message)
	if err != nil {
		log.Printf("API: 写入日志失败: %v", err)
	}
	return err
}

// GetMessages 获取下行指令
func (a *apiImpl) GetMessages(uuid string) ([]interface{}, error) {
	// TODO: 从消息队列中弹出所有消息
	return nil, nil
}
