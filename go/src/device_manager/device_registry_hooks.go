package device_manager

// DeviceRegistryHooks 描述设备生命周期变化时需要触发的运行时副作用。
// 当前主要用于在删除设备后清理在线状态，后续也可扩展为分布式事件通知。
type DeviceRegistryHooks struct {
	OnDelete func(uuid string)
}
