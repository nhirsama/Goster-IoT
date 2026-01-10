#include "ConfigManager.h"

void ConfigManager::begin() {
    prefs.begin(NS, false); // false = 读写模式
}

AppConfig ConfigManager::loadConfig() {
    AppConfig config;
    config.wifi_ssid = prefs.getString("ssid", "");
    config.wifi_pass = prefs.getString("pass", "");
    config.server_ip = prefs.getString("srv_ip", "192.168.1.100"); // 默认 IP
    config.server_port = prefs.getUShort("srv_port", 8081);

    // 检查 Token 是否存在以避免 Error Log 刷屏
    if (prefs.isKey("token")) {
        config.device_token = prefs.getString("token", "");
    } else {
        config.device_token = "";
    }

    config.is_registered = !config.device_token.isEmpty();
    return config;
}

void ConfigManager::saveConfig(const AppConfig &config) {
    prefs.putString("ssid", config.wifi_ssid);
    prefs.putString("pass", config.wifi_pass);
    prefs.putString("srv_ip", config.server_ip);
    prefs.putUShort("srv_port", config.server_port);
    // Token 通常单独保存，不随普通配置覆盖
}

void ConfigManager::saveToken(const String &token) {
    prefs.putString("token", token);
}

void ConfigManager::clearConfig() {
    Serial.println("正在清除 NVS 配置...");
    prefs.clear(); // 这将清除 "goster" 命名空间下的所有内容
}
