package inter

import "io"

// =============================================================================
// Goster-WY 协议常量与类型定义
// =============================================================================

const (
	// MagicNumber 协议魔数 (0x5759 = "WY")
	MagicNumber uint16 = 0x5759
	// ProtocolVersion 当前协议版本号
	ProtocolVersion uint8 = 0x01
	// HeaderSize 固定头部大小 (32 Bytes)
	HeaderSize uint32 = 32
	// FooterSize 固定尾部大小 (16 Bytes)
	FooterSize uint32 = 16
)

// CmdID 指令 ID 类型别名
type CmdID uint16

// 系统指令 (System)
const (
	// CmdHandshakeInit 握手初始化指令，用于建立连接基础
	CmdHandshakeInit CmdID = 0x0001 + iota
	// CmdHandshakeResp 握手响应指令，服务端对初始化的回应
	CmdHandshakeResp
	// CmdAuthVerify 身份鉴权请求指令，设备提交 Token 进行验证
	CmdAuthVerify
	// CmdAuthAck 身份鉴权确认指令，服务端返回验证结果
	CmdAuthAck
	// CmdDeviceRegister 设备注册申请指令，无 Token 设备提交元数据
	CmdDeviceRegister
	// CmdErrorReport 错误上报指令，用于传输协议层或系统级的异常信息
	CmdErrorReport CmdID = 0x00FF
)

// 设备到服务端的上行指令 (Uplink)
const (
	// CmdMetricsReport 传感器采样指标数据上报
	CmdMetricsReport CmdID = 0x0101 + iota
	// CmdLogReport 设备运行日志上报
	CmdLogReport
	// CmdEventReport 关键事件或报警信息上报
	CmdEventReport
	// CmdHeartbeat 心跳
	CmdHeartbeat
	// CmdKeyExchangeUplink 密钥交换请求，设备上传 X25519 公钥
	CmdKeyExchangeUplink
)

// 服务端到设备的下行指令 (Downlink)
const (
	// CmdConfigPush 配置参数下发请求
	CmdConfigPush CmdID = 0x0201 + iota
	// CmdOtaData OTA 固件升级数据块下发
	CmdOtaData
	// CmdActionExec 远程控制动作执行指令
	CmdActionExec
	// CmdScreenWy 屏幕显示或 UI 控制指令
	CmdScreenWy
	// CmdKeyExchangeDownlink 密钥交换响应，服务端下发 X25519 公钥
	CmdKeyExchangeDownlink
)

// Packet 表示一个解码后的 Goster-WY 协议帧
type Packet struct {
	// CmdID 指令类型
	CmdID CmdID
	// KeyID 加密所使用的密钥 ID (0 表示未加密)
	KeyID uint32
	// IsAck 是否为确认响应包
	IsAck bool
	// IsEncrypted 数据部分是否已加密
	IsEncrypted bool
	// IsCompressed 数据部分是否已压缩
	IsCompressed bool

	// Payload 经过解密、解压后的原始业务数据
	Payload []byte
}

// DownlinkMessage 表示一个待下发给设备的指令
type DownlinkMessage struct {
	CmdID   CmdID
	Payload []byte
}

// ProtocolCodec 定义了协议封包与解包的核心接口
type ProtocolCodec interface {
	// Pack 将业务 Payload 封装为传输用的字节流
	// payload: 原始数据, cmd: 指令ID, keyID: 密钥ID, sessionKey: 加密密钥, seqNonce: 序列号/Nonce, isAck: 是否为响应包
	Pack(payload []byte, cmd CmdID, keyID uint32, sessionKey []byte, seqNonce uint64, isAck bool) ([]byte, error)

	// Unpack 从输入流中解析出一帧完整的协议包
	// reader: 数据源, key: 用于解密的对称密钥 (若未加密可传 nil)
	Unpack(reader io.Reader, key []byte) (*Packet, error)
}
