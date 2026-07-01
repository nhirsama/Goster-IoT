package gosterwy

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
)

type Codec struct{}

func NewCodec() ProtocolCodec { return &Codec{} }

func (c *Codec) Pack(payload []byte, cmd CmdID, keyID uint32, sessionKey []byte, seqNonce uint64, isAck bool) ([]byte, error) {
	payloadLen := len(payload)
	if payloadLen > int(MaxPayloadSize) {
		return nil, fmt.Errorf("payload 过大: %d", payloadLen)
	}

	totalSize := int(HeaderSize) + payloadLen + int(FooterSize)
	buf := make([]byte, HeaderSize, totalSize)

	var flags uint8
	if isAck {
		flags |= 0x01
	}
	isEncrypted := sessionKey != nil && keyID != 0
	if isEncrypted {
		flags |= 0x02
	}

	binary.LittleEndian.PutUint16(buf[0:], MagicNumber)
	buf[2] = ProtocolVersion
	buf[3] = flags
	binary.LittleEndian.PutUint16(buf[4:], 0)
	binary.LittleEndian.PutUint16(buf[6:], uint16(cmd))
	binary.LittleEndian.PutUint32(buf[8:], keyID)
	binary.LittleEndian.PutUint32(buf[12:], uint32(payloadLen))
	buf[16] = nonceDirection(cmd, isAck)
	binary.LittleEndian.PutUint64(buf[20:], seqNonce)

	hCRC := crc16Modbus(buf[:28])
	binary.LittleEndian.PutUint16(buf[28:], hCRC)

	if isEncrypted {
		block, err := aes.NewCipher(sessionKey)
		if err != nil {
			return nil, fmt.Errorf("AES 初始化失败: %w", err)
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("GCM 初始化失败: %w", err)
		}
		buf = gcm.Seal(buf, buf[16:28], payload, buf[:28])
		if len(buf) != totalSize {
			return nil, fmt.Errorf("加密输出大小不匹配: 期望 %d, 实际 %d", totalSize, len(buf))
		}
		return buf, nil
	}

	buf = append(buf, payload...)
	chk := crc32.NewIEEE()
	_, _ = chk.Write(buf)
	currentLen := len(buf)
	buf = append(buf, make([]byte, FooterSize)...)
	binary.LittleEndian.PutUint32(buf[currentLen:], chk.Sum32())
	return buf, nil
}

func nonceDirection(cmd CmdID, isAck bool) byte {
	switch cmd {
	case CmdHandshakeInit, CmdAuthVerify, CmdDeviceRegister, CmdKeyExchangeUplink:
		return 0x01
	case CmdHandshakeResp, CmdAuthAck, CmdKeyExchangeDownlink:
		return 0x02
	case CmdMetricsReport, CmdLogReport, CmdEventReport, CmdHeartbeat, CmdErrorReport:
		if isAck {
			return 0x02
		}
		return 0x01
	case CmdConfigPush, CmdOtaData, CmdActionExec, CmdScreenWy:
		if isAck {
			return 0x01
		}
		return 0x02
	default:
		if isAck {
			return 0x02
		}
		return 0x01
	}
}

func (c *Codec) Unpack(r io.Reader, key []byte) (*Packet, error) {
	headerBuf := make([]byte, HeaderSize)
	if _, err := io.ReadFull(r, headerBuf); err != nil {
		return nil, err
	}

	magic := binary.LittleEndian.Uint16(headerBuf[0:])
	if magic != MagicNumber {
		return nil, fmt.Errorf("无效 magic: 0x%X", magic)
	}
	expectedCRC := binary.LittleEndian.Uint16(headerBuf[28:])
	actualCRC := crc16Modbus(headerBuf[:28])
	if expectedCRC != actualCRC {
		return nil, fmt.Errorf("头部 CRC 校验失败: 期望 0x%X, 实际 0x%X", expectedCRC, actualCRC)
	}

	flags := headerBuf[3]
	cmdID := CmdID(binary.LittleEndian.Uint16(headerBuf[6:]))
	keyID := binary.LittleEndian.Uint32(headerBuf[8:])
	length := binary.LittleEndian.Uint32(headerBuf[12:])
	nonce := headerBuf[16:28]
	sequence := binary.LittleEndian.Uint64(headerBuf[20:28])
	isAck := flags&0x01 != 0
	isEncrypted := flags&0x02 != 0
	isCompressed := flags&0x04 != 0

	if length > MaxPayloadSize {
		return nil, fmt.Errorf("接收到的 payload 过大: %d", length)
	}

	bodyLen := length + FooterSize
	bodyBuf := make([]byte, bodyLen)
	if _, err := io.ReadFull(r, bodyBuf); err != nil {
		return nil, err
	}

	var finalPayload []byte
	if isEncrypted {
		if key == nil {
			return nil, errors.New("收到加密包但未提供会话密钥")
		}
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}
		plaintext, err := gcm.Open(nil, nonce, bodyBuf, headerBuf[:28])
		if err != nil {
			return nil, fmt.Errorf("解密失败: %w", err)
		}
		finalPayload = plaintext
	} else {
		rawPayload := bodyBuf[:length]
		footer := bodyBuf[length:]
		chk := crc32.NewIEEE()
		_, _ = chk.Write(headerBuf)
		_, _ = chk.Write(rawPayload)
		actualSum := chk.Sum32()
		expectedSum := binary.LittleEndian.Uint32(footer[0:])
		if actualSum != expectedSum {
			return nil, fmt.Errorf("payload CRC32 校验失败: 期望 0x%X, 实际 0x%X", expectedSum, actualSum)
		}
		finalPayload = rawPayload
	}

	return &Packet{
		CmdID:        cmdID,
		KeyID:        keyID,
		Sequence:     sequence,
		IsAck:        isAck,
		IsEncrypted:  isEncrypted,
		IsCompressed: isCompressed,
		Payload:      finalPayload,
	}, nil
}

func crc16Modbus(data []byte) uint16 {
	var crc uint16 = 0xFFFF
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&0x0001 != 0 {
				crc = (crc >> 1) ^ 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}
