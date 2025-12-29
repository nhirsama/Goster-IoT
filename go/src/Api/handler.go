package Api

import (
	"encoding/binary"
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// BusinessHandler 处理应用层业务逻辑 (Per Session)
type BusinessHandler struct {
	dataStore       inter.DataStore
	deviceManager   inter.DeviceManager
	identityManager inter.IdentityManager

	// Session State
	uuid          string
	authenticated bool
}

// NewBusinessHandler 创建业务逻辑处理器
func NewBusinessHandler(ds inter.DataStore, dm inter.DeviceManager, im inter.IdentityManager) *BusinessHandler {
	return &BusinessHandler{
		dataStore:       ds,
		deviceManager:   dm,
		identityManager: im,
		authenticated:   false,
	}
}

// IsAuthenticated 检查当前会话是否已鉴权
func (h *BusinessHandler) IsAuthenticated() bool {
	return h.authenticated
}

// GetUUID 获取当前会话的 UUID
func (h *BusinessHandler) GetUUID() string {
	return h.uuid
}

// Authenticate 处理 Token 鉴权 (Cmd: 0x0003)
func (h *BusinessHandler) Authenticate(token string) (byte, []byte, error) {
	uuid, err := h.identityManager.Authenticate(token)
	if err != nil {
		// 鉴权失败，返回 0x01 (Fail)
		return 0x01, nil, err
	}
	h.uuid = uuid
	h.authenticated = true
	// 鉴权成功，返回 0x00 (Success)
	return 0x00, nil, nil
}

// HandleRegistration 处理设备注册申请 (Cmd: 0x0005)
func (h *BusinessHandler) HandleRegistration(payload string) (byte, []byte, error) {
	parts := strings.Split(payload, "\x1e")
	if len(parts) < 6 {
		return 0x01, nil, fmt.Errorf("registration payload invalid: expected 6 fields, got %d", len(parts))
	}

	meta := inter.DeviceMetadata{
		Name:          parts[0],
		SerialNumber:  parts[1],
		MACAddress:    parts[2],
		HWVersion:     parts[3],
		SWVersion:     parts[4],
		ConfigVersion: parts[5],
		// Token: 尚未生成
		// CreatedAt: 将由 DataStore 处理
	}

	// 1. 生成/计算 UUID
	uuid := h.identityManager.GenerateUUID(meta)

	// 2. 查询设备状态
	existingMeta, err := h.dataStore.LoadConfig(uuid)

	if err != nil {
		// 设备不存在 -> 首次注册
		log.Printf("API: 新设备注册申请 (UUID: %s, SN: %s)", uuid, meta.SerialNumber)

		//meta.AuthenticateStatus = inter.AuthenticatePending

		if err := h.identityManager.RegisterDevice(meta); err != nil {
			return 0x01, nil, fmt.Errorf("init device failed: %w", err)
		}

		// 返回 0x02 (Pending)
		return 0x02, nil, nil
	}

	// 设备已存在 -> 检查状态
	switch existingMeta.AuthenticateStatus {
	case inter.AuthenticatePending:
		// 仍在审核中 -> 0x02
		return 0x02, nil, nil

	case inter.AuthenticateRefuse:
		// 已被拒绝 -> 0x01
		return 0x01, nil, fmt.Errorf("device registration refused")

	case inter.Authenticated:
		// 已通过 -> 返回 Token，允许接入 -> 0x00
		h.uuid = uuid
		h.authenticated = true
		return 0x00, []byte(existingMeta.Token), nil

	default:
		return 0x01, nil, fmt.Errorf("unknown auth status")
	}
}

// HandleHeartbeat 处理心跳包
func (h *BusinessHandler) HandleHeartbeat() bool {
	if !h.authenticated {
		return false
	}
	h.deviceManager.HandleHeartbeat(h.uuid)
	return h.deviceManager.QueueIsEmpty(h.uuid)
}

// HandleMetrics 处理传感器数据上报
func (h *BusinessHandler) HandleMetrics(payload []byte) error {
	if !h.authenticated {
		return fmt.Errorf("unauthorized")
	}

	if len(payload) < 17 {
		return fmt.Errorf("payload too short")
	}

	data := inter.MetricsUploadData{
		StartTimestamp: int64(binary.LittleEndian.Uint64(payload[0:8])),
		SampleInterval: binary.LittleEndian.Uint32(payload[8:12]),
		DataType:       payload[12],
		Count:          binary.LittleEndian.Uint32(payload[13:17]),
		DataBlob:       payload[17:],
	}

	log.Printf("API: 解析采样数据 (UUID: %s, Count: %d)", h.uuid, data.Count)

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

	return h.dataStore.BatchAppendMetrics(h.uuid, points)
}

// HandleLog 处理日志上报
func (h *BusinessHandler) HandleLog(payload []byte) error {
	if !h.authenticated {
		return fmt.Errorf("unauthorized")
	}
	// 结构: [Timestamp(8B)] + [Level(1B)] + [MsgLen(2B)] + [Message(N)]
	if len(payload) < 11 { // 8+1+2
		return fmt.Errorf("log payload too short")
	}

	ts := int64(binary.LittleEndian.Uint64(payload[0:8]))
	levelVal := inter.LogLevel(payload[8])
	msgLen := int(binary.LittleEndian.Uint16(payload[9:11]))

	if len(payload) < 11+msgLen {
		return fmt.Errorf("log message truncated")
	}

	message := string(payload[11 : 11+msgLen])

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
	return h.dataStore.WriteLog(h.uuid, levelStr, finalMsg)
}

// HandleDownlinkAck 处理下行指令确认
func (h *BusinessHandler) HandleDownlinkAck(cmd inter.CmdID) {
	if !h.authenticated {
		return
	}
	log.Printf("API: 收到下行确认 (UUID: %s, Cmd: 0x%X)", h.uuid, cmd)
	// TODO: 调用相关的消息队列 ACK 接口
}

// HandleEvent 处理事件上报
func (h *BusinessHandler) HandleEvent(payload []byte) {
	if !h.authenticated {
		return
	}
	log.Printf("API: 收到事件上报 (UUID: %s)", h.uuid)
}

// HandleError 处理错误上报
func (h *BusinessHandler) HandleError(payload []byte) {
	log.Printf("API: 收到设备错误: %s", string(payload))
}
