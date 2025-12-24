package DeviceManager

import (
	"errors"
	"sync"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

type DeviceManager struct {
	DataStore     inter.DataStore
	DeviceManager inter.DeviceManager
	timer         sync.Map
}

func NewDeviceManager(ds inter.DataStore, deviceManager inter.DeviceManager) inter.DeviceManager {
	return &DeviceManager{
		DataStore:     ds,
		DeviceManager: deviceManager,
		timer:         sync.Map{},
	}
}

func (d *DeviceManager) HandleHeartbeat(uuid string) {
	d.timer.Store(uuid, time.Now())
}

func (d *DeviceManager) QueryDeviceStatus(uuid string) (inter.DeviceStatus, error) {
	if val, ok := d.timer.Load(uuid); ok {

	} else {
		return inter.StatusOffline, errors.New("设备不存在")
	}
}
