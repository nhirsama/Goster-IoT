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
	"github.com/sigurn/crc16"
)

// GosterCodec 实现 inter.ProtocolCodec 接口
type GosterCodec struct{}

// NewGosterCodec 创建一个新的编解码器实例
func NewGosterCodec() inter.ProtocolCodec {
	return &GosterCodec{}
}

// 初始化 Modbus CRC16 表
var modbusTable = crc16.MakeTable(crc16.CRC16_MODBUS)

func crc16Modbus(data []byte) uint16 {
	return crc16.Checksum(data, modbusTable)
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
