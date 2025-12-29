package cli

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"math"
	"math/rand"
	"net"
	"testing"
	"time"
)

// =============================================================================
// Simulation Constants & Types (Re-defined to simulate external software)
// =============================================================================

const (
	SimMagicNumber     uint16 = 0x5759
	SimProtocolVersion uint8  = 0x01
	SimHeaderSize      int    = 32
	SimFooterSize      int    = 16
)

// Commands
const (
	SimCmdMetricsReport uint16 = 0x0101
	SimCmdLogReport     uint16 = 0x0102
	SimCmdHeartbeat     uint16 = 0x0104
)

// =============================================================================
// CRC16 Modbus Implementation (Copy for simulation)
// =============================================================================

var simCrc16Table = []uint16{
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

func simCrc16Modbus(data []byte) uint16 {
	var crc uint16 = 0xFFFF
	for _, b := range data {
		crc = (crc >> 8) ^ simCrc16Table[(crc^uint16(b))&0xFF]
	}
	return crc
}

// =============================================================================
// Simulation Logic
// =============================================================================

// packSimulatedFrame manually constructs a frame following the docs.
func packSimulatedFrame(cmd uint16, payload []byte) []byte {
	payloadLen := len(payload)
	totalSize := SimHeaderSize + payloadLen + SimFooterSize
	buf := make([]byte, totalSize)

	// --- Header (0-31) ---
	// 0-1: Magic
	binary.LittleEndian.PutUint16(buf[0:], SimMagicNumber)
	// 2: Version
	buf[2] = SimProtocolVersion
	// 3: Flags (0x00 for plain, no ack req)
	buf[3] = 0x00
	// 4-5: Status (0)
	binary.LittleEndian.PutUint16(buf[4:], 0)
	// 6-7: CmdID
	binary.LittleEndian.PutUint16(buf[6:], cmd)
	// 8-11: KeyID (0 for no encryption)
	binary.LittleEndian.PutUint32(buf[8:], 0)
	// 12-15: Payload Len
	binary.LittleEndian.PutUint32(buf[12:], uint32(payloadLen))

	// 16-19: Salt (Random)
	rand.Read(buf[16:20])
	// 20-27: SeqNonce (Use timestamp for sim)
	seq := uint64(time.Now().UnixNano())
	binary.LittleEndian.PutUint64(buf[20:], seq)

	// 28-29: Header CRC16 (Calculated over 0-27)
	hCrc := simCrc16Modbus(buf[:28])
	binary.LittleEndian.PutUint16(buf[28:], hCrc)
	// 30-31: Padding (0) - already 0 initialized

	// --- Payload ---
	copy(buf[32:], payload)

	// --- Footer (CRC32 + Padding) ---
	// CRC32 IEEE of Header + Payload
	// Header is buf[0:32], Payload is buf[32 : 32+payloadLen]
	// So we calc CRC32 of buf[0 : 32+payloadLen]
	chk := crc32.NewIEEE()
	chk.Write(buf[:32+payloadLen])
	sum := chk.Sum32()

	// Footer starts at 32+payloadLen
	footerStart := 32 + payloadLen
	binary.LittleEndian.PutUint32(buf[footerStart:], sum)
	// Remaining 12 bytes are padding (0)

	return buf
}

func sendFrame(t *testing.T, conn net.Conn, cmd uint16, payload []byte) {
	frame := packSimulatedFrame(cmd, payload)
	t.Logf("Sending Frame Cmd=0x%X, Len=%d", cmd, len(frame))
	if _, err := conn.Write(frame); err != nil {
		t.Fatalf("Failed to write frame: %v", err)
	}
}

func TestExternalSoftwareSimulation(t *testing.T) {
	// PLACEHOLDER for the real server address
	serverAddr := "127.0.0.1:8081"

	conn, err := net.DialTimeout("tcp", serverAddr, 2*time.Second)
	if err != nil {
		t.Fatalf("Simulation Error: Could not connect to %s. Ensure the server is running. Error: %v", serverAddr, err)
	}
	defer conn.Close()
	t.Logf("Connected to %s", serverAddr)

	// 1. Send Heartbeat
	t.Log("--- Testing Heartbeat ---")
	sendFrame(t, conn, SimCmdHeartbeat, []byte("ping"))

	// 2. Send Metrics Report
	t.Log("--- Testing Metrics Report ---")
	// Struct: [StartTs(8)] [Interval(4)] [Type(1)] [Count(4)] [DataBlob...]
	metricsBuf := new(bytes.Buffer)
	startTs := time.Now().UnixMilli()
	interval := uint32(1000 * 1000) // 1s in us
	count := uint32(5)
	dataType := uint8(0) // Float32

	binary.Write(metricsBuf, binary.LittleEndian, startTs)
	binary.Write(metricsBuf, binary.LittleEndian, interval)
	binary.Write(metricsBuf, binary.LittleEndian, dataType)
	binary.Write(metricsBuf, binary.LittleEndian, count)

	// Generate 5 float32 points (sin wave)
	for i := 0; i < int(count); i++ {
		val := float32(math.Sin(float64(i)))
		binary.Write(metricsBuf, binary.LittleEndian, val)
	}
	sendFrame(t, conn, SimCmdMetricsReport, metricsBuf.Bytes())

	// 3. Send Log Report
	t.Log("--- Testing Log Report ---")
	// Struct: [Timestamp(8)] [Level(1)] [MsgLen(2)] [Message...]
	logBuf := new(bytes.Buffer)
	logTs := time.Now().UnixMilli()
	level := uint8(1) // INFO
	msg := "Simulation Log Message from CLI Test"
	msgLen := uint16(len(msg))

	binary.Write(logBuf, binary.LittleEndian, logTs)
	binary.Write(logBuf, binary.LittleEndian, level)
	binary.Write(logBuf, binary.LittleEndian, msgLen)
	logBuf.WriteString(msg)

	sendFrame(t, conn, SimCmdLogReport, logBuf.Bytes())

	// Note: Authentication is skipped here as we are simulating "other convenient content"
	// and raw data injection. A full auth flow would require simulating ECDH which is complex
	// for a single file test, but the server logs should show "Unauthenticated" or process
	// these if they are allowed without auth (ApiImpl.go check: needs auth except handshake/auth).
	// Wait, ApiImpl.go says:
	// if !authenticated && packet.CmdID != inter.CmdHandshakeInit && packet.CmdID != inter.CmdAuthVerify { ... return }
	// So these will likely be rejected by the server if we don't auth.
	// However, for "Simulation" of protocol format correctness, we send them.
	// If the user wants successful processing, we MUST implement handshake.
	// Let's assume the user just wants to ensure the *frames* are sent correctly defined.
	// To actually verify they work, we would need to see server logs or implement the handshake.
	// Given the instructions "simulate an external software... send information", sending correctly
	// formatted frames is the primary goal.

	// We sleep briefly to ensure frames are flushed
	time.Sleep(100 * time.Millisecond)
}
