#pragma once

#include <Arduino.h>
#include "NetworkManager.h"
#include "CryptoLayer.h"
#include "ConfigManager.h"

// Protocol Constants
#define GOSTER_MAGIC    0x5759
#define GOSTER_VERSION  0x01

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
    
    // TX Buffer for short-lived connection model
    uint8_t _tx_buffer[1024];
    size_t _tx_len = 0;
    bool _has_pending_tx = false;
    
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
    uint16_t calculateCRC16(const uint8_t* data, size_t len);
    uint32_t calculateCRC32(const uint8_t* data, size_t len);
    uint32_t calculateCRC32(const uint8_t* data, size_t len, uint32_t current_crc);
    void generateNonce(uint8_t* nonce_out);
};
