#include "GosterProtocol.h"
#include "ProtocolUtils.h"

GosterProtocol::GosterProtocol(NetworkManager &net, CryptoLayer &crypto, ConfigManager &config)
    : _net(net), _crypto(crypto), _config(config) {
}

void GosterProtocol::begin() {
    _state = STATE_DISCONNECTED;
}

void GosterProtocol::loop() {
    // 1. WiFi 检查
    if (!_net.isConnected()) {
        _state = STATE_DISCONNECTED;
        return;
    }

    WiFiClient *client = _net.getClient();

    // 2. 如果有待发送数据且串口空闲超过500ms，自动建立连接
    if (!client->connected() && !_tx_queue.empty()) {
        if (millis() - _last_rx_activity < 500) {
            // 还在接收数据，等待...
            return;
        }
        
        Serial.println("串口接收空闲，开始连接发送...");
        
        AppConfig cfg = _config.loadConfig();

        // 简单的互联网连接检查
        if (!_net.checkInternet()) {
            Serial.println("等待网络就绪...");
            delay(1000);
            return;
        }

        Serial.printf("正在连接到 %s:%d 以处理待发送任务...\n", cfg.server_ip.c_str(), cfg.server_port);
        if (_net.connectServer(cfg.server_ip, cfg.server_port)) {
            Serial.println("TCP 连接成功!");
            _state = STATE_DISCONNECTED; // 将在 handleStateLogic 中触发握手
            _last_activity = millis();
        } else {
            Serial.println("TCP 连接失败! 2秒后重试...");
            delay(2000);
            return;
        }
    }

    // 3. 空闲自动断开连接
    if (client->connected() && _state == STATE_READY && _tx_queue.empty()) {
        if (millis() - _last_activity > 2000) { // 2秒空闲超时 (快速断开)
            Serial.println("任务完成，主动断开连接.");
            client->stop();
            _state = STATE_DISCONNECTED;
        }
    }

    // 4. 协议处理
    if (client->connected()) {
        handleStateLogic();
        processIncomingData();

        // 如果就绪，刷新缓冲区
        if (_state == STATE_READY && !_tx_queue.empty()) {
            Serial.printf("刷新队列 (大小: %d)\n", _tx_queue.size());
            
            // 获取最旧的数据包
            const std::vector<uint8_t>& pkt = _tx_queue.front();
            sendFrame(CMD_METRICS_REPORT, pkt.data(), pkt.size(), true);
            
            // 移除已发送的数据包
            _tx_queue.pop_front();
            
            _last_activity = millis(); // 刷新活动时间
        }
    }
}

void GosterProtocol::handleStateLogic() {
    switch (_state) {
        case STATE_DISCONNECTED:
            // TCP 连接后立即发起握手
            if (!_crypto.generateKeyPair()) {
                Serial.println("密钥生成失败! 断开连接。");
                _net.getClient()->stop();
                return;
            } // 为新会话重新生成密钥
            sendHandshake();
            _state = STATE_HANDSHAKE_SENT;
            Serial.println("状态: 已发送握手 (HANDSHAKE_SENT)");
            _last_activity = millis();
            break;

        case STATE_READY:
            // 短连接无需心跳
            break;

        default:
            break;
    }
}

void GosterProtocol::processIncomingData() {
    WiFiClient *client = _net.getClient();
    while (client->available()) {
        _last_activity = millis(); // 接收数据时更新活动状态

        int r = client->read(_rx_buffer + _rx_len, sizeof(_rx_buffer) - _rx_len);
        if (r > 0) _rx_len += r;

        // 尝试解析帧
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
            Serial.println("解密失败!");
            return;
        }
    }

    switch (header.cmd_id) {
        case CMD_HANDSHAKE_RESP:
            Serial.println("RX: 握手响应 (Handshake Resp)");
            if (_crypto.computeSharedSecret(process_ptr)) {
                Serial.println("共享密钥计算完成。");
                sendAuth();
                _state = STATE_AUTH_SENT;
            }
            break;

        case CMD_AUTH_ACK:
            Serial.println("RX: 认证确认 (Auth ACK)");
            if (process_ptr[0] == 0x00) {
                Serial.println("认证成功! 就绪。");
                _state = STATE_READY;
            } else {
                Serial.printf("认证失败: %02X\n", process_ptr[0]);
                _net.getClient()->stop();
                // 关键: 如果认证失败，停止重试
                _tx_queue.clear();
            }
            break;

        case CMD_CONFIG_PUSH:
            Serial.println("RX: 配置推送 (Config Push)");
            break;
            
        case CMD_METRICS_REPORT: // 指标上报确认
            Serial.println("RX: 指标确认 (Metrics ACK)");
            // 事务完成
            break;
    }
}

// --- 发送函数 ---

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
    Serial.println("TX: 心跳 (Heartbeat)");
    sendFrame(CMD_HEARTBEAT, nullptr, 0, true);
}

// 公共 API: 仅缓冲数据
void GosterProtocol::sendMetricReport(const uint8_t *payload, size_t len) {
    if (len == 0) return;

    // 检查队列限制
    if (_tx_queue.size() >= MAX_TX_QUEUE_SIZE) {
        Serial.println("队列已满，丢弃最旧的数据包!");
        _tx_queue.pop_front();
    }

    // 推入新数据包
    std::vector<uint8_t> pkt(payload, payload + len);
    _tx_queue.push_back(pkt);
    
    _last_rx_activity = millis(); // 更新接收活动时间
    Serial.printf("指标已入队。队列大小: %d\n", _tx_queue.size());
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

        // 计算 CRC32 (Header + Payload) 单次通过
        CRC32 crc;
        crc.setPolynome(0x04C11DB7);
        crc.setInitial(0xFFFFFFFF);
        crc.setXorOut(0xFFFFFFFF); // 标准 IEEE 802.3
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