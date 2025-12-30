#include "GosterProtocol.h"

GosterProtocol::GosterProtocol(NetworkManager& net, CryptoLayer& crypto, ConfigManager& config)
    : _net(net), _crypto(crypto), _config(config) {}

void GosterProtocol::begin() {
    _state = STATE_DISCONNECTED;
}

void GosterProtocol::loop() {
    // 1. Connection Check
    if (!_net.isConnected()) {
        _state = STATE_DISCONNECTED;
        return;
    }

    // 2. TCP Client Connection
    if (!_net.getClient()->connected()) {
        // Only load config when we actually need to connect
        AppConfig cfg = _config.loadConfig();

        // 先检查是否真的连上了互联网
        if (!_net.checkInternet()) {
            Serial.println("等待网络就绪...");
            delay(2000); // 稍作等待再重试
            return;
        }

        Serial.printf("正在连接服务器 %s:%d ...\n", cfg.server_ip.c_str(), cfg.server_port);
        if (_net.connectServer(cfg.server_ip, cfg.server_port)) {
            Serial.println("TCP 连接成功!");
            // Reset State to start Handshake
            _state = STATE_DISCONNECTED; 
        } else {
            Serial.println("TCP 连接失败! 3秒后重试...");
            delay(3000); // 防止刷屏
            return; // Retry next loop
        }
    }

    // 3. State Machine
    handleStateLogic();

    // 4. Process Incoming TCP Data
    processIncomingData();
}

void GosterProtocol::handleStateLogic() {
    switch (_state) {
        case STATE_DISCONNECTED:
            // Initiate Handshake immediately after TCP connect
            _crypto.generateKeyPair(); // Regen keys for new session
            sendHandshake();
            _state = STATE_HANDSHAKE_SENT;
            Serial.println("State: HANDSHAKE_SENT");
            break;
            
        case STATE_READY:
            // Send Heartbeat every 30s
            if (millis() - _last_heartbeat > 30000) {
                sendHeartbeat();
                _last_heartbeat = millis();
            }
            break;
            
        default:
            break;
    }
}

void GosterProtocol::processIncomingData() {
    WiFiClient* client = _net.getClient();
    while (client->available()) {
        // Simple buffer handling (In production, use a ring buffer)
        int r = client->read(_rx_buffer + _rx_len, sizeof(_rx_buffer) - _rx_len);
        if (r > 0) _rx_len += r;
        
        // Try to parse frame
        if (_rx_len >= sizeof(GosterHeader)) {
            GosterHeader* header = (GosterHeader*)_rx_buffer;
            
            // Validate Magic
            if (header->magic != GOSTER_MAGIC) {
                Serial.printf("无效 Magic: %04X. 断开连接.\n", header->magic);
                client->stop();
                _rx_len = 0;
                return;
            }

            // Validate Header CRC16 (Offset 0-27)
            uint16_t calcCRC = calculateCRC16((uint8_t*)header, 28);
            if (calcCRC != header->h_crc16) {
                Serial.printf("Header CRC 错误: 期望 %04X, 实际 %04X\n", header->h_crc16, calcCRC);
                client->stop();
                _rx_len = 0;
                return;
            }

            uint32_t payload_len = header->length;
            // +16 Footer (Tag or CRC32)
            size_t total_frame_size = sizeof(GosterHeader) + payload_len + 16; 

            if (_rx_len >= total_frame_size) {
                // We have a full frame
                handlePacket(*header, _rx_buffer + sizeof(GosterHeader), payload_len);
                
                // Shift buffer (Remove processed frame)
                size_t remaining = _rx_len - total_frame_size;
                memmove(_rx_buffer, _rx_buffer + total_frame_size, remaining);
                _rx_len = remaining;
            }
        }
    }
}
void GosterProtocol::handlePacket(const GosterHeader& header, const uint8_t* payload_in, size_t len) {
    bool is_encrypted = header.flags & FLAG_ENCRYPTED;
    uint8_t plain_payload[1024];
    const uint8_t* process_ptr = payload_in;
    size_t process_len = len;

    // Decrypt if needed
    if (is_encrypted) {
        // Tag is in Footer (first 16 bytes after payload)
        const uint8_t* tag = payload_in + len; 
        // AAD is first 28 bytes of header
        if (_crypto.decrypt(payload_in, len, (uint8_t*)&header, 28, plain_payload, tag, header.nonce)) {
            process_ptr = plain_payload;
        } else {
            Serial.println("Decryption Failed!");
            return;
        }
    }

    // Handle Commands
    switch (header.cmd_id) {
        case CMD_HANDSHAKE_RESP:
            Serial.println("RX: Handshake Resp");
            if (_crypto.computeSharedSecret(process_ptr)) {
                Serial.println("Shared Key Computed.");
                sendAuth();
                _state = STATE_AUTH_SENT;
            }
            break;

        case CMD_AUTH_ACK:
            Serial.println("RX: Auth ACK");
            // Payload[0] is status
            if (process_ptr[0] == 0x00) {
                Serial.println("Auth Success! Ready.");
                _state = STATE_READY;
                // If Token is new, save it (Logic omitted for brevity)
            } else {
                Serial.printf("Auth Failed: %02X\n", process_ptr[0]);
                _net.getClient()->stop();
            }
            break;
            
        case CMD_CONFIG_PUSH:
            Serial.println("RX: Config Push (TODO)");
            break;
    }
}

// --- Senders ---

