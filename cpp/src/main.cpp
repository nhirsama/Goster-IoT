#include <Arduino.h>
#include "ConfigManager.h"
#include "Hardware.h"
#include "NetworkManager.h"
#include "CryptoLayer.h"
#include "GosterProtocol.h"

// 闲置超时时间，10 秒无活动则休眠
#define IDLE_TIMEOUT_MS 10000
#define TEST_INTERVAL_MS 30000 // 每30秒发送一次测试数据

// Modules
ConfigManager configMgr;
Hardware hw;
NetworkManager netMgr(configMgr);
CryptoLayer crypto;
GosterProtocol protocol(netMgr, crypto, configMgr);

unsigned long lastActivityTime = 0;
unsigned long lastTestTime = 0;

// Callback: STM32 Data Received (COBS Decoded)
void onPacketReceived(const uint8_t *buffer, size_t size) {
    Serial.printf("收到 STM32 数据: %d 字节\n", size);
    hw.blinkLed(1, 50);

    // 转发给服务器
    protocol.sendMetricReport(buffer, size);

    // 更新最后活动时间
    lastActivityTime = millis();
}

// Generate Dummy Data for Testing
void generateTestPacket() {
    // Construct Payload: [Time(8)][Interval(4)][Type(1)][Count(4)][Values(N*4)]
    // Total header = 17 bytes
    const int count = 5;
    uint8_t buffer[128];
    size_t offset = 0;

    // 1. Timestamp (Int64, Little Endian)
    uint64_t now = time(nullptr) * 1000; // Milliseconds
    // ESP32 time might not be synced, but let's use what we have or 0
    if (now < 100000) now = 1735692000000; // Fake 2025-01-01 if not synced
    
    memcpy(buffer + offset, &now, 8); offset += 8;

    // 2. Interval (UInt32) - 1000ms
    uint32_t interval = 1000000; // microseconds
    memcpy(buffer + offset, &interval, 4); offset += 4;

    // 3. Type (Byte) - 0 = Float32
    buffer[offset++] = 0;

    // 4. Count (UInt32)
    uint32_t cnt = count;
    memcpy(buffer + offset, &cnt, 4); offset += 4;

    // 5. Data (Float32s)
    for (int i = 0; i < count; i++) {
        float val = 20.0f + (random(0, 100) / 10.0f); // Random 20.0 - 30.0
        memcpy(buffer + offset, &val, 4); offset += 4;
    }

    Serial.println("--- 生成测试数据包 ---");
    protocol.sendMetricReport(buffer, offset);
    lastActivityTime = millis();
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
    hw.setResetCallback(onFactoryReset, nullptr);
    hw.getPacketSerial().setPacketHandler(&onPacketReceived);

    // 2. 初始化加密模块
    if (!crypto.begin()) {
        Serial.println("加密模块初始化失败!");
        while (1) hw.blinkLed(1, 500);
    }

    // 3. 联网
    netMgr.begin();

    // 4. 协议栈启动
    protocol.begin();

    lastActivityTime = millis();
    lastTestTime = -TEST_INTERVAL_MS; // Trigger immediately? No, wait a bit
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

    // 测试模式：如果没有真实数据，定期生成假数据
    if (millis() - lastTestTime > TEST_INTERVAL_MS) {
        generateTestPacket();
        lastTestTime = millis();
    }

    // 低功耗逻辑：如果 10 秒内没有任何串口数据发送或活动，进入深度睡眠
    if (millis() - lastActivityTime > IDLE_TIMEOUT_MS) {
        Serial.println("无活动超时，进入深度睡眠...");
        Serial.flush();

        // 关闭 WiFi 射频以省电
        WiFi.disconnect(true);
        WiFi.mode(WIFI_OFF);

        delay(100);
        esp_deep_sleep_start();
    }
}