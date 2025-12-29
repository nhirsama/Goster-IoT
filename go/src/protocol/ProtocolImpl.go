package protocol

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// GosterCodec 实现 inter.ProtocolCodec 接口
type GosterCodec struct{}

// NewGosterCodec 创建一个新的编解码器实例
func NewGosterCodec() inter.ProtocolCodec {
	return &GosterCodec{}
}

var crc16Table = []uint16{
	0x0000, 0xC0C1, 0xC181, 0x0140, 0xC301, 0x03C0, 0x0280, 0xC241,
	0xC601, 0x06C0, 0x0780, 0xC741, 0x0500, 0xC5C1, 0xC481, 0x0440,
	0xCC01, 0x0CC0, 0x0D80, 0xCD41, 0x0F00, 0xCFC1, 0xCE81, 0x0E40,
	0x0A00, 0xCAC1, 0xCB81, 0x0B40, 0xC901, 0x09C0, 0x0880, 0xC841,
	0xD801, 0x18C0, 0x1980, 0xD941, 0x1B00, 0xDBC0, 0xDA80, 0x1A41,
	0x1E00, 0xDEC1, 0xDF81, 0x1F40, 0xDD01, 0x1DC0, 0x1C80, 0xDC41,
	0x1400, 0xD4C1, 0xD581, 0x1540, 0xD701, 0x17C0, 0x1680, 0xD641,
	0xD201, 0x12C0, 0x1380, 0xD341, 0x1100, 0xD1C1, 0xD081, 0x1040,
	0xF001, 0x30C0, 0x3180, 0xF141, 0x3300, 0xF3C1, 0xF281, 0x3240,
	0x3600, 0xF6C1, 0xF781, 0x3740, 0xF501, 0x35C0, 0x3480, 0xF441,
	0x3C00, 0xFCC1, 0xFD81, 0x3D40, 0xFF01, 0x3FC0, 0x3E80, 0xFE41,
	0xFA01, 0x3AC0, 0x3B80, 0xFB41, 0x3900, 0xF9C1, 0xF881, 0x3840,
	0x2800, 0xE8C1, 0xE981, 0x2940, 0xEB01, 0x2BC0, 0x2A80, 0xEA41,
	0xEE01, 0x2EC0, 0x2F80, 0xEF41, 0x2D00, 0xEDC1, 0xEC81, 0x2C40,
	0xE401, 0x24C0, 0x2580, 0xE541, 0x2700, 0xE7C1, 0xE681, 0x2640,
	0x2200, 0xE2C1, 0xE381, 0x2340, 0xE101, 0x21C0, 0x2080, 0xE041,
	0xA001, 0x60C0, 0x6180, 0xA141, 0x6300, 0xA3C1, 0xA281, 0x6240,
	0x6600, 0xA6C1, 0xA781, 0x6740, 0xA501, 0x65C0, 0x6480, 0xA441,
	0x6C00, 0xACC1, 0xAD81, 0x6D40, 0xAF01, 0x6FC0, 0x6E80, 0xAE41,
	0xAA01, 0x6AC0, 0x6B80, 0xAB41, 0x6900, 0xA9C1, 0xA881, 0x6840,
	0x7800, 0xB8C1, 0xB981, 0x7940, 0xBB01, 0xBBC0, 0xBA80, 0x7A41,
	0xBE01, 0x7EC0, 0x7F80, 0xBF41, 0x7D00, 0xBDC1, 0xBC81, 0x7C40,
	0xB401, 0x74C0, 0x7580, 0xB541, 0x7700, 0xB7C1, 0xB681, 0x7640,
	0x7200, 0xB2C1, 0xB381, 0x7340, 0xB101, 0x71C0, 0x7081, 0xB041,
	0x5000, 0x90C1, 0x9181, 0x5140, 0x9301, 0x53C0, 0x5280, 0x9241,
	0x9601, 0x56C0, 0x5780, 0x9741, 0x5500, 0x95C1, 0x9481, 0x5440,
	0x9C01, 0x5CC0, 0x5D80, 0x9D41, 0x5F00, 0x9FC1, 0x9E81, 0x5E40,
	0x5A00, 0x9AC1, 0x9B81, 0x5B40, 0x9901, 0x59C0, 0x5880, 0x9841,
	0x8801, 0x48C0, 0x4980, 0x8941, 0x4B00, 0x8BC0, 0x8A80, 0x4A41,
	0x4E00, 0x8EC1, 0x8F81, 0x4F40, 0x8D01, 0x4DC0, 0x4C80, 0x8C41,
	0x4400, 0x84C1, 0x8581, 0x4540, 0x8701, 0x47C0, 0x4680, 0x8641,
	0x8201, 0x42C0, 0x4380, 0x8340, 0x4100, 0x81C1, 0x8081, 0x4040,
}

