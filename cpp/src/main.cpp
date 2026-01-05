#include <Arduino.h>
#include "ConfigManager.h"
#include "Hardware.h"
#include "NetworkManager.h"
#include "CryptoLayer.h"
#include "GosterProtocol.h"
// 闲置超时时间，10 秒无活动则休眠
constexpr uint16_t IDLE_TIMEOUT_MS = 10000;
// Modules
ConfigManager configMgr;
Hardware hw;
NetworkManager netMgr(configMgr);
CryptoLayer crypto;
GosterProtocol protocol(netMgr, crypto, configMgr);

unsigned long lastActivityTime = 0;

// Callback: STM32 Data Received (COBS Decoded)
void onPacketReceived(const uint8_t *buffer, size_t size) {
    Serial.printf("收到 STM32 数据: %d 字节\n", size);
    hw.blinkLed(1, 50);

    // 转发给服务器
    protocol.sendMetricReport(buffer, size);

    // 更新最后活动时间
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
        while (true) hw.blinkLed(1, 500);
    }

    // 3. 联网
    netMgr.begin();

    // 4. 协议栈启动
    protocol.begin();

    lastActivityTime = millis();
}

void loop() {
    // 转发从 STM32 (Serial1) 接收到的原始数据到电脑 (Serial)
    while (Serial1.available()) {
        Serial.write(Serial1.read());
        lastActivityTime = millis(); // 收到数据视为有活动，推迟休眠
    }

    // 处理各个模块的轮询
    hw.update();
    netMgr.loop();
    protocol.loop();

    // 如果 TCP 连接保持中，视为有活动，防止休眠
    if (netMgr.getClient()->connected()) {
        lastActivityTime = millis();
    }

    // 每隔 1 秒打印一次当前时间 (非阻塞)
    static unsigned long lastPrintTime = 0;
    if (millis() - lastPrintTime >= 1000) {
        lastPrintTime = millis();
        time_t now_time;
        time(&now_time);
        Serial.println(now_time);
    }

    // 低功耗逻辑：如果 10 秒内没有任何串口数据发送或活动，进入深度睡眠
    // if (millis() - lastActivityTime > IDLE_TIMEOUT_MS) {
    //     Serial.println("无活动超时，进入深度睡眠...");
    //     Serial.flush();
    //
    //     // 关闭 WiFi 射频以省电
    //     WiFi.disconnect(true);
    //     WiFi.mode(WIFI_OFF);
    //
    //     delay(1000);
    //     esp_deep_sleep_start();
    // }
}
