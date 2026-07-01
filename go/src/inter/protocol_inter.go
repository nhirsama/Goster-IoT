package inter

// CmdID 是平台保留的设备下行协议命令编号。
// 具体帧编解码和设备连接已迁移到 protocol-ingress 微服务；
// core-api 仅保留命令编号用于下行队列、持久化和 ingress RPC 状态回填。
type CmdID uint16

// 服务端到设备的下行指令。
const (
	// CmdConfigPush 配置参数下发请求。
	CmdConfigPush CmdID = 0x0201 + iota
	// CmdOtaData OTA 固件升级数据块下发。
	CmdOtaData
	// CmdActionExec 远程控制动作执行指令。
	CmdActionExec
	// CmdScreenWy 屏幕显示或 UI 控制指令。
	CmdScreenWy
)

// DownlinkMessage 表示一个待下发给设备的指令。
type DownlinkMessage struct {
	CommandID int64
	CmdID     CmdID
	Payload   []byte
}
