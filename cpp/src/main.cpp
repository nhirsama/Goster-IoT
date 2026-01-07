#include <Arduino.h>
#include "ConfigManager.h"
#include "Hardware.h"
#include "NetworkManager.h"
#include "CryptoLayer.h"
#include "GosterProtocol.h"
#include "SerialBridge.h"

constexpr uint16_t IDLE_TIMEOUT_MS = 2000;
// Modules
ConfigManager configMgr;
Hardware hw;
NetworkManager netMgr(configMgr);
CryptoLayer crypto;
GosterProtocol protocol(netMgr, crypto, configMgr);
SerialBridge serialBridge;

// Global Flags
bool g_timeSynced = false;

unsigned long lastActivityTime = 0;

// 匹配 STM32 发送的结构体 (packed 以防对齐问题)
struct __attribute__((packed)) SensorPacket {
    int8_t temperature;
    uint8_t humidity;
    float lux;
};

// 发送时间同步指令给 STM32
void sendTimeSyncToSTM32() {
    int64_t ts = NetworkManager::getTimestamp();
    if (ts == 0) return;

    // Payload: 8 bytes timestamp (Little Endian)
    uint8_t payload[8];
    memcpy(payload, &ts, 8); // ESP32 is Little Endian usually, but confirm. 
    // NetworkManager::getTimestamp returns int64_t.
    // Let's ensure LE explicitly if needed, but memcpy is fine for same-endian archs.
    // ESP32 is LE.

    // Construct Frame
    // 1. Header
    GosterHeader header;
    memset(&header, 0, sizeof(header));
    header.magic = GOSTER_MAGIC;
    header.version = GOSTER_VERSION;
    header.cmd_id = CMD_TIME_SYNC;
    header.length = 8; // payload len

    // Nonce/Seq: Just use a static or random for Serial? 
    // Serial link security is relaxed (unencrypted).
    // Just zero is fine or increment a local counter.
    static uint64_t serial_seq = 0;
    serial_seq++;
    memcpy(header.nonce + 4, &serial_seq, 8);

    // Header CRC16
    header.h_crc16 = ProtocolUtils::calculateCRC16((uint8_t *) &header, 28);

    // 2. Footer (CRC32 of Header + Payload)
    uint32_t crc32 = ProtocolUtils::calculateCRC32((uint8_t *) &header, sizeof(header));
    // ProtocolUtils::calculateCRC32 doesn't support continue.
    // We need to concat or modify ProtocolUtils. 
    // Since we can't easily modify ProtocolUtils (User restriction), let's copy to a temp buffer.
    uint8_t frameBuf[32 + 8 + 16]; // 56 bytes
    memcpy(frameBuf, &header, 32);
    memcpy(frameBuf + 32, payload, 8);

    // Recalculate CRC32 on the contiguous buffer [Header + Payload]
    crc32 = ProtocolUtils::calculateCRC32(frameBuf, 32 + 8);

    // Footer
    uint8_t footer[16] = {0};
    memcpy(footer, &crc32, 4);
    memcpy(frameBuf + 40, footer, 16);

    // 3. Send via PacketSerial (Handles COBS encoding)
    // PacketSerial::send(buffer, size) wraps it in 0x00 and escapes bytes.
    hw.getPacketSerial().send(frameBuf, 56);
    Serial.printf("[TimeSync] Sent Timestamp: %lld to STM32\n", ts);
}

// 对应 Rust 端 MetricReport 的 Payload 结构 (紧凑布局)
struct __attribute__((packed)) MetricReportHeader {
    uint64_t start_timestamp;
    uint32_t sample_interval;
    uint8_t data_type;
    uint32_t count;
};

