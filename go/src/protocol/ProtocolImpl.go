package protocol

import (
	"crypto/aes"
	"crypto/cipher"
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

// -----------------------------------------------------------------------------
// CRC16/MODBUS 实现
// -----------------------------------------------------------------------------

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

// -----------------------------------------------------------------------------
// Pack 实现
// -----------------------------------------------------------------------------

func (c *GosterCodec) Pack(payload []byte, cmd inter.CmdID, keyID uint32, sessionKey []byte, seqNonce uint64) ([]byte, error) {
	// 1. 准备标志位
	var flags uint8 = 0
	isEncrypted := sessionKey != nil && keyID != 0
	if isEncrypted {
		flags |= 0x02 // Set Bit 1: ENCRYPTED
	}
	// TODO: 压缩支持可在此处添加

	// 2. 构造 Nonce (12 Bytes)
	// 建议结构: Salt(4B) + Seq(8B) 或直接使用 Seq 填充
	// 这里简化实现：直接将 seqNonce (uint64) 填入后 8 字节，前 4 字节留空或填 0
	nonce := make([]byte, 12)
	binary.LittleEndian.PutUint64(nonce[4:], seqNonce) // 简单的 Nonce 构造

	// 3. 准备 Payload (加密或原始内容)
	var finalPayload []byte
	var tag []byte
	var length uint32

	if isEncrypted {
		block, err := aes.NewCipher(sessionKey)
		if err != nil {
			return nil, fmt.Errorf("aes init failed: %w", err)
		}

		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, fmt.Errorf("gcm init failed: %w", err)
		}

		// 构造 AAD (Header 前 28 字节)
		// 我们需要先构建一个 Header 才能做 AAD，但 Length 字段此时取决于密文长度
		// AES-GCM 密文长度通常 = 明文长度 (不含 Tag，Tag 在 Seal 时追加)
		// Go 的 gcm.Seal 会将 Tag 追加在密文后面。
		// 但协议要求 Tag 放在 Footer。
		// 因此我们需要把 Tag 分离出来。

		// 这里的 length 是指 Payload 长度。
		// 加密后的 Payload 长度 = 明文长度。 Tag 是额外的。
		length = uint32(len(payload))

		// 预构建 Header 用于 AAD
		headerAAD := make([]byte, 28) // Offset 0-27
		binary.LittleEndian.PutUint16(headerAAD[0:], inter.MagicNumber)
		headerAAD[2] = inter.ProtocolVersion
		headerAAD[3] = flags
		binary.LittleEndian.PutUint16(headerAAD[4:], 0) // Status (Reserved)
		binary.LittleEndian.PutUint16(headerAAD[6:], uint16(cmd))
		binary.LittleEndian.PutUint32(headerAAD[8:], keyID)
		binary.LittleEndian.PutUint32(headerAAD[12:], length)
		copy(headerAAD[16:], nonce)

		// 执行加密
		// gcm.Seal(dst, nonce, plaintext, additionalData)
		// 注意：Go 的 gcm.Seal 会把 Authentication Tag (16 bytes) 附加在 ciphertext 的末尾
		ciphertextWithTag := gcm.Seal(nil, nonce, payload, headerAAD)

		// 分离 ciphertext 和 tag
		tagSize := gcm.Overhead() // 通常是 16
		if len(ciphertextWithTag) < tagSize {
			return nil, errors.New("encryption error: output too short")
		}

		finalPayload = ciphertextWithTag[:len(ciphertextWithTag)-tagSize]
		tag = ciphertextWithTag[len(ciphertextWithTag)-tagSize:]

	} else {
		// 明文模式
		length = uint32(len(payload))
		finalPayload = payload

		// 计算 CRC32 作为 Footer 的一部分
		// 协议: CRC32(Header + Payload) -> Footer[0:4]
		// 但这时候 Header 还没完全生成(缺 CRC16)，这会造成循环依赖吗？
		// 文档 2.3 节: "Byte 0-3: Header + Payload 的 CRC32 校验值"
		// 这里的 Header 通常是指除了 H_CRC16 之外的部分，或者整个已完成的 Header?
		// 通常为了简便，明文校验往往校验 payload。
		// 让我们再次查看 docs.md: "Header + Payload 的 CRC32"。
		// 如果 Header 包含 H_CRC16，那么必须先算 H_CRC16。
		// H_CRC16 计算范围是 Offset 0~27。
		// 所以我们可以先生成完整的 Header，再算 Footer 的 CRC32。
	}

	// 4. 构建最终 Header (32 Bytes)
	header := make([]byte, inter.HeaderSize)
	binary.LittleEndian.PutUint16(header[0:], inter.MagicNumber)
	header[2] = inter.ProtocolVersion
	header[3] = flags
	binary.LittleEndian.PutUint16(header[4:], 0) // Status
	binary.LittleEndian.PutUint16(header[6:], uint16(cmd))
	binary.LittleEndian.PutUint32(header[8:], keyID)
	binary.LittleEndian.PutUint32(header[12:], length)
	copy(header[16:], nonce)

	// 计算 Header CRC16 (前 28 字节)
	hCrc := crc16Modbus(header[:28])
	binary.LittleEndian.PutUint16(header[28:], hCrc)
	// header[30:32] 是 padding (0), make 已初始化为 0

	// 5. 构建 Footer (16 Bytes)
	footer := make([]byte, inter.FooterSize)
	if isEncrypted {
		// 填入 Tag (16 Bytes)
		if len(tag) != 16 {
			return nil, fmt.Errorf("invalid tag size: %d", len(tag))
		}
		copy(footer, tag)
	} else {
		// 填入 CRC32 + Padding
		// 计算范围: Header (32B) + Payload
		chk := crc32.NewIEEE()
		chk.Write(header)
		chk.Write(finalPayload)
		sum := chk.Sum32()
		binary.LittleEndian.PutUint32(footer[0:], sum)
		// footer[4:] 默认为 0
	}

	// 6. 拼接最终数据包
	// Size = 32 + Length + 16
	totalSize := int(inter.HeaderSize) + len(finalPayload) + int(inter.FooterSize)
	buf := make([]byte, totalSize)

	copy(buf[0:], header)
	copy(buf[inter.HeaderSize:], finalPayload)
	copy(buf[inter.HeaderSize+uint32(len(finalPayload)):], footer)

	return buf, nil
}

