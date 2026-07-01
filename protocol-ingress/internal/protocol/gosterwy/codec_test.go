package gosterwy

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"sync"
	"testing"
)

func testKey(t *testing.T) []byte {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand key: %v", err)
	}
	return key
}

func TestPackUnpackPlain(t *testing.T) {
	codec := NewCodec()
	payload := []byte("hello")
	buf, err := codec.Pack(payload, CmdMetricsReport, 0, nil, 99, false)
	if err != nil {
		t.Fatalf("Pack failed: %v", err)
	}
	if len(buf) != int(HeaderSize)+len(payload)+int(FooterSize) {
		t.Fatalf("unexpected packet len: %d", len(buf))
	}
	pkt, err := codec.Unpack(bytes.NewReader(buf), nil)
	if err != nil {
		t.Fatalf("Unpack failed: %v", err)
	}
	if pkt.CmdID != CmdMetricsReport || pkt.Sequence != 99 || pkt.IsAck || pkt.IsEncrypted || !bytes.Equal(pkt.Payload, payload) {
		t.Fatalf("unexpected packet: %+v", pkt)
	}
}

func TestPackUnpackEncryptedAck(t *testing.T) {
	codec := NewCodec()
	key := testKey(t)
	payload := []byte("secret")
	buf, err := codec.Pack(payload, CmdConfigPush, 7, key, 100, true)
	if err != nil {
		t.Fatalf("Pack failed: %v", err)
	}
	pkt, err := codec.Unpack(bytes.NewReader(buf), key)
	if err != nil {
		t.Fatalf("Unpack failed: %v", err)
	}
	if pkt.CmdID != CmdConfigPush || pkt.KeyID != 7 || pkt.Sequence != 100 || !pkt.IsAck || !pkt.IsEncrypted || !bytes.Equal(pkt.Payload, payload) {
		t.Fatalf("unexpected packet: %+v", pkt)
	}
}

func TestEncryptedNonceUsesDirectionAndSequence(t *testing.T) {
	codec := NewCodec()
	key := testKey(t)
	up, err := codec.Pack([]byte("up"), CmdMetricsReport, 7, key, 55, false)
	if err != nil {
		t.Fatalf("Pack uplink failed: %v", err)
	}
	down, err := codec.Pack([]byte("down"), CmdMetricsReport, 7, key, 55, true)
	if err != nil {
		t.Fatalf("Pack downlink ack failed: %v", err)
	}
	if up[16] != 0x01 || down[16] != 0x02 {
		t.Fatalf("unexpected nonce direction bytes: up=%#x down=%#x", up[16], down[16])
	}
	if !bytes.Equal(up[20:28], down[20:28]) {
		t.Fatalf("same sequence should be encoded identically")
	}
	if bytes.Equal(up[16:28], down[16:28]) {
		t.Fatal("opposite directions with same sequence must not reuse nonce")
	}
}

func TestUnpackRejectsCorruption(t *testing.T) {
	codec := NewCodec()
	buf, err := codec.Pack([]byte("important"), CmdLogReport, 0, nil, 1, false)
	if err != nil {
		t.Fatalf("Pack failed: %v", err)
	}
	buf[6] ^= 0xFF
	if _, err := codec.Unpack(bytes.NewReader(buf), nil); err == nil {
		t.Fatal("expected header crc error")
	}

	buf, err = codec.Pack([]byte("important"), CmdLogReport, 0, nil, 1, false)
	if err != nil {
		t.Fatalf("Pack failed: %v", err)
	}
	buf[HeaderSize] ^= 0xFF
	if _, err := codec.Unpack(bytes.NewReader(buf), nil); err == nil {
		t.Fatal("expected payload crc error")
	}
}

func TestUnpackRejectsTooLargePayload(t *testing.T) {
	fakeHeader := make([]byte, HeaderSize)
	binary.LittleEndian.PutUint16(fakeHeader[0:], MagicNumber)
	binary.LittleEndian.PutUint32(fakeHeader[12:], MaxPayloadSize+1)
	crc := crc16Modbus(fakeHeader[:28])
	binary.LittleEndian.PutUint16(fakeHeader[28:], crc)
	if _, err := NewCodec().Unpack(bytes.NewReader(fakeHeader), nil); err == nil {
		t.Fatal("expected too large payload error")
	}
}

func TestPackRejectsTooLargePayload(t *testing.T) {
	payload := make([]byte, MaxPayloadSize+1)
	if _, err := NewCodec().Pack(payload, CmdLogReport, 0, nil, 1, false); err == nil {
		t.Fatal("expected too large payload error")
	}
}

func TestCodecIsStatelessForConcurrentPack(t *testing.T) {
	codec := NewCodec()
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(seq uint64) {
			defer wg.Done()
			if _, err := codec.Pack([]byte("data"), CmdEventReport, 0, nil, seq, false); err != nil {
				t.Errorf("Pack failed: %v", err)
			}
		}(uint64(i))
	}
	wg.Wait()
}

func FuzzUnpackDoesNotPanic(f *testing.F) {
	codec := NewCodec()
	key := bytes.Repeat([]byte{0x42}, 32)
	plain, err := codec.Pack([]byte("seed"), CmdEventReport, 0, nil, 1, false)
	if err != nil {
		f.Fatalf("Pack plain seed failed: %v", err)
	}
	encrypted, err := codec.Pack([]byte("secret"), CmdMetricsReport, 1, key, 2, false)
	if err != nil {
		f.Fatalf("Pack encrypted seed failed: %v", err)
	}
	f.Add([]byte{})
	f.Add(plain)
	f.Add(encrypted)
	f.Fuzz(func(t *testing.T, data []byte) {
		_, _ = codec.Unpack(bytes.NewReader(data), key)
	})
}
