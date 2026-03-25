package runtime

import (
	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/storage/command"
	"github.com/nhirsama/Goster-IoT/src/storage/device"
	"github.com/nhirsama/Goster-IoT/src/storage/external"
	"github.com/nhirsama/Goster-IoT/src/storage/internal/bunrepo"
	"github.com/nhirsama/Goster-IoT/src/storage/telemetry"
	"github.com/nhirsama/Goster-IoT/src/storage/tenant"
	"github.com/nhirsama/Goster-IoT/src/storage/user"
)

// Store 组合各个业务域的存储子模块，作为统一运行时存储层入口。
// 每个业务模块依赖自己的 repo 接口，数据库后端实现由存储层内部组合。
type Store struct {
	base *bunrepo.Store

	*device.Repository
	telemetryRepo *telemetry.Repository
	commandRepo   *command.Repository
	externalRepo  *external.Repository
	userRepo      *user.Repository
	tenantRepo    *tenant.Repository
}

var (
	_ inter.DeviceRepository         = (*Store)(nil)
	_ inter.ScopedDeviceRepository   = (*Store)(nil)
	_ inter.MetricsRepository        = (*Store)(nil)
	_ inter.DeviceLogRepository      = (*Store)(nil)
	_ inter.DeviceCommandRepository  = (*Store)(nil)
	_ inter.ExternalEntityRepository = (*Store)(nil)
	_ inter.UserRepository           = (*Store)(nil)
	_ inter.TenantRoleRepository     = (*Store)(nil)
	_ inter.CoreStore                = (*Store)(nil)
	_ inter.WebV1Store               = (*Store)(nil)
)

func OpenSQLite(path string) (*Store, error) {
	base, err := bunrepo.OpenSQLite(path)
	if err != nil {
		return nil, err
	}
	return newStore(base), nil
}

func OpenPostgres(dsn string) (*Store, error) {
	base, err := bunrepo.OpenPostgres(dsn)
	if err != nil {
		return nil, err
	}
	return newStore(base), nil
}

func newStore(base *bunrepo.Store) *Store {
	deviceRepo := device.NewRepository(base.DB)
	telemetryRepo := telemetry.NewWithDevice(base.DB, deviceRepo)
	commandRepo := command.NewRepository(base.DB, deviceRepo)
	externalRepo := external.NewRepository(base.DB)
	userRepo := user.NewRepository(base.DB)
	tenantRepo := tenant.NewRepository(base.DB)
	return &Store{
		base:          base,
		Repository:    deviceRepo,
		telemetryRepo: telemetryRepo,
		commandRepo:   commandRepo,
		externalRepo:  externalRepo,
		userRepo:      userRepo,
		tenantRepo:    tenantRepo,
	}
}

func (s *Store) Close() error {
	if s == nil || s.base == nil {
		return nil
	}
	return s.base.Close()
}

func (s *Store) AppendMetric(uuid string, point inter.MetricPoint) error {
	return s.telemetryRepo.AppendMetric(uuid, point)
}

func (s *Store) BatchAppendMetrics(uuid string, points []inter.MetricPoint) error {
	return s.telemetryRepo.BatchAppendMetrics(uuid, points)
}

func (s *Store) QueryMetrics(uuid string, start, end int64) ([]inter.MetricPoint, error) {
	return s.telemetryRepo.QueryMetrics(uuid, start, end)
}

func (s *Store) QueryMetricsByTenant(tenantID, uuid string, start, end int64) ([]inter.MetricPoint, error) {
	return s.telemetryRepo.QueryMetricsByTenant(tenantID, uuid, start, end)
}

func (s *Store) WriteLog(uuid string, level string, message string) error {
	return s.telemetryRepo.WriteLog(uuid, level, message)
}

func (s *Store) CreateDeviceCommand(uuid string, cmdID inter.CmdID, command string, payloadJSON []byte) (int64, error) {
	return s.commandRepo.CreateDeviceCommand(uuid, cmdID, command, payloadJSON)
}

func (s *Store) CreateDeviceCommandByTenant(tenantID, uuid string, cmdID inter.CmdID, command string, payloadJSON []byte) (int64, error) {
	return s.commandRepo.CreateDeviceCommandByTenant(tenantID, uuid, cmdID, command, payloadJSON)
}

func (s *Store) UpdateDeviceCommandStatus(commandID int64, status inter.DeviceCommandStatus, errorText string) error {
	return s.commandRepo.UpdateDeviceCommandStatus(commandID, status, errorText)
}

func (s *Store) UpsertExternalEntity(entity inter.ExternalEntity) error {
	return s.externalRepo.UpsertExternalEntity(entity)
}

func (s *Store) GetExternalEntity(source, entityID string) (inter.ExternalEntity, error) {
	return s.externalRepo.GetExternalEntity(source, entityID)
}

func (s *Store) ListExternalEntities(source, domain string, limit, offset int) ([]inter.ExternalEntity, error) {
	return s.externalRepo.ListExternalEntities(source, domain, limit, offset)
}

func (s *Store) BatchAppendExternalObservations(items []inter.ExternalObservation) error {
	return s.externalRepo.BatchAppendExternalObservations(items)
}

func (s *Store) QueryExternalObservations(source, entityID string, start, end int64, limit int) ([]inter.ExternalObservation, error) {
	return s.externalRepo.QueryExternalObservations(source, entityID, start, end, limit)
}

func (s *Store) GetUserCount() (int, error) {
	return s.userRepo.GetUserCount()
}

func (s *Store) ListUsers() ([]inter.User, error) {
	return s.userRepo.ListUsers()
}

func (s *Store) GetUserPermission(username string) (inter.PermissionType, error) {
	return s.userRepo.GetUserPermission(username)
}

func (s *Store) UpdateUserPermission(username string, perm inter.PermissionType) error {
	return s.userRepo.UpdateUserPermission(username, perm)
}

func (s *Store) GetUserTenantRoles(username string) (map[string]inter.TenantRole, error) {
	return s.tenantRepo.GetUserTenantRoles(username)
}
