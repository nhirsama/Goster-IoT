#include "ConfigManager.h"

void ConfigManager::begin() {
    prefs.begin(NS, false); // false = R/W mode
}

AppConfig ConfigManager::loadConfig() {
    AppConfig config;
    config.wifi_ssid = prefs.getString("ssid", "");
    config.wifi_pass = prefs.getString("pass", "");
    config.server_ip = prefs.getString("srv_ip", "192.168.1.100"); // 默认 IP
    config.server_port = prefs.getUShort("srv_port", 8080);
    
    // Check if token exists to avoid Error Log spam
    if (prefs.isKey("token")) {
        config.device_token = prefs.getString("token", "");
    } else {
        config.device_token = "";
    }
    
    config.is_registered = !config.device_token.isEmpty();
    return config;
}

void ConfigManager::saveConfig(const AppConfig& config) {
    prefs.putString("ssid", config.wifi_ssid);
    prefs.putString("pass", config.wifi_pass);
    prefs.putString("srv_ip", config.server_ip);
    prefs.putUShort("srv_port", config.server_port);
    // Token 通常单独保存，不随普通配置覆盖
}

void ConfigManager::saveToken(const String& token) {
    prefs.putString("token", token);
}

void ConfigManager::clearConfig() {
    Serial.println("Clearing NVS config...");
    prefs.clear(); // This wipes everything in the "goster" namespace
}
