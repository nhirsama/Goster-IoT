package bunrepo

import (
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/uptrace/bun"
)

type MetricRow struct {
	bun.BaseModel `bun:"table:metrics"`

	UUID     string  `bun:"uuid"`
	TenantID string  `bun:"tenant_id"`
	TS       int64   `bun:"ts"`
	Value    float32 `bun:"value"`
	Type     uint8   `bun:"type"`
}

type LogRow struct {
	bun.BaseModel `bun:"table:logs"`

	ID        int64     `bun:"id,pk,autoincrement"`
	UUID      string    `bun:"uuid"`
	TenantID  string    `bun:"tenant_id"`
	Level     string    `bun:"level"`
	Message   string    `bun:"message"`
	CreatedAt time.Time `bun:"created_at"`
}

func ToMetricPoints(rows []MetricRow) []inter.MetricPoint {
	out := make([]inter.MetricPoint, 0, len(rows))
	for _, row := range rows {
		out = append(out, inter.MetricPoint{
			Timestamp: row.TS,
			Value:     row.Value,
			Type:      row.Type,
		})
	}
	return out
}
