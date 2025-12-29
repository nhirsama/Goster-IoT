package cli

import (
	"crypto/ecdh"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/nhirsama/Goster-IoT/src/inter"
	"github.com/nhirsama/Goster-IoT/src/protocol"
)

// GosterClient 封装单个会话连接
type GosterClient struct {
	conn       net.Conn
	codec      inter.ProtocolCodec
	privateKey *ecdh.PrivateKey
	sessionKey []byte
	writeSeq   uint64
}

func NewGosterClient() *GosterClient {
	priv, _ := ecdh.X25519().GenerateKey(rand.Reader)
	return &GosterClient{
		codec:      protocol.NewGosterCodec(),
		privateKey: priv,
	}
}

func (c *GosterClient) Connect(address string) error {
	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

func (c *GosterClient) Close() {
	if c.conn != nil {
		c.conn.Close()
	}
}

// Handshake 执行密钥交换
func (c *GosterClient) Handshake() error {
	// 1. 发送公钥 (Handshake Init)
	pubKey := c.privateKey.PublicKey().Bytes()
	c.writeSeq++
	reqBuf, err := c.codec.Pack(pubKey, inter.CmdHandshakeInit, 0, nil, c.writeSeq)
	if err != nil {
		return err
	}
	if _, err := c.conn.Write(reqBuf); err != nil {
		return err
	}

	// 2. 接收响应
	packet, err := c.codec.Unpack(c.conn, nil)
	if err != nil {
		return err
	}
	if packet.CmdID != inter.CmdHandshakeResp {
		return fmt.Errorf("unexpected cmd: 0x%X", packet.CmdID)
	}

	// 3. 计算 Session Key
	peerPubKey, err := ecdh.X25519().NewPublicKey(packet.Payload)
	if err != nil {
		return err
	}
	c.sessionKey, err = c.privateKey.ECDH(peerPubKey)
	return err
}

// Register 发送注册请求 (0x0005)
func (c *GosterClient) Register(meta inter.DeviceMetadata) (byte, string, error) {
	// 构造元数据 Payload (RS 分隔)
	parts := []string{
		meta.Name, meta.SerialNumber, meta.MACAddress,
		meta.HWVersion, meta.SWVersion, meta.ConfigVersion,
	}
	payload := strings.Join(parts, "\x1e")

	c.writeSeq++
	reqBuf, err := c.codec.Pack([]byte(payload), inter.CmdDeviceRegister, 1, c.sessionKey, c.writeSeq)
	if err != nil {
		return 0, "", err
	}
	if _, err := c.conn.Write(reqBuf); err != nil {
		return 0, "", err
	}

	return c.readAck()
}

// Authenticate 发送 Token 鉴权 (0x0003)
func (c *GosterClient) Authenticate(token string) (byte, error) {
	c.writeSeq++
	reqBuf, err := c.codec.Pack([]byte(token), inter.CmdAuthVerify, 1, c.sessionKey, c.writeSeq)
	if err != nil {
		return 0, err
	}
	if _, err := c.conn.Write(reqBuf); err != nil {
		return 0, err
	}

	status, _, err := c.readAck()
	return status, err
}

// SendMetrics 发送模拟指标数据
func (c *GosterClient) SendMetrics() error {
	// [StartTs(8)] [Interval(4)] [Type(1)] [Count(4)] [DataBlob...]
	startTs := time.Now().UnixMilli()
	interval := uint32(1000 * 1000) // 1s
	count := uint32(5)
	dataType := uint8(0) // Float32

	buf := make([]byte, 17+int(count)*4)
	binary.LittleEndian.PutUint64(buf[0:], uint64(startTs))
	binary.LittleEndian.PutUint32(buf[8:], interval)
	buf[12] = dataType
	binary.LittleEndian.PutUint32(buf[13:], count)

	for i := 0; i < int(count); i++ {
		val := float32(math.Sin(float64(i)))
		bits := math.Float32bits(val)
		binary.LittleEndian.PutUint32(buf[17+i*4:], bits)
	}

	c.writeSeq++
	reqBuf, err := c.codec.Pack(buf, inter.CmdMetricsReport, 1, c.sessionKey, c.writeSeq)
	if err != nil {
		return err
	}
	_, err = c.conn.Write(reqBuf)
	return err
}

func (c *GosterClient) readAck() (byte, string, error) {
	packet, err := c.codec.Unpack(c.conn, c.sessionKey)
	if err != nil {
		return 0, "", err
	}
	if packet.CmdID != inter.CmdAuthAck {
		return 0, "", fmt.Errorf("unexpected cmd: 0x%X", packet.CmdID)
	}

	status := packet.Payload[0]
	token := ""
	if len(packet.Payload) > 1 {
		token = string(packet.Payload[1:])
	}
	return status, token, nil
}

// TestInteractiveRegistration 模拟真实设备接入交互
// 流程：
// 1. 发起注册 -> 收到 Pending (0x02)
// 2. 循环轮询 (用户手动去后台批准)
// 3. 再次注册 -> 收到 Success (0x00) + Token
// 4. 使用 Token 鉴权 -> Success
func TestInteractiveRegistration(t *testing.T) {
	serverAddr := "127.0.0.1:8081"

	// 1. 随机生成设备信息
	ts := time.Now().UnixNano()
	sn := fmt.Sprintf("SN-%d", ts%100000)
	mac := fmt.Sprintf("52:54:%02x:%02x:%02x:%02x",
		(ts>>24)&0xFF, (ts>>16)&0xFF, (ts>>8)&0xFF, ts&0xFF)
	name := fmt.Sprintf("TestDevice-%s", sn[len(sn)-4:])

	meta := inter.DeviceMetadata{
		Name:          name,
		SerialNumber:  sn,
		MACAddress:    mac,
		HWVersion:     "v1.0-Sim",
		SWVersion:     "v2.0-Test",
		ConfigVersion: "v1",
	}

	t.Logf("=== 开始模拟设备交互测试 ===")
	t.Logf("设备: %s (SN: %s, MAC: %s)", name, sn, mac)

	var receivedToken string

	// 2. 注册轮询循环 (最多尝试 20 次，每次间隔 3 秒)
	// 这样给你足够的时间在后台点击 "Approve"
	maxRetries := 20
	retryInterval := 3 * time.Second

	for i := 0; i < maxRetries; i++ {
		t.Logf("--- 尝试连接与注册 (第 %d/%d 次) ---", i+1, maxRetries)

		client := NewGosterClient()
		if err := client.Connect(serverAddr); err != nil {
			t.Logf("连接失败: %v (请确保 API 服务运行中)", err)
			time.Sleep(retryInterval)
			continue
		}

		// Handshake
		if err := client.Handshake(); err != nil {
			client.Close()
			t.Fatalf("握手失败: %v", err)
		}

		// Register (Retry)
		status, token, err := client.Register(meta)
		if err != nil {
			// 某些情况下服务器断开连接会导致 EOF，这在 Pending 状态下是预期的
			t.Logf("注册响应读取错误 (可能连接已关闭): %v", err)
		} else {
			t.Logf("收到注册状态码: 0x%02X", status)
			if status == 0x00 && token != "" {
				receivedToken = token
				t.Logf(">>> 成功获取 Token: %s", token)
				client.Close()
				break // 成功！跳出循环
			} else if status == 0x02 {
				t.Log(">>> 状态: 待审核 (Pending)")
				t.Log(">>> [请现在去 Web 管理后台批准该设备接入!]")
			} else if status == 0x01 {
				t.Log(">>> 状态: 被拒绝 (Refused)")
			}
		}

		client.Close()

		if i < maxRetries-1 {
			t.Logf("等待 %v 后重试...", retryInterval)
			time.Sleep(retryInterval)
		}
	}

	if receivedToken == "" {
		t.Fatalf("超时未获取到 Token，测试失败。请检查是否在后台批准了设备。")
	}

	// 3. 最终验证：使用获取到的 Token 进行鉴权
	t.Log("--- 最终验证：使用 Token 登录 ---")
	client := NewGosterClient()
	if err := client.Connect(serverAddr); err != nil {
		t.Fatal(err)
	}
	defer client.Close()

	if err := client.Handshake(); err != nil {
		t.Fatal(err)
	}

	status, err := client.Authenticate(receivedToken)
	if err != nil {
		t.Fatalf("Token 鉴权失败: %v", err)
	}

	if status == 0x00 {
		t.Log("Token 鉴权成功！")
		// 发送一些数据作为 Proof of Work
		if err := client.SendMetrics(); err != nil {
			t.Logf("数据上报失败: %v", err)
		} else {
			t.Log("模拟数据上报成功")
		}
	} else {
		t.Fatalf("Token 鉴权被拒绝 (Status: 0x%X)", status)
	}

	t.Log("=== 交互测试顺利通过 ===")
}
