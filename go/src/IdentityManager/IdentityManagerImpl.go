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

func NewIdentityManager(ds inter.DataStore) inter.IdentityManager {
	return &IdentityManager{
		DataStore: ds,
	}
}
func (i IdentityManager) GenerateUUID(meta inter.DeviceMetadata) (uuid string) {
	sumSN := sha256.Sum256([]byte(meta.SerialNumber))
	sumMAC := sha256.Sum256([]byte(meta.MACAddress))

	combined := make([]byte, 64)
	copy(combined[:32], sumSN[:])
	copy(combined[32:], sumMAC[:])

	finalHash := sha256.Sum256(combined)

	uuid = string(finalHash[:])
	return uuid
}
func (i IdentityManager) generateToken(uuid string) (token string) {
	milli := strconv.FormatInt(time.Now().UnixMilli(), 10)
	sumToken := sha256.Sum256([]byte(uuid + milli))
	token = "gi_" + hex.EncodeToString(sumToken[:16])
	return token
}
func (i IdentityManager) RegisterDevice(uuid string, meta inter.DeviceMetadata) (token string, err error) {
	token = i.generateToken(uuid)
	meta.Token = token
	err = i.DataStore.InitDevice(uuid, meta)
	return token, err
}

func (i IdentityManager) Authenticate(token string) (uuid string, err error) {
	return i.DataStore.GetDeviceByToken(token)
}

func (i IdentityManager) RefreshToken(uuid string) (newToken string, err error) {
	token := i.generateToken(uuid)
	return token, i.DataStore.UpdateToken(uuid, token)
}
func (i IdentityManager) RevokeToken(uuid string) error {
	token := i.generateToken(uuid)
	// 删除 token 哪有直接生成一个新 token 来的方便
	return i.DataStore.UpdateToken(uuid, token+"_invalid_token")
}