func crc16Modbus(data []byte) uint16 {
	var crc uint16 = 0xFFFF
	for _, b := range data {
		crc = (crc >> 8) ^ crc16Table[(crc^uint16(b))&0xFF]
	}
	return crc
}

func (c *GosterCodec) Pack(payload []byte, cmd inter.CmdID, keyID uint32, sessionKey []byte, seqNonce uint64, isAck bool) ([]byte, error) {
	payloadLen := len(payload)
	if payloadLen > 1*1024*1024 { // 1MB 限制
		return nil, fmt.Errorf("payload过大: %d", payloadLen)
	}

	totalSize := int(inter.HeaderSize) + payloadLen + int(inter.FooterSize)

	// 初始长度为HeaderSize用于填充头部，容量为totalSize用于追加Payload/Footer
	buf := make([]byte, inter.HeaderSize, totalSize)

	var flags uint8 = 0
	if isAck {
		flags |= 0x01 // Bit 0: ACK
	}
	isEncrypted := sessionKey != nil && keyID != 0
	if isEncrypted {
		flags |= 0x02 // Bit 1: 加密
	}

	// 填充头部 (Offset 0-31)
	binary.LittleEndian.PutUint16(buf[0:], inter.MagicNumber)
	buf[2] = inter.ProtocolVersion
	buf[3] = flags
	binary.LittleEndian.PutUint16(buf[4:], 0) // Status
	binary.LittleEndian.PutUint16(buf[6:], uint16(cmd))
	binary.LittleEndian.PutUint32(buf[8:], keyID)
	binary.LittleEndian.PutUint32(buf[12:], uint32(payloadLen))

	// Nonce: Salt(4B) + Seq(8B) 在 Offset 16
	if _, err := io.ReadFull(rand.Reader, buf[16:20]); err != nil {
		return nil, fmt.Errorf("生成随机Salt失败: %w", err)
	}
	binary.LittleEndian.PutUint64(buf[20:], seqNonce)

	// 计算 Header CRC16 (Offset 0-27)
	// 覆盖前 28 字节 (Magic, Ver, Flags, Status, Cmd, Key, Len, Nonce)
	hCrc := crc16Modbus(buf[:28])
	binary.LittleEndian.PutUint16(buf[28:], hCrc)
	// buf[30:32] 是填充位，已为 0

	// 加密或追加 Payload
	if isEncrypted {
		block, err := aes.NewCipher(sessionKey)
		if err != nil {
			return nil, fmt.Errorf("AES初始化失败: %w", err)
		}

		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("GCM初始化失败: %w", err)
		}

		// Nonce 在 buf[16:28]
		nonce := buf[16:28]
		// AAD 是 buf[:28] (不含CRC/Padding的头部)
		aad := buf[:28]

		// 加密并追加到 buf
		// gcm.Seal 追加 (ciphertext + tag) 到 dst
		// 此时 buf 长度为 32，追加后长度为 32 + len(payload) + 16
		buf = gcm.Seal(buf, nonce, payload, aad)

		// 校验最终大小
		if len(buf) != totalSize {
			return nil, fmt.Errorf("加密输出大小不匹配: 期望 %d, 实际 %d", totalSize, len(buf))
		}

	} else {
		// 明文模式
		// 追加 Payload
		buf = append(buf, payload...)

		// 计算 CRC32 (Header + Payload)
		chk := crc32.NewIEEE()
		chk.Write(buf)
		sum := chk.Sum32()

		// 追加 Footer (CRC32 + Padding)
		// Footer 共 16 字节，前 4 字节为 CRC32，其余为 0
		currentLen := len(buf)
		buf = append(buf, make([]byte, inter.FooterSize)...)
		binary.LittleEndian.PutUint32(buf[currentLen:], sum)
	}

	return buf, nil
}

