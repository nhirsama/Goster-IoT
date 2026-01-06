#include "GosterProtocol.h"
#include "ProtocolUtils.h"

GosterProtocol::GosterProtocol(NetworkManager &net, CryptoLayer &crypto, ConfigManager &config)
    : _net(net), _crypto(crypto), _config(config) {
}

void GosterProtocol::begin() {
    _state = STATE_DISCONNECTED;
}

void GosterProtocol::loop() {
    // 1. WiFi Check
    if (!_net.isConnected()) {
        _state = STATE_DISCONNECTED;
        return;
    }

    WiFiClient *client = _net.getClient();

    // 2. Auto-Connect if we have pending data
    if (!client->connected() && !_tx_queue.empty()) {
        AppConfig cfg = _config.loadConfig();

        // Simple internet check
        if (!_net.checkInternet()) {
            Serial.println("等待网络就绪...");
            delay(1000);
            return;
        }

        Serial.printf("Connecting to %s:%d for pending task...\n", cfg.server_ip.c_str(), cfg.server_port);
        if (_net.connectServer(cfg.server_ip, cfg.server_port)) {
            Serial.println("TCP 连接成功!");
            _state = STATE_DISCONNECTED; // Will trigger Handshake in handleStateLogic
            _last_activity = millis();
        } else {
            Serial.println("TCP 连接失败! 2秒后重试...");
            delay(2000);
            return;
        }
    }

    // 3. Auto-Disconnect if idle
    if (client->connected() && _state == STATE_READY && _tx_queue.empty()) {
        if (millis() - _last_activity > 2000) { // 2s Idle Timeout (Quick disconnect)
            Serial.println("任务完成，主动断开连接.");
            client->stop();
            _state = STATE_DISCONNECTED;
        }
    }

    // 4. Protocol Processing
    if (client->connected()) {
        handleStateLogic();
        processIncomingData();

        // Flush Buffer if Ready
        if (_state == STATE_READY && !_tx_queue.empty()) {
            Serial.printf("Flushing queue (Size: %d)\n", _tx_queue.size());
            
            // Get the oldest packet
            const std::vector<uint8_t>& pkt = _tx_queue.front();
            sendFrame(CMD_METRICS_REPORT, pkt.data(), pkt.size(), true);
            
            // Remove sent packet
            _tx_queue.pop_front();
            
            _last_activity = millis(); // Refresh activity
        }
    }
}

void GosterProtocol::handleStateLogic() {
    switch (_state) {
        case STATE_DISCONNECTED:
            // Initiate Handshake immediately after TCP connect
            _crypto.generateKeyPair(); // Regen keys for new session
            sendHandshake();
            _state = STATE_HANDSHAKE_SENT;
            Serial.println("State: HANDSHAKE_SENT");
            _last_activity = millis();
            break;

        case STATE_READY:
            // No Heartbeat needed for short-lived connections
            break;

        default:
            break;
    }
}

void GosterProtocol::processIncomingData() {
    WiFiClient *client = _net.getClient();
    while (client->available()) {
        _last_activity = millis(); // Update activity on RX

        int r = client->read(_rx_buffer + _rx_len, sizeof(_rx_buffer) - _rx_len);
        if (r > 0) _rx_len += r;

        // Try to parse frame
        if (_rx_len >= sizeof(GosterHeader)) {
            GosterHeader *header = (GosterHeader *) _rx_buffer;

            if (header->magic != GOSTER_MAGIC) {
                Serial.printf("无效 Magic: %04X. 断开连接.\n", header->magic);
                client->stop();
                _rx_len = 0;
                return;
            }

            uint16_t calcCRC = ProtocolUtils::calculateCRC16((uint8_t *) header, 28);
            if (calcCRC != header->h_crc16) {
                Serial.printf("Header CRC 错误: 期望 %04X, 实际 %04X\n", header->h_crc16, calcCRC);
                client->stop();
                _rx_len = 0;
                return;
            }

            uint32_t payload_len = header->length;
            size_t total_frame_size = sizeof(GosterHeader) + payload_len + 16;

            if (_rx_len >= total_frame_size) {
                handlePacket(*header, _rx_buffer + sizeof(GosterHeader), payload_len);

                size_t remaining = _rx_len - total_frame_size;
                memmove(_rx_buffer, _rx_buffer + total_frame_size, remaining);
                _rx_len = remaining;
            }
        }
    }
}

