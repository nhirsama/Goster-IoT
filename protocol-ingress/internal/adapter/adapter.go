package adapter

import (
	"context"
	"time"
)

type Adapter interface {
	Name() string
	Start(ctx context.Context) error
}

type EventSink interface {
	HandleAdapterEvent(ctx context.Context, event AdapterEvent) error
}

type CommandSink interface {
	HandleCommand(ctx context.Context, command AdapterCommand) error
}

type Identity struct {
	Type   string
	Value  string
	Issuer string
}

type Value struct {
	Number *float64
	String *string
	Bool   *bool
	Bytes  []byte
	JSON   map[string]any
}

type DeviceDescriptor struct {
	UUID            string
	Name            string
	SerialNumber    string
	MACAddress      string
	HardwareVersion string
	SoftwareVersion string
	ConfigVersion   string
	Manufacturer    string
	Vendor          string
	Model           string
	FirmwareVersion string
	DeviceType      string
	NetworkAddress  string
	Identities      []Identity
	Labels          map[string]string
	Attributes      map[string]any
}

type MetricPoint struct {
	Name             string
	Value            Value
	Unit             string
	ObservedAt       time.Time
	LegacyMetricType uint32
	Tags             map[string]string
}

type LogRecord struct {
	Level      string
	Message    string
	Namespace  string
	ObservedAt time.Time
	Fields     map[string]any
}

type CommandReceipt struct {
	CommandID           int64
	CommandUUID         string
	Status              string
	ProtocolCommandCode uint32
	Operation           string
	ErrorText           string
	ObservedAt          time.Time
	Raw                 []byte
}

type FrameInfo struct {
	MagicNumber  uint32
	CommandCode  uint32
	KeyID        uint32
	Sequence     uint64
	IsAck        bool
	IsEncrypted  bool
	IsCompressed bool
	PayloadLen   uint32
	Headers      map[string]string
}

// AdapterEvent 是 adapter 层输出给 normalizer 的协议中立事件容器。
// 具体协议的原始字段应放入 Labels、Attributes 或 Raw 中。
type AdapterEvent struct {
	EventID         string
	AdapterName     string
	ProtocolName    string
	ProtocolVersion string
	Transport       string
	RemoteAddr      string
	LocalAddr       string
	TenantHint      string
	TenantID        string
	RequestID       string
	TraceID         string
	CorrelationID   string

	Identity   Identity
	Identities []Identity
	UUID       string
	Device     *DeviceDescriptor

	Kind         string
	OccurredAt   time.Time
	ReceivedAt   time.Time
	Metrics      []MetricPoint
	Log          *LogRecord
	Availability string
	Receipt      *CommandReceipt

	Raw            []byte
	RawContentType string
	Attributes     map[string]any
	Labels         map[string]string
	Frame          FrameInfo
}

// AdapterCommand 是 normalizer 输出给具体 adapter 的执行请求。
type AdapterCommand struct {
	CommandID           int64
	CommandUUID         string
	TenantID            string
	UUID                string
	Operation           string
	ProtocolCommandCode uint32
	Target              Identity
	Payload             []byte
	PayloadContentType  string
	Properties          map[string]string
	Options             map[string]any
	Timeout             time.Duration
	MaxAttempts         int32
	Labels              map[string]string
}
