package bunrepo

import (
	"time"

	"github.com/uptrace/bun"
)

type DeviceCommandRow struct {
	bun.BaseModel `bun:"table:device_commands"`

	ID          int64      `bun:"id,pk,autoincrement"`
	TenantID    string     `bun:"tenant_id"`
	UUID        string     `bun:"uuid"`
	CmdID       int        `bun:"cmd_id"`
	Command     string     `bun:"command"`
	PayloadJSON *string    `bun:"payload_json"`
	Status      string     `bun:"status"`
	ErrorText   *string    `bun:"error_text"`
	RequestedAt time.Time  `bun:"requested_at"`
	ExecutedAt  *time.Time `bun:"executed_at"`
}
