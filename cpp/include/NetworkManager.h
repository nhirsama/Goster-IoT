#pragma once

#include <WiFiManager.h>
#include "ConfigManager.h"

class NetworkManager {
public:
    NetworkManager(ConfigManager& configMgr); // Constructor injection
    
    void begin();
    void loop();
    
    bool isConnected() { return WiFi.status() == WL_CONNECTED; }
    
    // 进入强制配网模式
    void startConfigPortal();

    // 清除 WiFi 凭据
    void resetWiFi() { _wm.resetSettings(); }

    // TCP 连接
    WiFiClient* getClient() { return &_client; }
    bool connectServer(const String& ip, uint16_t port);
    
    // 检查互联网连接 (DNS 解析测试)
    bool checkInternet();

private:
    ConfigManager& _configMgr;
    WiFiManager _wm;
    WiFiClient _client;
    
    // Helper to save params
    void setupWiFiManagerParams();
};
