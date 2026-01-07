#pragma once

#include <Arduino.h>
#include <deque>
#include <vector>
#include "NetworkManager.h"
#include "CryptoLayer.h"
#include "ConfigManager.h"

// Protocol Constants
#define GOSTER_MAGIC    0x5759
#define GOSTER_VERSION  0x01
#define MAX_TX_QUEUE_SIZE 10 // 最大缓存 10 个数据包，满后覆盖旧数据

// 编译时推导缓冲区大小 (与 Rust 端逻辑保持一致)
// 1. 定义核心变量
constexpr size_t MAX_SAMPLES = 128; 

// 2. 计算各部分大小
constexpr size_t SZ_METRIC_HEADER = 17; // start_ts(8) + interval(4) + type(1) + count(4)
constexpr size_t SZ_FLOAT = 4;
constexpr size_t SZ_PAYLOAD = SZ_METRIC_HEADER + (MAX_SAMPLES * SZ_FLOAT);

constexpr size_t SZ_PROTO_HEADER = 32;
constexpr size_t SZ_PROTO_FOOTER = 16;
constexpr size_t SZ_RAW_FRAME = SZ_PROTO_HEADER + SZ_PAYLOAD + SZ_PROTO_FOOTER;

// 3. 计算 COBS 编码最大膨胀 (每 254 字节增加 1 字节 overhead，加上首尾 0x00)
constexpr size_t COBS_OVERHEAD = (SZ_RAW_FRAME / 254) + 2;

// 4. 最终推导出的接收缓冲区大小 (留少量余量对齐)
constexpr size_t RX_BUFFER_SIZE = SZ_RAW_FRAME + COBS_OVERHEAD + 16; 

// Flags
#define FLAG_ACK        0x01
#define FLAG_ENCRYPTED  0x02
#define FLAG_COMPRESSED 0x04

// Command IDs
#define CMD_HANDSHAKE_INIT  0x0001
#define CMD_HANDSHAKE_RESP  0x0002
#define CMD_AUTH_VERIFY     0x0003
#define CMD_AUTH_ACK        0x0004
#define CMD_DEVICE_REGISTER 0x0005
#define CMD_METRICS_REPORT  0x0101
#define CMD_HEARTBEAT       0x0104
#define CMD_CONFIG_PUSH     0x0201
#define CMD_TIME_SYNC       0x0204

// Header Structure (32 Bytes, Packed)
#pragma pack(push, 1)
struct GosterHeader {
    uint16_t magic;
    uint8_t  version;
    uint8_t  flags;
    uint16_t status;
    uint16_t cmd_id;
    uint32_t key_id;
    uint32_t length;     // Payload length
    uint8_t  nonce[12];  // AES-GCM IV
    uint16_t h_crc16;    // Header CRC
    uint16_t padding;
};
#pragma pack(pop)

enum ProtocolState {
    STATE_DISCONNECTED,
    STATE_HANDSHAKE_SENT,
    STATE_AUTH_SENT,
    STATE_READY
};

class GosterProtocol {
public:
    GosterProtocol(NetworkManager& net, CryptoLayer& crypto, ConfigManager& config);
    
    void begin();
    void loop();

    // 发送来自 STM32 的传感器数据
    // payload: 指向 Postcard 解码后的二进制数据
    // len: 数据长度
    void sendMetricReport(const uint8_t* payload, size_t len);

private:
    NetworkManager& _net;
    CryptoLayer& _crypto;
    ConfigManager& _config;
    
    ProtocolState _state = STATE_DISCONNECTED;
    uint8_t _rx_buffer[1024]; // TCP Receive Buffer
    size_t _rx_len = 0;
    
    // TX Queue for buffering metrics
    std::deque<std::vector<uint8_t>> _tx_queue;
    
    unsigned long _last_activity = 0;
    uint64_t _tx_sequence = 0; // For Nonce generation

    // Internal Helpers
    void handleStateLogic();
    void sendHandshake();
    void sendAuth();
    void sendHeartbeat();
    
    // Packet Processing
    void processIncomingData();
    void handlePacket(const GosterHeader& header, const uint8_t* payload, size_t len);

    // Low-level Send
    void sendFrame(uint16_t cmd_id, const uint8_t* data, size_t len, bool encrypted);
    
    // Utils
    void generateNonce(uint8_t* nonce_out);
};
