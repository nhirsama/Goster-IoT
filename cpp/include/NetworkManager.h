#pragma once

#include <WiFiManager.h>
#include "ConfigManager.h"
#include  <chrono>

class NetworkManager {
public:
    // NTP Configuration
    static constexpr long GMT_OFFSET_SEC = 8 * 3600; // UTC+8
    static constexpr int DAYLIGHT_OFFSET_SEC = 0;
    static constexpr const char *NTP_SERVER1 = "ntp.aliyun.com";
    static constexpr const char *NTP_SERVER2 = "pool.ntp.org";
    static constexpr const char *NTP_SERVER3 = "time.windows.com";

    NetworkManager(ConfigManager &configMgr); // Constructor injection

    void begin();

    void loop();

    bool isConnected() { return WiFi.status() == WL_CONNECTED; }

    // Time Synchronization
    void syncTime();

    static bool isTimeValid();

    // 获取当前 Unix 时间戳 (秒)
    static int64_t getTimestamp();

    // 进入强制配网模式
    void startConfigPortal();

    // 清除 WiFi 凭据
    void resetWiFi() { _wm.resetSettings(); }

    // TCP 连接
    WiFiClient *getClient() { return &_client; }

    bool connectServer(const String &ip, uint16_t port);

    // 检查互联网连接 (DNS 解析测试)
    static bool checkInternet();

private:
    ConfigManager &_configMgr;
    WiFiManager _wm;
    WiFiClient _client;

    // Helper to save params
    void setupWiFiManagerParams();
};