void GosterProtocol::handlePacket(const GosterHeader &header, const uint8_t *payload_in, size_t len) {
    bool is_encrypted = header.flags & FLAG_ENCRYPTED;
    uint8_t plain_payload[1024];
    const uint8_t *process_ptr = payload_in;

    if (is_encrypted) {
        const uint8_t *tag = payload_in + len;
        if (_crypto.decrypt(payload_in, len, (uint8_t *) &header, 28, plain_payload, tag, header.nonce)) {
            process_ptr = plain_payload;
        } else {
            Serial.println("Decryption Failed!");
            return;
        }
    }

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
            if (process_ptr[0] == 0x00) {
                Serial.println("Auth Success! Ready.");
                _state = STATE_READY;
            } else {
                Serial.printf("Auth Failed: %02X\n", process_ptr[0]);
                _net.getClient()->stop();
                // Critical: Stop retrying if Auth failed
                _tx_queue.clear();
            }
            break;

        case CMD_CONFIG_PUSH:
            Serial.println("RX: Config Push");
            break;
            
        case CMD_METRICS_REPORT: // ACK for Metrics
            Serial.println("RX: Metrics ACK");
            // Transaction complete
            break;
    }
}

// --- Senders ---

void GosterProtocol::sendHandshake() {
    const uint8_t *pub = _crypto.getPublicKey();
    sendFrame(CMD_HANDSHAKE_INIT, pub, 32, false);
}

void GosterProtocol::sendAuth() {
    AppConfig cfg = _config.loadConfig();
    if (cfg.is_registered) {
        sendFrame(CMD_AUTH_VERIFY, (uint8_t *) cfg.device_token.c_str(), cfg.device_token.length(), true);
    } else {
        String reg_data = String("ESP32-Device") + "\x1E" +
                          "SN123456" + "\x1E" +
                          WiFi.macAddress() + "\x1E" +
                          "1.0" + "\x1E" +
                          "1.0" + "\x1E" +
                          "1";
        sendFrame(CMD_DEVICE_REGISTER, (uint8_t *) reg_data.c_str(), reg_data.length(), true);
    }
}

void GosterProtocol::sendHeartbeat() {
    Serial.println("TX: Heartbeat");
    sendFrame(CMD_HEARTBEAT, nullptr, 0, true);
}

// Public API: Just buffer the data
void GosterProtocol::sendMetricReport(const uint8_t *payload, size_t len) {
    if (len == 0) return;

    // Check queue limit
    if (_tx_queue.size() >= MAX_TX_QUEUE_SIZE) {
        Serial.println("Queue full, dropping oldest packet!");
        _tx_queue.pop_front();
    }

    // Push new packet
    std::vector<uint8_t> pkt(payload, payload + len);
    _tx_queue.push_back(pkt);
    
    Serial.printf("Metric queued. Queue size: %d\n", _tx_queue.size());
}

void GosterProtocol::sendFrame(uint16_t cmd_id, const uint8_t *data, size_t len, bool encrypted) {
    GosterHeader header;
    memset(&header, 0, sizeof(header));

    header.magic = GOSTER_MAGIC;
    header.version = GOSTER_VERSION;
    header.flags = 0;
    header.cmd_id = cmd_id;
    header.length = len;

    if (encrypted) header.flags |= FLAG_ENCRYPTED;

    generateNonce(header.nonce);
    header.h_crc16 = ProtocolUtils::calculateCRC16((uint8_t *) &header, 28);

    WiFiClient *client = _net.getClient();

    if (encrypted) {
        uint8_t cipher[1024];
        uint8_t tag[16];

        if (!_crypto.encrypt(data, len, (uint8_t *) &header, 28, cipher, tag, header.nonce)) {
            Serial.printf("API: 加密失败 (Cmd: %04X)! 放弃发送。\n", cmd_id);
            return;
        }

        client->write((uint8_t *) &header, sizeof(header));
        if (len > 0) client->write(cipher, len);
        client->write(tag, 16);
    } else {
        client->write((uint8_t *) &header, sizeof(header));
        if (len > 0) client->write(data, len);

        // Calculate CRC32 (Header + Payload) using single pass
        CRC32 crc;
        crc.setPolynome(0x04C11DB7);
        crc.setInitial(0xFFFFFFFF);
        crc.setXorOut(0xFFFFFFFF); // Standard IEEE 802.3
        crc.setReverseIn(true);
        crc.setReverseOut(true);
        crc.restart();

        crc.add((uint8_t *) &header, sizeof(header));
        if (len > 0) crc.add(data, len);
        
        uint32_t sum = crc.calc();

        uint8_t footer[16] = {0};
        memcpy(footer, &sum, 4);
        client->write(footer, 16);
    }
}

void GosterProtocol::generateNonce(uint8_t *nonce_out) {
    memset(nonce_out, 0, 12);
    _tx_sequence++;
    memcpy(nonce_out + 4, &_tx_sequence, 8);
}