// Callback: Validated Packet from SerialBridge
void onValidPacket(uint16_t cmdId, const uint8_t *payload, size_t len) {
    switch (cmdId) {
        case 0x0101: {
            // CMD_METRICS_REPORT
            if (len < sizeof(MetricReportHeader)) {
                Serial.printf("[RX] 错误：数据包过短 (%d)\n", len);
                return;
            }

            const MetricReportHeader *header = (const MetricReportHeader *) payload;
            const float *data_points = (const float *) (payload + sizeof(MetricReportHeader));

            const char *type_str = "未知";
            if (header->data_type == 0x01) type_str = "温度";
            else if (header->data_type == 0x02) type_str = "湿度";
            else if (header->data_type == 0x04) type_str = "光照";

            Serial.printf("[RX] 收到批量%s数据: %d 个点, 起始时间: %llu\n",
                          type_str, header->count, header->start_timestamp);

            if (header->count > 0) {
                Serial.printf("     最新值: %.2f\n", data_points[header->count - 1]);
            }

            hw.blinkLed(1, 50);

            // 转发给服务器
            protocol.sendMetricReport(payload, len);

            // 更新最后活动时间
            lastActivityTime = millis();
            break;
        }

        default:
            Serial.printf("[RX] 收到未知指令: %04X\n", cmdId);
            break;
    }
}

// Callback: STM32 Data Received (COBS Decoded) -> Feed to Bridge
void onPacketReceived(const uint8_t *buffer, size_t size) {
    if (size == 0) {
        // 收到空包 (0x00)，视为唤醒信号
        if (NetworkManager::isTimeValid()) {
            Serial.println("[RX] 收到唤醒信号 (0x00)，回复时间同步响应...");
            sendTimeSyncToSTM32();
            g_timeSynced = true;
        } else {
            Serial.println("[RX] 收到唤醒信号 (0x00)，时间未就绪，回复 'R'...");
            delay(50);
            Serial1.write(0x52);
        }
        return;
    }

    serialBridge.processFrame(buffer, size);
}

// Callback: Button Long Press -> Factory Reset
void onFactoryReset(void *param) {
    Serial.println("!!! 恢复出厂设置已触发 !!!");
    hw.blinkLed(10, 50); // 快速闪烁
    netMgr.resetWiFi(); // 清除 WiFi 信息
    configMgr.clearConfig(); // 清除服务器/Token 信息
    delay(1000);
    ESP.restart();
}

void setup() {
    Serial.begin(115200);
    Serial.println("\n--- Goster-IoT ESP32 网关已启动 ---");

    // 1. 初始化硬件与配置
    configMgr.begin();
    hw.begin();

    delay(100);

    hw.setResetCallback(onFactoryReset, nullptr);
    hw.getPacketSerial().setPacketHandler(&onPacketReceived);

    // Init Serial Bridge
    serialBridge.begin(onValidPacket);

    // 2. 初始化加密模块
    if (!crypto.begin()) {
        Serial.println("加密模块初始化失败!");
        while (true) hw.blinkLed(1, 500);
    }

    // 3. 联网
    netMgr.begin();

    // 4. 协议栈启动
    protocol.begin();

    lastActivityTime = millis();
}

void deep_sleep_start() {
    Serial.println("无活动超时，进入深度睡眠...");
    Serial.flush();
    Serial1.flush(); // 确保数据发完

    // 关闭 WiFi 射频以省电 (Deep Sleep 会自动关闭，但显式调用更安全)
    WiFi.disconnect(true);
    WiFi.mode(WIFI_OFF);

    // 配置 GPIO 唤醒 (GPIO 5 = RX)
    // 唤醒电平: LOW (因为 STM32 发送 0x00 起始位是低电平)
    esp_deep_sleep_enable_gpio_wakeup(1ULL << PIN_UART_RX, ESP_GPIO_WAKEUP_GPIO_LOW);

    Serial.println("已配置 GPIO 5 低电平唤醒，Zzz...");
    delay(100); // 等待打印完成

    esp_deep_sleep_start();
}

void loop() {
    // 处理各个模块的轮询
    hw.update();
    netMgr.loop();
    protocol.loop();

    // 如果 TCP 连接保持中，视为有活动，防止休眠
    if (netMgr.getClient()->connected()) {
        lastActivityTime = millis();
    }

    // 每隔 3 秒检查一次时间同步状态
    static unsigned long lastPrintTime = 0;
    if (millis() - lastPrintTime >= 3000) {
        lastPrintTime = millis();
        time_t now_time;
        time(&now_time);
        Serial.println(now_time);

        // 仅在初次联网且时间有效时，如果还没同步过，发一次
        if (!g_timeSynced && NetworkManager::isTimeValid()) {
            sendTimeSyncToSTM32();
            g_timeSynced = true;
        }
    }
    if (millis() - lastActivityTime > IDLE_TIMEOUT_MS) {
        deep_sleep_start();
    }
}
