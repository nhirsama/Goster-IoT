package inter

import "io"

// =============================================================================
// Goster-WY 协议常量与类型定义
// =============================================================================

const (
	// MagicNumber 协议魔数 (0x5759 = "WY")
	MagicNumber uint16 = 0x5759
	// ProtocolVersion 当前协议版本
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
	CmdHandshakeInit CmdID = 0x0001
	CmdHandshakeResp CmdID = 0x0002
	CmdAuthVerify    CmdID = 0x0003
	CmdAuthAck       CmdID = 0x0004
	CmdErrorReport   CmdID = 0x00FF
)

// 上行指令 (Uplink)
const (
	CmdMetricsReport CmdID = 0x0101
	CmdLogReport     CmdID = 0x0102
	CmdEventReport   CmdID = 0x0103
)

// 下行指令 (Downlink)
const (
	CmdConfigPush CmdID = 0x0201
	CmdOtaData    CmdID = 0x0202
	CmdActionExec CmdID = 0x0203
	CmdScreenWy   CmdID = 0x0204
)

// Packet 表示一个解码的 Goster-WY 帧
type Packet struct {
	CmdID        CmdID
	KeyID        uint32
	IsAck        bool
	IsEncrypted  bool
	IsCompressed bool

	// Payload 是解密、解压后的纯净数据 (Protobuf 字节流或 Raw Binary)
	Payload []byte
}

// SessionKeyProvider 定义一个回调函数，用于根据 KeyID 获取会话密钥
// 如果返回 nil, nil，表示未找到密钥
type SessionKeyProvider func(keyID uint32) ([]byte, error)

// ProtocolCodec 定义封包与解包的接口
type ProtocolCodec interface {
	// Pack 封包：将纯净的 Payload 封装成传输用的字节流
	// payload: 业务数据 (如 Protobuf 序列化后的 bytes)
	// cmd: 指令 ID
	// keyID: 会话 ID (0 表示握手阶段)
	// sessionKey: 加密用的对称密钥 (若不需要加密则传 nil)
	// seqNonce: 用于生成 IV 的序列号或盐 (确保每次调用不同)
	Pack(payload []byte, cmd CmdID, keyID uint32, sessionKey []byte, seqNonce uint64) ([]byte, error)

	// Unpack 解包：从 Reader 中读取一帧并解密
	// reader: 数据流来源 (通常是 TCP Conn)
	// keyProvider: 用于查找解密密钥的回调函数
	Unpack(reader io.Reader, keyProvider SessionKeyProvider) (*Packet, error)
}
