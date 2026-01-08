package protocol

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"testing"

	"github.com/nhirsama/Goster-IoT/src/inter"
)

// =============================================================================
// 辅助函数与变量
// =============================================================================

var (
	testKeyID      uint32 = 1001
	testSessionKey []byte
)

func init() {
	testSessionKey = make([]byte, 32)
	rand.Read(testSessionKey)
}

// 生成指定大小的随机 Payload
func generatePayload(size int) []byte {
	p := make([]byte, size)
	rand.Read(p)
	return p
}

// =============================================================================
// 单元测试 (Unit Tests)
// =============================================================================

// 测试：明文模式下的封包与解包
func TestPackUnpack_Plain(t *testing.T) {
	codec := NewGosterCodec()
	payload := []byte("Hello Goster IoT")
	cmd := inter.CmdMetricsReport

	// 1. Pack
	buf, err := codec.Pack(payload, cmd, 0, nil, 12345)
	if err != nil {
		t.Fatalf("Pack failed: %v", err)
	}

	// 验证总长度: Header(32) + Payload + Footer(16)
	expectedLen := int(inter.HeaderSize) + len(payload) + int(inter.FooterSize)
	if len(buf) != expectedLen {
		t.Errorf("Pack length mismatch: got %d, want %d", len(buf), expectedLen)
	}

	// 2. Unpack
	// 明文解包，key 传 nil
	packet, err := codec.Unpack(bytes.NewReader(buf), nil)
	if err != nil {
		t.Fatalf("Unpack failed: %v", err)
	}

	// 3. Verify
	if packet.CmdID != cmd {
		t.Errorf("CmdID mismatch: got %v, want %v", packet.CmdID, cmd)
	}
	if packet.IsEncrypted {
		t.Error("Packet should not be encrypted")
	}
	if !bytes.Equal(packet.Payload, payload) {
		t.Errorf("Payload mismatch: got %x, want %x", packet.Payload, payload)
	}
}

// 测试：加密模式下的封包与解包
func TestPackUnpack_Encrypted(t *testing.T) {
	codec := NewGosterCodec()
	payload := generatePayload(1024) // 1KB Random Data
	cmd := inter.CmdConfigPush

	// 1. Pack
	buf, err := codec.Pack(payload, cmd, testKeyID, testSessionKey, 98765)
	if err != nil {
		t.Fatalf("Pack failed: %v", err)
	}

	// 加密后的长度通常等于: Header + Payload + Tag(16) [在Footer位置]
	expectedLen := int(inter.HeaderSize) + len(payload) + int(inter.FooterSize)
	if len(buf) != expectedLen {
		t.Errorf("Pack length mismatch: got %d, want %d", len(buf), expectedLen)
	}

	// 2. Unpack
	// 加密解包，传入对应的 sessionKey
	packet, err := codec.Unpack(bytes.NewReader(buf), testSessionKey)
	if err != nil {
		t.Fatalf("Unpack failed: %v", err)
	}

	// 3. Verify
	if !packet.IsEncrypted {
		t.Error("Packet should be encrypted")
	}
	if packet.KeyID != testKeyID {
		t.Errorf("KeyID mismatch: got %d, want %d", packet.KeyID, testKeyID)
	}
	if !bytes.Equal(packet.Payload, payload) {
		t.Error("Payload mismatch after decryption")
	}
}

// 测试：解包时的 Magic 校验
func TestUnpack_InvalidMagic(t *testing.T) {
	codec := NewGosterCodec()
	buf := make([]byte, inter.HeaderSize+inter.FooterSize)
	// 默认全是0，Magic 0x0000 != 0x5759

	_, err := codec.Unpack(bytes.NewReader(buf), nil)
	if err == nil {
		t.Error("Expect error for invalid magic, got nil")
	}
}

// 测试：解包时的 Header CRC 校验失败
func TestUnpack_HeaderCorruption(t *testing.T) {
	codec := NewGosterCodec()
	payload := []byte("test")
	buf, _ := codec.Pack(payload, inter.CmdHandshakeInit, 0, nil, 1)

	// 修改 Header 中的一个字节 (Offset 6 is CmdID low byte)
	buf[6] ^= 0xFF

	_, err := codec.Unpack(bytes.NewReader(buf), nil)
	if err == nil {
		t.Error("Expect error for header corruption, got nil")
	}
}