// -----------------------------------------------------------------------------
// Unpack 实现
// -----------------------------------------------------------------------------

func (c *GosterCodec) Unpack(r io.Reader, keyProvider inter.SessionKeyProvider) (*inter.Packet, error) {
	// 1. 读取 Header (32 Bytes)
	headerBuf := make([]byte, inter.HeaderSize)
	if _, err := io.ReadFull(r, headerBuf); err != nil {
		return nil, err
	}

	// 2. 验证 Magic
	magic := binary.LittleEndian.Uint16(headerBuf[0:])
	if magic != inter.MagicNumber {
		return nil, fmt.Errorf("invalid magic: 0x%X", magic)
	}

	// 3. 验证 Header CRC16
	expectedCRC := binary.LittleEndian.Uint16(headerBuf[28:])
	actualCRC := crc16Modbus(headerBuf[:28])
	if expectedCRC != actualCRC {
		return nil, fmt.Errorf("header crc mismatch: expect 0x%X, got 0x%X", expectedCRC, actualCRC)
	}

	// 4. 解析 Header 字段
	// version := headerBuf[2]
	flags := headerBuf[3]
	cmdID := inter.CmdID(binary.LittleEndian.Uint16(headerBuf[6:]))
	keyID := binary.LittleEndian.Uint32(headerBuf[8:])
	length := binary.LittleEndian.Uint32(headerBuf[12:]) // Payload 长度
	nonce := headerBuf[16:28]                            // 12 Bytes

	isEncrypted := (flags & 0x02) != 0
	isAck := (flags & 0x01) != 0

	// 5. 读取 Payload + Footer
	// 总共需要读取 length + 16 字节
	bodyLen := length + inter.FooterSize
	bodyBuf := make([]byte, bodyLen)
	if _, err := io.ReadFull(r, bodyBuf); err != nil {
		return nil, err
	}

	rawPayload := bodyBuf[:length]
	footer := bodyBuf[length:]

	// 6. 验证与解密
	var finalPayload []byte

	if isEncrypted {
		// 查找密钥
		if keyProvider == nil {
			return nil, errors.New("encrypted packet received but no key provider")
		}
		sessionKey, err := keyProvider(keyID)
		if err != nil {
			return nil, fmt.Errorf("key lookup failed: %w", err)
		}
		if sessionKey == nil {
			return nil, fmt.Errorf("session key not found for ID: %d", keyID)
		}

		// AES-GCM 解密
		block, err := aes.NewCipher(sessionKey)
		if err != nil {
			return nil, err
		}
		gcm, err := cipher.NewGCM(block)
		if err != nil {
			return nil, err
		}

		// 重构 ciphertext (Payload + Tag) 用于 gcm.Open
		// Tag 位于 Footer
		tag := footer // 16 bytes
		ciphertextWithTag := make([]byte, len(rawPayload)+len(tag))
		copy(ciphertextWithTag, rawPayload)
		copy(ciphertextWithTag[len(rawPayload):], tag)

		// 构造 AAD (Header 前 28 字节)
		headerAAD := headerBuf[:28]

		// 解密
		// gcm.Open(dst, nonce, ciphertext, additionalData)
		plaintext, err := gcm.Open(nil, nonce, ciphertextWithTag, headerAAD)
		if err != nil {
			return nil, fmt.Errorf("decryption failed: %w", err)
		}
		finalPayload = plaintext

	} else {
		// 明文校验
		// 计算 Header + Payload 的 CRC32
		chk := crc32.NewIEEE()
		chk.Write(headerBuf)
		chk.Write(rawPayload)
		actualSum := chk.Sum32()

		expectedSum := binary.LittleEndian.Uint32(footer[0:])
		if actualSum != expectedSum {
			return nil, fmt.Errorf("payload crc32 mismatch: expect 0x%X, got 0x%X", expectedSum, actualSum)
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
