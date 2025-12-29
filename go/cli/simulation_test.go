package cli

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"hash/crc32"
	"io"
	"net"
	"os"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// 协议常量本地定义
// =============================================================================

const (
	MagicNumber uint16 = 0x5759
	HeaderSize  int    = 32
	FooterSize  int    = 16

	CmdHandshakeInit  uint16 = 0x0001
	CmdHandshakeResp  uint16 = 0x0002
	CmdAuthVerify     uint16 = 0x0003
	CmdAuthAck        uint16 = 0x0004
	CmdDeviceRegister uint16 = 0x0005
	CmdErrorReport    uint16 = 0x00FF
	CmdMetricsReport  uint16 = 0x0101
	CmdLogReport      uint16 = 0x0102
	CmdEventReport    uint16 = 0x0103
	CmdHeartbeat      uint16 = 0x0104
)

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

func calcCRC16(data []byte) uint16 {
	var crc uint16 = 0xFFFF
	for _, b := range data {
		crc = (crc >> 8) ^ crc16Table[(crc^uint16(b))&0xFF]
	}
	return crc
}

// =============================================================================
// SimClient 模拟端实现
// =============================================================================

type SimPacket struct {
	CmdID   uint16
	IsAck   bool
	Payload []byte
}

type SimClient struct {
	conn       net.Conn
	privKey    *ecdh.PrivateKey
	sessionKey []byte
	writeSeq   uint64
}

func NewSimClient() *SimClient {
	p, _ := ecdh.X25519().GenerateKey(rand.Reader)
	return &SimClient{privKey: p}
}

