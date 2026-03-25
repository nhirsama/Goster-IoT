package core

import (
	appcfg "github.com/nhirsama/Goster-IoT/src/config"
	"github.com/nhirsama/Goster-IoT/src/device_manager"
	"github.com/nhirsama/Goster-IoT/src/inter"
)

// Services 代表核心业务层对外暴露的一组标准服务。
// 启动入口和测试夹具优先从这里取依赖，避免继续散落地手工拼装对象。
type Services struct {
	DeviceRegistry   inter.DeviceRegistry
	DevicePresence   inter.DevicePresence
	ExternalEntities inter.ExternalEntityService
	TelemetryIngest  inter.TelemetryIngestService
	DownlinkQueue    inter.DeviceCommandQueue
	DownlinkCommands inter.DownlinkCommandService
}

// NewServices 使用默认配置构建核心服务集合。
func NewServices(ds inter.CoreStore) Services {
	return NewServicesWithConfig(ds, appcfg.DefaultDeviceManagerConfig())
}

// NewServicesWithConfig 按给定配置构建核心服务集合。
func NewServicesWithConfig(ds inter.CoreStore, cfg appcfg.DeviceManagerConfig) Services {
	if ds == nil {
		panic("core services require datastore")
	}

	n := appcfg.NormalizeDeviceManagerConfig(cfg)
	presence := device_manager.NewDevicePresenceWithStore(n.HeartbeatDeadline, device_manager.NewInMemoryDevicePresenceStore())
	registry := device_manager.NewDeviceRegistryWithHooks(ds, device_manager.DeviceRegistryHooks{
		OnDelete: presence.RemoveDevice,
	})
	queue := device_manager.NewDeviceCommandQueue(n.QueueCapacity)

	return Services{
		DeviceRegistry:   registry,
		DevicePresence:   presence,
		ExternalEntities: device_manager.NewExternalEntityService(ds, n),
		TelemetryIngest:  device_manager.NewTelemetryIngestService(ds),
		DownlinkQueue:    queue,
		DownlinkCommands: device_manager.NewDownlinkCommandService(ds, queue),
	}
}
