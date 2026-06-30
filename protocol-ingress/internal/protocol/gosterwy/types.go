package gosterwy

import "io"

const (
	// MagicNumber 协议魔数 (0x5759 = "WY")。
	MagicNumber uint16 = 0x5759
	// ProtocolVersion 当前协议版本号。
	ProtocolVersion uint8 = 0x01
	// HeaderSize 固定头部大小。
	HeaderSize uint32 = 32
	// FooterSize 固定尾部大小；明文模式为 CRC32 + padding，加密模式承载 GCM Tag。
	FooterSize uint32 = 16
	// MaxPayloadSize 限制单帧载荷，避免异常设备撑爆内存。
	MaxPayloadSize uint32 = 1 * 1024 * 1024
)

type CmdID uint16

// 系统指令。
const (
	CmdHandshakeInit CmdID = 0x0001 + iota
	CmdHandshakeResp
	CmdAuthVerify
	CmdAuthAck
	CmdDeviceRegister
	CmdErrorReport CmdID = 0x00FF
)

// 设备到服务端的上行指令。
const (
	CmdMetricsReport CmdID = 0x0101 + iota
	CmdLogReport
	CmdEventReport
	CmdHeartbeat
	CmdKeyExchangeUplink
)

// 服务端到设备的下行指令。
const (
	CmdConfigPush CmdID = 0x0201 + iota
	CmdOtaData
	CmdActionExec
	CmdScreenWy
	CmdKeyExchangeDownlink
)

type Packet struct {
	CmdID        CmdID
	KeyID        uint32
	Sequence     uint64
	IsAck        bool
	IsEncrypted  bool
	IsCompressed bool
	Payload      []byte
}

type ProtocolCodec interface {
	Pack(payload []byte, cmd CmdID, keyID uint32, sessionKey []byte, seqNonce uint64, isAck bool) ([]byte, error)
	Unpack(reader io.Reader, key []byte) (*Packet, error)
}

func IsDownlinkCommand(cmd CmdID) bool {
	switch cmd {
	case CmdConfigPush, CmdOtaData, CmdActionExec, CmdScreenWy:
		return true
	default:
		return false
	}
}

func CommandName(cmd CmdID) string {
	switch cmd {
	case CmdConfigPush:
		return "config_push"
	case CmdOtaData:
		return "ota_data"
	case CmdActionExec:
		return "action_exec"
	case CmdScreenWy:
		return "screen_wy"
	case CmdHeartbeat:
		return "heartbeat"
	case CmdMetricsReport:
		return "metrics"
	case CmdLogReport:
		return "log"
	case CmdEventReport:
		return "event"
	case CmdErrorReport:
		return "error"
	default:
		return "unknown"
	}
}

func CommandByOperation(operation string) (CmdID, bool) {
	switch operation {
	case "config_push", "config", "set_config":
		return CmdConfigPush, true
	case "ota_data", "ota":
		return CmdOtaData, true
	case "action_exec", "action", "exec":
		return CmdActionExec, true
	case "screen_wy", "screen":
		return CmdScreenWy, true
	default:
		return 0, false
	}
}