func (c *SimClient) Connect(addr string) error {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

func (c *SimClient) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

func (c *SimClient) pack(payload []byte, cmd uint16, isAck bool) []byte {
	c.writeSeq++
	payloadLen := len(payload)
	var flags uint8 = 0
	if isAck {
		flags |= 0x01
	}

	nonce := make([]byte, 12)
	binary.LittleEndian.PutUint64(nonce[4:], c.writeSeq)

	headerBase := make([]byte, 28)
	binary.LittleEndian.PutUint16(headerBase[0:], MagicNumber)
	headerBase[2] = 0x01 // Version
	headerBase[3] = flags
	binary.LittleEndian.PutUint16(headerBase[6:], cmd)
	binary.LittleEndian.PutUint32(headerBase[8:], 1) // KeyID
	binary.LittleEndian.PutUint32(headerBase[12:], uint32(payloadLen))
	copy(headerBase[16:], nonce)

	finalPayload := payload
	totalSize := HeaderSize + payloadLen + FooterSize

	if c.sessionKey != nil {
		headerBase[3] |= 0x02 // Encrypted
		block, _ := aes.NewCipher(c.sessionKey)
		gcm, _ := cipher.NewGCM(block)
		finalPayload = gcm.Seal(nil, nonce, payload, headerBase)
		totalSize = HeaderSize + len(finalPayload)
	}

	buf := make([]byte, totalSize)
	copy(buf[:28], headerBase)
	binary.LittleEndian.PutUint16(buf[28:], calcCRC16(buf[:28]))
	copy(buf[32:], finalPayload)

	if c.sessionKey == nil {
		chk := crc32.NewIEEE()
		chk.Write(buf[:32+payloadLen])
		binary.LittleEndian.PutUint32(buf[32+payloadLen:], chk.Sum32())
	}
	return buf
}

func (c *SimClient) unpack() (*SimPacket, error) {
	header := make([]byte, HeaderSize)
	if _, err := io.ReadFull(c.conn, header); err != nil {
		return nil, err
	}
	flags := header[3]
	cmd := binary.LittleEndian.Uint16(header[6:])
	pLen := binary.LittleEndian.Uint32(header[12:])
	nonce := header[16:28]

	body := make([]byte, int(pLen)+FooterSize)
	if flags&0x02 == 0 {
		if _, err := io.ReadFull(c.conn, body); err != nil {
			return nil, err
		}
	} else {
		// 加密模式下，Footer 就是 Tag，已包含在 pLen 中？
		// 实际上服务端 Pack 加密包时，TotalSize = 32 + len + 16 (Tag)
		// 且 Header.Length = len (明文长度)
		// 所以物理上需要读取 明文长度 + 16 字节
		body = make([]byte, int(pLen)+16)
		if _, err := io.ReadFull(c.conn, body); err != nil {
			return nil, err
		}
	}

	payload := body[:pLen]
	if flags&0x02 != 0 && c.sessionKey != nil {
		block, _ := aes.NewCipher(c.sessionKey)
		gcm, _ := cipher.NewGCM(block)
		plain, err := gcm.Open(nil, nonce, body, header[:28])
		if err != nil {
			return nil, err
		}
		payload = plain
	}
	return &SimPacket{CmdID: cmd, IsAck: flags&0x01 != 0, Payload: payload}, nil
}

func (c *SimClient) DoHandshake() error {
	c.conn.Write(c.pack(c.privKey.PublicKey().Bytes(), CmdHandshakeInit, false))
	resp, err := c.unpack()
	if err != nil {
		return err
	}
	peerPub, _ := ecdh.X25519().NewPublicKey(resp.Payload)
	c.sessionKey, _ = c.privKey.ECDH(peerPub)
	return nil
}

// =============================================================================
// 全场景集成测试
// =============================================================================

func TestCompleteIoTSystemSimulation(t *testing.T) {
	os.Setenv("DB_PATH", "./full_test.db")
	os.Setenv("HTML_DIR", "../html")
	defer os.Remove("./full_test.db")

	t.Log(">>> 启动 Goster-IoT 后台全功能服务...")
	go Run()
	time.Sleep(2 * time.Second)

	serverAddr := "127.0.0.1:8081"
	ts := time.Now().UnixNano()
	sn := fmt.Sprintf("FULL-SN-%d", ts%100000)
	mac := "00:AA:BB:CC:DD:EE"

	var deviceToken string

	// --- 场景 1: 新设备注册并进入轮询 ---
	t.Run("RegistrationAndPolling", func(t *testing.T) {
		for i := 1; i <= 10; i++ {
			t.Logf("轮次 %d: 尝试注册设备 %s", i, sn)
			client := NewSimClient()
			if err := client.Connect(serverAddr); err != nil {
				t.Fatal(err)
			}
			client.DoHandshake()

			regPayload := strings.Join([]string{"Comprehensive-Sensor", sn, mac, "v1", "v1", "v1"}, "\x1e")
			client.conn.Write(client.pack([]byte(regPayload), CmdDeviceRegister, false))

			ack, err := client.unpack()
			if err != nil {
				t.Log("连接断开 (审核中)")
				client.Close()
				time.Sleep(3 * time.Second)
				continue
			}

			if ack.Payload[0] == 0x00 {
				deviceToken = string(ack.Payload[1:])
				t.Logf(">>> 审批通过！拿到 Token: %s", deviceToken)

				// 场景 2: 链式业务测试 (不换连接)
				t.Run("ChainedSessionBusiness", func(t *testing.T) {
					t.Log(">>> 执行链式业务: 鉴权 -> 心跳 -> 指标")
					client.conn.Write(client.pack([]byte(deviceToken), CmdAuthVerify, false))
					authAck, _ := client.unpack()
					if authAck.Payload[0] != 0x00 {
						t.Fatal("链式鉴权失败")
					}

					// 心跳
					client.conn.Write(client.pack(nil, CmdHeartbeat, false))
					hbAck, _ := client.unpack()
					if !hbAck.IsAck {
						t.Error("心跳未收到 ACK")
					}

					// 指标
					metrics := make([]byte, 17)                    // 伪造 5 个点的 float32 数据头
					binary.LittleEndian.PutUint32(metrics[13:], 0) // 0 points for simplicity
					client.conn.Write(client.pack(metrics, CmdMetricsReport, false))
					mAck, _ := client.unpack()
					if !mAck.IsAck {
						t.Error("指标未收到 ACK")
					}
				})
				client.Close()
				break
			}
			client.Close()
			t.Log("等待 3 秒后重试...")
			time.Sleep(3 * time.Second)
		}
	})

	if deviceToken == "" {
		t.Fatalf("未能完成注册流程")
	}

	// --- 场景 3: 断开重连登录 (Token Auth) ---
	t.Run("ReconnectTokenAuth", func(t *testing.T) {
		t.Log(">>> 模拟设备断电重启，使用持久化 Token 登录")
		client := NewSimClient()
		client.Connect(serverAddr)
		client.DoHandshake()

		client.conn.Write(client.pack([]byte(deviceToken), CmdAuthVerify, false))
		ack, _ := client.unpack()
		if ack.Payload[0] != 0x00 {
			t.Fatal("Token 重连鉴权失败")
		}

		// 上报日志
		logMsg := "System rebooted successfully"
		logBuf := new(bytes.Buffer)
		binary.Write(logBuf, binary.LittleEndian, time.Now().UnixMilli())
		logBuf.WriteByte(1) // INFO
		binary.Write(logBuf, binary.LittleEndian, uint16(len(logMsg)))
		logBuf.WriteString(logMsg)

		client.conn.Write(client.pack(logBuf.Bytes(), CmdLogReport, false))
		lAck, _ := client.unpack()
		if !lAck.IsAck {
			t.Error("日志未收到 ACK")
		}
		client.Close()
	})

	// --- 场景 4: 错误报告测试 (Error Case) ---
	t.Run("ErrorReporting", func(t *testing.T) {
		t.Log(">>> 模拟设备发生故障并上报错误")
		client := NewSimClient()
		client.Connect(serverAddr)
		client.DoHandshake()
		client.conn.Write(client.pack([]byte(deviceToken), CmdAuthVerify, false))
		client.unpack()

		// 发送错误报告
		client.conn.Write(client.pack([]byte("SENSOR_HARDWARE_FAILURE"), CmdErrorReport, false))

		// 验证服务器是否主动断开
		oneByte := make([]byte, 1)
		client.conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		_, err := client.conn.Read(oneByte)
		if err != io.EOF {
			t.Errorf("上报错误后服务器未及时断开连接: %v", err)
		}
		client.Close()
	})

	t.Log("=== 所有模拟场景测试通过 ===")
}
