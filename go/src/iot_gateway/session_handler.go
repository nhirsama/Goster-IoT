package iot_gateway

import (
	"errors"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/logger"
)

// SessionHandler 负责单个设备连接的会话状态与消息调度。
// 它只处理协议会话内的状态，不直接依赖具体存储实现或设备管理兼容层。
type SessionHandler struct {
	backend inter.GatewayBackend
	logger  inter.Logger

	uuid                string
	authenticated       bool
	pendingDownlinkAcks map[inter.CmdID][]int64
}

// NewSessionHandler 创建单连接会话处理器。
func NewSessionHandler(backend inter.GatewayBackend, l inter.Logger) *SessionHandler {
	if l == nil {
		l = logger.Default()
	}
	return &SessionHandler{
		backend:             backend,
		logger:              l,
		pendingDownlinkAcks: make(map[inter.CmdID][]int64),
	}
}

// IsAuthenticated 检查当前会话是否已鉴权。
func (h *SessionHandler) IsAuthenticated() bool {
	return h.authenticated
}

// GetUUID 获取当前会话对应的设备 UUID。
func (h *SessionHandler) GetUUID() string {
	return h.uuid
}

// Authenticate 处理 Token 鉴权 (Cmd: 0x0003)。
func (h *SessionHandler) Authenticate(token string) (byte, []byte, error) {
	uuid, err := h.backend.AuthenticateDevice(token)
	if err != nil {
		return 0x01, nil, err
	}
	h.bindDevice(uuid)
	return 0x00, nil, nil
}

// HandleRegistration 处理设备注册申请 (Cmd: 0x0005)。
func (h *SessionHandler) HandleRegistration(payload string) (byte, []byte, error) {
	meta, err := parseRegistrationPayload(payload)
	if err != nil {
		return 0x01, nil, err
	}

	result, err := h.backend.RegisterDevice(meta)
	if err != nil && result.Status == inter.RegistrationAccepted {
		return byte(result.Status), []byte(result.Token), err
	}
	if result.Status == inter.RegistrationAccepted {
		h.bindDevice(result.UUID)
		return byte(result.Status), []byte(result.Token), nil
	}
	return byte(result.Status), nil, err
}

// HandleHeartbeat 处理设备心跳。
func (h *SessionHandler) HandleHeartbeat() error {
	if !h.authenticated {
		return errors.New("unauthorized")
	}
	return h.backend.ReportHeartbeat(h.uuid)
}

// HandleMetrics 处理传感器数据上报。
func (h *SessionHandler) HandleMetrics(payload []byte) error {
	if !h.authenticated {
		return errors.New("unauthorized")
	}

	points, err := parseMetricsPayload(payload)
	if err != nil {
		return err
	}
	h.logger.Debug("开始解析指标数据", inter.String("uuid", h.uuid), inter.Int("count", len(points)))
	return h.backend.ReportMetrics(h.uuid, points)
}

// HandleLog 处理日志上报。
func (h *SessionHandler) HandleLog(payload []byte) error {
	if !h.authenticated {
		return errors.New("unauthorized")
	}

	data, err := parseLogPayload(payload)
	if err != nil {
		return err
	}
	return h.backend.ReportLog(h.uuid, data)
}

// HandleDownlinkAck 处理下行指令确认。
func (h *SessionHandler) HandleDownlinkAck(cmd inter.CmdID) {
	if !h.authenticated {
		return
	}
	h.logger.Debug("收到下行确认", inter.String("uuid", h.uuid), inter.Int("cmd_id", int(cmd)))

	pending := h.pendingDownlinkAcks[cmd]
	if len(pending) == 0 {
		h.logger.Warn("收到无法匹配的下行确认", inter.String("uuid", h.uuid), inter.Int("cmd_id", int(cmd)))
		return
	}

	commandID := pending[0]
	h.pendingDownlinkAcks[cmd] = pending[1:]
	if len(h.pendingDownlinkAcks[cmd]) == 0 {
		delete(h.pendingDownlinkAcks, cmd)
	}

	if err := h.backend.MarkDownlinkAcked(commandID); err != nil {
		h.logger.Warn("下行确认状态落库失败",
			inter.String("uuid", h.uuid),
			inter.Int("cmd_id", int(cmd)),
			inter.Int64("command_id", commandID),
			inter.Err(err))
	}
}

// PopMessage 获取并弹出一个待处理的下行消息。
func (h *SessionHandler) PopMessage() (inter.DownlinkMessage, bool) {
	if !h.authenticated {
		return inter.DownlinkMessage{}, false
	}
	msg, ok, err := h.backend.PopDownlink(h.uuid)
	if err != nil {
		h.logger.Warn("拉取下行消息失败", inter.String("uuid", h.uuid), inter.Err(err))
		return inter.DownlinkMessage{}, false
	}
	return msg, ok
}

// MarkDownlinkSent 记录已发送的下行消息，用于 ACK 回填状态。
func (h *SessionHandler) MarkDownlinkSent(msg inter.DownlinkMessage) {
	if !h.authenticated || msg.CommandID <= 0 {
		return
	}

	h.pendingDownlinkAcks[msg.CmdID] = append(h.pendingDownlinkAcks[msg.CmdID], msg.CommandID)
	if err := h.backend.MarkDownlinkSent(msg.CommandID); err != nil {
		h.logger.Warn("下行发送状态落库失败",
			inter.String("uuid", h.uuid),
			inter.Int("cmd_id", int(msg.CmdID)),
			inter.Int64("command_id", msg.CommandID),
			inter.Err(err))
	}
}

// MarkDownlinkFailed 标记下行消息发送失败。
func (h *SessionHandler) MarkDownlinkFailed(msg inter.DownlinkMessage, err error) {
	if !h.authenticated || msg.CommandID <= 0 {
		return
	}

	errorText := ""
	if err != nil {
		errorText = err.Error()
	}
	if updateErr := h.backend.MarkDownlinkFailed(msg.CommandID, errorText); updateErr != nil {
		h.logger.Warn("下行失败状态落库失败",
			inter.String("uuid", h.uuid),
			inter.Int("cmd_id", int(msg.CmdID)),
			inter.Int64("command_id", msg.CommandID),
			inter.Err(updateErr))
	}
}

// HandleEvent 处理事件上报。
func (h *SessionHandler) HandleEvent(payload []byte) error {
	if !h.authenticated {
		return errors.New("unauthorized")
	}
	h.logger.Info("收到事件上报", inter.String("uuid", h.uuid))
	return h.backend.ReportEvent(h.uuid, payload)
}

// HandleError 处理设备错误上报。
func (h *SessionHandler) HandleError(payload []byte) {
	h.logger.Warn("收到设备错误上报", inter.String("message", string(payload)))
	if !h.authenticated {
		return
	}
	if err := h.backend.ReportDeviceError(h.uuid, payload); err != nil {
		h.logger.Warn("设备错误落库失败", inter.String("uuid", h.uuid), inter.Err(err))
	}
}

func (h *SessionHandler) bindDevice(uuid string) {
	h.uuid = uuid
	h.authenticated = true
	h.logger = h.logger.With(inter.String("uuid", uuid))
}
