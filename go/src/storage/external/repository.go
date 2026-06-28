package external

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/storage/internal/bunrepo"
	"github.com/uptrace/bun"
)

type Repository struct {
	db *bun.DB
}

func NewRepository(db *bun.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) UpsertExternalEntity(entity inter.ExternalEntity) error {
	source := strings.TrimSpace(entity.Source)
	entityID := strings.TrimSpace(entity.EntityID)
	domain := strings.TrimSpace(entity.Domain)
	if source == "" || entityID == "" || domain == "" {
		return errors.New("source/entity_id/domain is required")
	}
	entity.Source = source
	entity.EntityID = entityID
	entity.Domain = domain
	if strings.TrimSpace(entity.ValueType) == "" {
		entity.ValueType = "string"
	}

	row, err := bunrepo.NewExternalEntityRow(entity)
	if err != nil {
		return err
	}
	_, err = r.db.NewInsert().
		Model(row).
		On("CONFLICT (source, entity_id) DO UPDATE").
		Set("domain = EXCLUDED.domain").
		Set("goster_uuid = EXCLUDED.goster_uuid").
		Set("device_id = EXCLUDED.device_id").
		Set("model = EXCLUDED.model").
		Set("name = EXCLUDED.name").
		Set("room_name = EXCLUDED.room_name").
		Set("unit = EXCLUDED.unit").
		Set("value_type = EXCLUDED.value_type").
		Set("device_class = EXCLUDED.device_class").
		Set("state_class = EXCLUDED.state_class").
		Set("attributes_json = EXCLUDED.attributes_json").
		Set("last_state_text = EXCLUDED.last_state_text").
		Set("last_state_num = EXCLUDED.last_state_num").
		Set("last_state_bool = EXCLUDED.last_state_bool").
		Set("last_seen_ts = EXCLUDED.last_seen_ts").
		Set("updated_at = CURRENT_TIMESTAMP").
		Returning("NULL").
		Exec(context.Background())
	return err
}

func (r *Repository) GetExternalEntity(source, entityID string) (inter.ExternalEntity, error) {
	var row bunrepo.ExternalEntityRow
	err := r.db.NewSelect().
		Model(&row).
		Where("source = ?", source).
		Where("entity_id = ?", entityID).
		Limit(1).
		Scan(context.Background())
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return inter.ExternalEntity{}, errors.New("external entity not found")
		}
		return inter.ExternalEntity{}, err
	}
	return row.ToExternalEntity()
}

func (r *Repository) ListExternalEntities(source, domain string, limit, offset int) ([]inter.ExternalEntity, error) {
	if limit <= 0 {
		limit = 100
	}
	if offset < 0 {
		offset = 0
	}

	query := r.db.NewSelect().
		Model((*bunrepo.ExternalEntityRow)(nil)).
		OrderExpr("last_seen_ts DESC, id DESC").
		Limit(limit).
		Offset(offset)
	if strings.TrimSpace(source) != "" {
		query = query.Where("source = ?", strings.TrimSpace(source))
	}
	if strings.TrimSpace(domain) != "" {
		query = query.Where("domain = ?", strings.TrimSpace(domain))
	}

	var rows []bunrepo.ExternalEntityRow
	if err := query.Scan(context.Background(), &rows); err != nil {
		return nil, err
	}

	out := make([]inter.ExternalEntity, 0, len(rows))
	for _, row := range rows {
		item, err := row.ToExternalEntity()
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}

func (r *Repository) BatchAppendExternalObservations(items []inter.ExternalObservation) error {
	if len(items) == 0 {
		return nil
	}

	rows := make([]bunrepo.ExternalObservationRow, 0, len(items))
	now := time.Now().UnixMilli()
	for _, item := range items {
		source := strings.TrimSpace(item.Source)
		entityID := strings.TrimSpace(item.EntityID)
		if source == "" || entityID == "" {
			return errors.New("source/entity_id is required")
		}
		item.Source = source
		item.EntityID = entityID
		if item.Timestamp <= 0 {
			item.Timestamp = now
		}
		if strings.TrimSpace(item.ValueSig) == "" {
			item.ValueSig = bunrepo.ExternalObservationSignature(item)
		}
		row, err := bunrepo.NewExternalObservationRow(item)
		if err != nil {
			return err
		}
		rows = append(rows, *row)
	}

	_, err := r.db.NewInsert().
		Model(&rows).
		On("CONFLICT (source, entity_id, ts, value_sig) DO NOTHING").
		Returning("NULL").
		Exec(context.Background())
	return err
}

func (r *Repository) QueryExternalObservations(source, entityID string, start, end int64, limit int) ([]inter.ExternalObservation, error) {
	if end <= 0 {
		end = time.Now().UnixMilli()
	}
	if start <= 0 || start > end {
		start = end - int64(24*time.Hour/time.Millisecond)
	}
	if limit <= 0 {
		limit = 1000
	}

	var rows []bunrepo.ExternalObservationRow
	err := r.db.NewSelect().
		Model(&rows).
		Where("source = ?", strings.TrimSpace(source)).
		Where("entity_id = ?", strings.TrimSpace(entityID)).
		Where("ts BETWEEN ? AND ?", start, end).
		OrderExpr("ts ASC").
		Limit(limit).
		Scan(context.Background())
	if err != nil {
		return nil, err
	}

	out := make([]inter.ExternalObservation, 0, len(rows))
	for _, row := range rows {
		item, err := row.ToExternalObservation()
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, nil
}