void GosterProtocol::sendHandshake() {
    const uint8_t* pub = _crypto.getPublicKey();
    sendFrame(CMD_HANDSHAKE_INIT, pub, 32, false);
}

void GosterProtocol::sendAuth() {
    AppConfig cfg = _config.loadConfig();
    if (cfg.is_registered) {
        // Send Token
        sendFrame(CMD_AUTH_VERIFY, (uint8_t*)cfg.device_token.c_str(), cfg.device_token.length(), true);
    } else {
        // Send Registration
        // Using string concatenation to prevent "hex escape sequence out of range"
        // 0x1E is RS (Record Separator)
        String reg_data = String("ESP32-Device") + "\x1E" + 
                          "SN123456" + "\x1E" + 
                          WiFi.macAddress() + "\x1E" + 
                          "1.0" + "\x1E" + 
                          "1.0" + "\x1E" + 
                          "1";
        sendFrame(CMD_DEVICE_REGISTER, (uint8_t*)reg_data.c_str(), reg_data.length(), true);
    }
}

void GosterProtocol::sendHeartbeat() {
    sendFrame(CMD_HEARTBEAT, nullptr, 0, true);
}

void GosterProtocol::sendMetricReport(const uint8_t* payload, size_t len) {
    if (_state != STATE_READY) return;
    Serial.printf("Sending Metric Report (%d bytes)\n", len);
    sendFrame(CMD_METRICS_REPORT, payload, len, true);
}

// ... (previous code)

void GosterProtocol::sendFrame(uint16_t cmd_id, const uint8_t* data, size_t len, bool encrypted) {
    GosterHeader header;
    memset(&header, 0, sizeof(header));
    
    header.magic = GOSTER_MAGIC;
    header.version = GOSTER_VERSION;
    header.flags = 0; // Request
    header.cmd_id = cmd_id;
    header.length = len;
    
    // Encrypt Flag
    if (encrypted) header.flags |= FLAG_ENCRYPTED;
    
    // Generate Nonce
    generateNonce(header.nonce);
    
    // Calc Header CRC
    header.h_crc16 = calculateCRC16((uint8_t*)&header, 28); // Offset 0-27

    WiFiClient* client = _net.getClient();
    
    if (encrypted) {
        uint8_t cipher[1024];
        uint8_t tag[16];
        
        // 1. 先尝试加密
        if (!_crypto.encrypt(data, len, (uint8_t*)&header, 28, cipher, tag, header.nonce)) {
            Serial.printf("API: 加密失败 (Cmd: %04X)! 放弃发送。\n", cmd_id);
            return;
        }

        // --- DEBUG HEX DUMP ---
        Serial.printf("TX Encrypted Frame (Cmd: %04X, Len: %d)\n", cmd_id, len);
        Serial.print("Nonce: "); 
        for(int i=0; i<12; i++) Serial.printf("%02X ", header.nonce[i]);
        Serial.println();
        Serial.print("Tag: "); 
        for(int i=0; i<16; i++) Serial.printf("%02X ", tag[i]);
        Serial.println();
        // ----------------------
        
        // 2. 加密成功后，再一次性发送 Header + Cipher + Tag
        // 这样保证服务器不会收到半个包
        client->write((uint8_t*)&header, sizeof(header));
        if (len > 0) client->write(cipher, len);
        client->write(tag, 16); 
        
    } else {
        // Plain Mode
        client->write((uint8_t*)&header, sizeof(header));
        if (len > 0) client->write(data, len);
        
        // Footer = CRC32 + Padding
        uint32_t crc = calculateCRC32((uint8_t*)&header, sizeof(header), 0xFFFFFFFF);
        if (len > 0) {
            crc = calculateCRC32(data, len, crc);
        }
        crc = ~crc; // Final XOR

        uint8_t footer[16] = {0};
        memcpy(footer, &crc, 4); // Little Endian
        client->write(footer, 16);
    }
}

void GosterProtocol::generateNonce(uint8_t* nonce_out) {
    // Simple nonce: 4 bytes random salt + 8 bytes sequence
    // Here strictly incrementing for simplicity
    memset(nonce_out, 0, 12);
    _tx_sequence++;
    memcpy(nonce_out + 4, &_tx_sequence, 8);
}

// CRC16 Modbus Table (Matches Go Server)
const uint16_t crc16Table[] = {
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
};

uint16_t GosterProtocol::calculateCRC16(const uint8_t* data, size_t len) {
    uint16_t crc = 0xFFFF;
    for (size_t i = 0; i < len; i++) {
        // Match Go: crc = (crc >> 8) ^ crc16Table[(crc ^ byte) & 0xFF]
        crc = (crc >> 8) ^ crc16Table[(crc ^ data[i]) & 0xFF];
    }
    return crc;
}

// IEEE 802.3 CRC32
uint32_t GosterProtocol::calculateCRC32(const uint8_t* data, size_t len, uint32_t current_crc) {
    uint32_t crc = current_crc; // Should init with 0xFFFFFFFF for first block
    for (size_t i = 0; i < len; i++) {
        crc ^= data[i];
        for (int j = 0; j < 8; j++) {
            if (crc & 1) crc = (crc >> 1) ^ 0xEDB88320;
            else crc >>= 1;
        }
    }
    return crc; // Don't invert here, allow chaining. Invert at end.
}

uint32_t GosterProtocol::calculateCRC32(const uint8_t* data, size_t len) {
    return ~calculateCRC32(data, len, 0xFFFFFFFF);
}
