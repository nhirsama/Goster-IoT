package bunrepo

import (
	"database/sql"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/uptrace/bun"
)

type DeviceModel struct {
	bun.BaseModel `bun:"table:devices"`

	UUID          string         `bun:"uuid,pk"`
	TenantID      string         `bun:"tenant_id"`
	Name          string         `bun:"name"`
	HWVersion     string         `bun:"hw_version"`
	SWVersion     string         `bun:"sw_version"`
	ConfigVersion string         `bun:"config_version"`
	SerialNumber  string         `bun:"sn"`
	MACAddress    string         `bun:"mac"`
	CreatedAt     time.Time      `bun:"created_at"`
	Token         sql.NullString `bun:"token"`
	AuthStatus    int            `bun:"auth_status"`
}

func NewDeviceModel(uuid string, meta inter.DeviceMetadata) *DeviceModel {
	return &DeviceModel{
		UUID:          uuid,
		TenantID:      DefaultTenantID,
		Name:          meta.Name,
		HWVersion:     meta.HWVersion,
		SWVersion:     meta.SWVersion,
		ConfigVersion: meta.ConfigVersion,
		SerialNumber:  meta.SerialNumber,
		MACAddress:    meta.MACAddress,
		CreatedAt:     time.Now(),
		Token:         NullableToken(meta.Token),
		AuthStatus:    int(meta.AuthenticateStatus),
	}
}

func (m DeviceModel) ToMetadata() inter.DeviceMetadata {
	out := inter.DeviceMetadata{
		Name:               m.Name,
		HWVersion:          m.HWVersion,
		SWVersion:          m.SWVersion,
		ConfigVersion:      m.ConfigVersion,
		SerialNumber:       m.SerialNumber,
		MACAddress:         m.MACAddress,
		CreatedAt:          m.CreatedAt,
		AuthenticateStatus: inter.AuthenticateStatusType(m.AuthStatus),
	}
	if m.Token.Valid {
		out.Token = m.Token.String
	}
	return out
}

func (m DeviceModel) ToRecord() inter.DeviceRecord {
	return inter.DeviceRecord{
		UUID: m.UUID,
		Meta: m.ToMetadata(),
	}
}
