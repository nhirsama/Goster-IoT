package IdentityManager

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

type IdentityManager struct {
	DataStore inter.DataStore
}

func NewIdentityManager(db inter.DataStore) inter.IdentityManager {
	return &IdentityManager{
		DataStore: db,
	}
}
func (i *IdentityManager) GenerateUUID(meta inter.DeviceMetadata) (uuid string) {
	sumSN := sha256.Sum256([]byte(meta.SerialNumber))
	sumMAC := sha256.Sum256([]byte(meta.MACAddress))

	combined := make([]byte, 64)
	copy(combined[:32], sumSN[:])
	copy(combined[32:], sumMAC[:])

	finalHash := sha256.Sum256(combined)

	uuid = hex.EncodeToString(finalHash[:])
	return uuid
}
func (i *IdentityManager) generateToken(uuid string) (token string) {
	milli := strconv.FormatInt(time.Now().UnixMilli(), 10)
	sumToken := sha256.Sum256([]byte(uuid + milli))
	token = "gi_" + hex.EncodeToString(sumToken[:16])
	return token
}
func (i *IdentityManager) RegisterDevice(meta inter.DeviceMetadata) (err error) {
	uuid := i.GenerateUUID(meta)
	meta.AuthenticateStatus = inter.AuthenticatePending
	meta.Token = i.generateToken(uuid) + "_invalid_token"
	err = i.DataStore.InitDevice(uuid, meta)
	return err
}

func (i *IdentityManager) UpdateDeviceAuthenticateStatus(uuid string, Status inter.AuthenticateStatusType) (token string, err error) {
	meta, err := i.DataStore.LoadConfig(uuid)
	if err != nil {
		return "", err
	}

	if meta.AuthenticateStatus == Status && Status == inter.Authenticated {
		return meta.Token, nil
	}

	switch Status {
	case inter.AuthenticatePending:
		meta.AuthenticateStatus = Status
		err = i.DataStore.SaveMetadata(uuid, meta)
		return "", i.RevokeToken(uuid)
	case inter.AuthenticateRefuse:
		meta.AuthenticateStatus = Status
		err = i.DataStore.SaveMetadata(uuid, meta)
		return "", i.RevokeToken(uuid)
	case inter.Authenticated:
		meta.Token = i.generateToken(uuid)
		meta.AuthenticateStatus = Status
		err = i.DataStore.SaveMetadata(uuid, meta)
		return meta.Token, err
	default:
		meta.AuthenticateStatus = Status
		err = i.DataStore.SaveMetadata(uuid, meta)
		return "", nil
	}
}

func (i *IdentityManager) Authenticate(token string) (uuid string, err error) {
	uuid, AuthenticateStatusType, err := i.DataStore.GetDeviceByToken(token)
	if err != nil {
		return uuid, err
	}
	switch AuthenticateStatusType {
	case inter.Authenticated:
		return uuid, err
	case inter.AuthenticateRefuse:
		return uuid, inter.ErrDeviceRefused
	case inter.AuthenticatePending:
		return uuid, inter.ErrDevicePending
	default:
		return uuid, inter.ErrDeviceUnknown
	}
}

func (i *IdentityManager) RefreshToken(uuid string) (newToken string, err error) {
	token := i.generateToken(uuid)
	return token, i.DataStore.UpdateToken(uuid, token)
}
func (i *IdentityManager) RevokeToken(uuid string) error {
	meta, err := i.DataStore.LoadConfig(uuid)
	// 生成一个无效 token 以确保唯一性约束
	token := i.generateToken(uuid) + "_invalid_token"
	if err != nil {
		return err
	}
	meta.Token = token
	meta.AuthenticateStatus = inter.AuthenticateRefuse
	err = i.DataStore.SaveMetadata(uuid, meta)
	if err != nil {
		return err
	}
	return err
}

// [Web 管理接口实现]

func (i *IdentityManager) ListDevices(page, size int) ([]inter.DeviceRecord, error) {
	return i.DataStore.ListDevices(page, size)
}

func (i *IdentityManager) GetDeviceMetadata(uuid string) (inter.DeviceMetadata, error) {
	return i.DataStore.LoadConfig(uuid)
}

func (i *IdentityManager) ApproveDevice(uuid string) error {
	_, err := i.UpdateDeviceAuthenticateStatus(uuid, inter.Authenticated)
	return err
}

func (i *IdentityManager) RejectDevice(uuid string) error {
	_, err := i.UpdateDeviceAuthenticateStatus(uuid, inter.AuthenticateRefuse)
	return err
}

func (i *IdentityManager) UnblockDevice(uuid string) error {
	_, err := i.UpdateDeviceAuthenticateStatus(uuid, inter.AuthenticatePending)
	return err
}

func (i *IdentityManager) DeleteDevice(uuid string) error {
	return i.DataStore.DestroyDevice(uuid)
}