// 测试：明文 Payload 篡改导致 CRC32 校验失败
func TestUnpack_Plain_PayloadCorruption(t *testing.T) {
	codec := NewGosterCodec()
	payload := []byte("important data")
	buf, _ := codec.Pack(payload, inter.CmdHandshakeInit, 0, nil, 1)

	// 修改 Payload 中的一个字节
	// Payload 始于 HeaderSize (32)
	buf[inter.HeaderSize] ^= 0xFF

	_, err := codec.Unpack(bytes.NewReader(buf), nil)
	if err == nil {
		t.Error("Expect error for payload corruption (CRC32), got nil")
	}
}

// 测试：加密数据篡改导致解密失败 (GCM Tag Check)
func TestUnpack_Encrypted_Tampering(t *testing.T) {
	codec := NewGosterCodec()
	payload := []byte("secret data")
	buf, _ := codec.Pack(payload, inter.CmdHandshakeInit, testKeyID, testSessionKey, 1)

	// 修改密文 (Payload部分)
	buf[inter.HeaderSize] ^= 0xFF

	_, err := codec.Unpack(bytes.NewReader(buf), testSessionKey)
	if err == nil {
		t.Error("Expect error for encrypted payload tampering, got nil")
	}
}

// 测试：Payload 过大 (超过 100MB 限制)
func TestPack_TooLarge(t *testing.T) {
	codec := NewGosterCodec()
	// 测试 Unpack 的限制

	fakeHeader := make([]byte, inter.HeaderSize)
	binary.LittleEndian.PutUint16(fakeHeader[0:], inter.MagicNumber)
	binary.LittleEndian.PutUint32(fakeHeader[12:], 100*1024*1024+1) // Length > 100MB

	// 计算正确的 CRC 使得能过 Header 校验
	crc := crc16Modbus(fakeHeader[:28])
	binary.LittleEndian.PutUint16(fakeHeader[28:], crc)

	_, err := codec.Unpack(bytes.NewReader(fakeHeader), nil)
	if err == nil {
		t.Error("Expect error for payload too large in Unpack")
	}
}

// 测试：并发安全性 (Codec 应该是无状态的)
func TestConcurrency(t *testing.T) {
	codec := NewGosterCodec()
	for i := 0; i < 100; i++ {
		go func() {
			payload := []byte("data")
			_, _ = codec.Pack(payload, inter.CmdLogReport, 0, nil, uint64(i))
		}()
	}
}

// =============================================================================
// 性能测试 (Benchmarks)
// =============================================================================

// 基准测试：明文封包
func BenchmarkPack_Plain_1KB(b *testing.B) {
	codec := NewGosterCodec()
	payload := generatePayload(1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = codec.Pack(payload, inter.CmdLogReport, 0, nil, uint64(i))
	}
}

func BenchmarkPack_Plain_64KB(b *testing.B) {
	codec := NewGosterCodec()
	payload := generatePayload(64 * 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = codec.Pack(payload, inter.CmdLogReport, 0, nil, uint64(i))
	}
}

// 基准测试：加密封包 (AES-GCM)
func BenchmarkPack_Encrypted_1KB(b *testing.B) {
	codec := NewGosterCodec()
	payload := generatePayload(1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = codec.Pack(payload, inter.CmdLogReport, testKeyID, testSessionKey, uint64(i))
	}
}

func BenchmarkPack_Encrypted_64KB(b *testing.B) {
	codec := NewGosterCodec()
	payload := generatePayload(64 * 1024)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = codec.Pack(payload, inter.CmdLogReport, testKeyID, testSessionKey, uint64(i))
	}
}

// 基准测试：明文解包
func BenchmarkUnpack_Plain_1KB(b *testing.B) {
	codec := NewGosterCodec()
	payload := generatePayload(1024)
	// 先 Pack 准备好数据
	buf, _ := codec.Pack(payload, inter.CmdLogReport, 0, nil, 1)
	reader := bytes.NewReader(buf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Reset(buf) // 重置 reader
		_, _ = codec.Unpack(reader, nil)
	}
}

// 基准测试：加密解包
func BenchmarkUnpack_Encrypted_1KB(b *testing.B) {
	codec := NewGosterCodec()
	payload := generatePayload(1024)
	buf, _ := codec.Pack(payload, inter.CmdLogReport, testKeyID, testSessionKey, 1)
	reader := bytes.NewReader(buf)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		reader.Reset(buf)
		_, _ = codec.Unpack(reader, testSessionKey)
	}
}
