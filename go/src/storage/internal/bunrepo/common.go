package bunrepo

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

const (
	DefaultTenantID = inter.DefaultTenantID
	LegacyTenantID  = inter.DefaultTenantID
)

func NullableToken(token string) sql.NullString {
	token = strings.TrimSpace(token)
	if token == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: token, Valid: true}
}

func NormalizeTenantRole(raw string) inter.TenantRole {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case string(inter.TenantRoleAdmin):
		return inter.TenantRoleAdmin
	case string(inter.TenantRoleRW):
		return inter.TenantRoleRW
	default:
		return inter.TenantRoleRO
	}
}

func PermissionToTenantRole(perm inter.PermissionType) inter.TenantRole {
	switch perm {
	case inter.PermissionAdmin:
		return inter.TenantRoleAdmin
	case inter.PermissionReadWrite:
		return inter.TenantRoleRW
	default:
		return inter.TenantRoleRO
	}
}

func NormalizeTenantID(tenantID string) string {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return DefaultTenantID
	}
	return tenantID
}

func NullableJSONString(v map[string]interface{}) (sql.NullString, error) {
	if len(v) == 0 {
		return sql.NullString{}, nil
	}
	b, err := json.Marshal(v)
	if err != nil {
		return sql.NullString{}, err
	}
	return sql.NullString{String: string(b), Valid: true}, nil
}

func ParseNullableJSONMap(raw sql.NullString) (map[string]interface{}, error) {
	if !raw.Valid || strings.TrimSpace(raw.String) == "" {
		return nil, nil
	}
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw.String), &out); err != nil {
		return nil, err
	}
	return out, nil
}

func BoolPtrToNullableInt(v *bool) sql.NullInt64 {
	if v == nil {
		return sql.NullInt64{}
	}
	if *v {
		return sql.NullInt64{Int64: 1, Valid: true}
	}
	return sql.NullInt64{Int64: 0, Valid: true}
}

func NullIntToBoolPtr(v sql.NullInt64) *bool {
	if !v.Valid {
		return nil
	}
	b := v.Int64 != 0
	return &b
}

func NullableFloat64(v *float64) sql.NullFloat64 {
	if v == nil {
		return sql.NullFloat64{}
	}
	return sql.NullFloat64{Float64: *v, Valid: true}
}

func NullableStringPtr(v *string) sql.NullString {
	if v == nil {
		return sql.NullString{}
	}
	if strings.TrimSpace(*v) == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: *v, Valid: true}
}

func PayloadStringPtr(payload []byte) *string {
	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func NullableOptionalString(v string) *string {
	v = strings.TrimSpace(v)
	if v == "" {
		return nil
	}
	return &v
}

func ExternalObservationSignature(item inter.ExternalObservation) string {
	var payload string
	switch {
	case item.ValueNum != nil:
		payload = fmt.Sprintf("n:%g|u:%s", *item.ValueNum, item.Unit)
	case item.ValueBool != nil:
		payload = fmt.Sprintf("b:%t|u:%s", *item.ValueBool, item.Unit)
	case item.ValueText != nil:
		payload = fmt.Sprintf("t:%s|u:%s", *item.ValueText, item.Unit)
	case len(item.ValueJSON) > 0:
		b, _ := json.Marshal(item.ValueJSON)
		payload = fmt.Sprintf("j:%s|u:%s", string(b), item.Unit)
	default:
		payload = fmt.Sprintf("empty|u:%s", item.Unit)
	}
	sum := sha256.Sum256([]byte(payload))
	return hex.EncodeToString(sum[:8])
}

func nullableFloatOut(v sql.NullFloat64) *float64 {
	if !v.Valid {
		return nil
	}
	out := v.Float64
	return &out
}

func nullableStringOut(v sql.NullString) *string {
	if !v.Valid {
		return nil
	}
	out := v.String
	return &out
}
