#pragma once

#include <Arduino.h>
#include <Preferences.h>

struct AppConfig {
    String wifi_ssid;
    String wifi_pass;
    String server_ip;
    uint16_t server_port;
    String device_token;
    bool is_registered; // 是否已注册
    
    // 默认值构造
    AppConfig() : server_port(8080), is_registered(false) {}
};

class ConfigManager {
public:
    void begin();
    AppConfig loadConfig();
    void saveConfig(const AppConfig& config);
    void saveToken(const String& token);
    void clearConfig(); // 恢复出厂设置

private:
    Preferences prefs;
    const char* NS = "goster"; // NVS Namespace
};