func (c *GosterCodec) Unpack(r io.Reader, key []byte) (*inter.Packet, error) {
	// 读取 Header (32 Bytes)
	headerBuf := make([]byte, inter.HeaderSize)
	if _, err := io.ReadFull(r, headerBuf); err != nil {
		return nil, err
	}

	magic := binary.LittleEndian.Uint16(headerBuf[0:])
	if magic != inter.MagicNumber {
		return nil, fmt.Errorf("无效Magic: 0x%X", magic)
	}

	// 验证 Header CRC16
	expectedCRC := binary.LittleEndian.Uint16(headerBuf[28:])
	actualCRC := crc16Modbus(headerBuf[:28])
	if expectedCRC != actualCRC {
		return nil, fmt.Errorf("头部CRC校验失败: 期望 0x%X, 实际 0x%X", expectedCRC, actualCRC)
	}

	// 解析 Header
	flags := headerBuf[3]
	cmdID := inter.CmdID(binary.LittleEndian.Uint16(headerBuf[6:]))
	keyID := binary.LittleEndian.Uint32(headerBuf[8:])
	length := binary.LittleEndian.Uint32(headerBuf[12:]) // Payload 长度
	nonce := headerBuf[16:28]                            // 12 Bytes

	isEncrypted := (flags & 0x02) != 0
	isAck := (flags & 0x01) != 0

	if length > 1*1024*1024 {
		return nil, fmt.Errorf("接收到的Payload过大: %d", length)
	}

	// 读取 Payload + Footer (一次性读取)
	// Body = Payload (length) + Footer (16)
	bodyLen := length + inter.FooterSize
	bodyBuf := make([]byte, bodyLen)
	if _, err := io.ReadFull(r, bodyBuf); err != nil {
		return nil, err
	}

	var finalPayload []byte

	if isEncrypted {
		// 查找密钥
		if key == nil {
			return nil, errors.New("收到加密包但未提供 KeyProvider")
		}

		// 初始化 AES-GCM
		block, err := aes.NewCipher(key)
		if err != nil {
			return nil, err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}

		// AAD (Header 前 28 字节)
		headerAAD := headerBuf[:28]

		// 解密
		// bodyBuf 包含 [EncryptedPayload... | Tag(16)]
		// 这正好符合 gcm.Open 对 ciphertext 的要求 (ciphertext + tag)
		plaintext, err := gcm.Open(nil, nonce, bodyBuf, headerAAD)
		if err != nil {
			return nil, fmt.Errorf("解密失败: %w", err)
		}
		finalPayload = plaintext

	} else {
		// 明文校验
		// 计算 Header + Payload 的 CRC32
		// bodyBuf 结构: [Payload... | Footer(16)]
		rawPayload := bodyBuf[:length]
		footer := bodyBuf[length:]

		chk := crc32.NewIEEE()
		chk.Write(headerBuf)
		chk.Write(rawPayload)
		actualSum := chk.Sum32()

		expectedSum := binary.LittleEndian.Uint32(footer[0:])
		if actualSum != expectedSum {
			return nil, fmt.Errorf("payload CRC32校验失败: 期望 0x%X, 实际 0x%X", expectedSum, actualSum)
		}
		finalPayload = rawPayload
	}

	return &inter.Packet{
		CmdID:       cmdID,
		KeyID:       keyID,
		IsAck:       isAck,
		IsEncrypted: isEncrypted,
		Payload:     finalPayload,
	}, nil
}
