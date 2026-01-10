#include "NetworkManager.h"

// 记录上次 NTP 同步成功的时间 (保留在 RTC 内存中，深度睡眠不丢失)
RTC_DATA_ATTR time_t g_last_ntp_sync_time = 0;

// 全局标志
bool shouldSaveConfig = false;

void saveConfigCallback() {
    Serial.println("准备保存配置");
    shouldSaveConfig = true;
}

NetworkManager::NetworkManager(ConfigManager &configMgr) : _configMgr(configMgr) {
}

void NetworkManager::begin() {
    // 1. 加载配置
    AppConfig cfg = _configMgr.loadConfig();

    // 2. 自定义 CSS 和 HTML
    // 注入 CSS: 更加现代的卡片式设计
    const char *custom_css = R"(
<style>
  body { font-family: "Microsoft YaHei", sans-serif; background-color: #f0f2f5; color: #333; }
  h1 { color: #1a73e8; margin-bottom: 20px; }
  .c { max-width: 400px; margin: 30px auto; padding: 20px; background: #fff; border-radius: 12px; box-shadow: 0 4px 12px rgba(0,0,0,0.1); text-align: center; }
  input { width: 100%; padding: 12px; margin: 8px 0; border: 1px solid #ddd; border-radius: 6px; box-sizing: border-box; font-size: 16px; }
  input:focus { border-color: #1a73e8; outline: none; }
  button { width: 100%; padding: 12px; margin-top: 15px; background-color: #1a73e8; color: white; border: none; border-radius: 6px; font-size: 16px; cursor: pointer; transition: background 0.3s; }
  button:hover { background-color: #1557b0; }
  .q { float: right; font-size: 12px; color: #888; } /* 信号质量 */
  div, form { text-align: left; }
  .btn { display: block; text-decoration: none; padding: 12px; background: #e8f0fe; color: #1a73e8; border-radius: 6px; margin-bottom: 10px; text-align: center; font-weight: 500; }
  .btn:hover { background: #d2e3fc; }
</style>
)";
    _wm.setCustomHeadElement(custom_css);

    // id, label, default, length
    WiFiManagerParameter custom_server_addr("server", "服务器地址 (域名或IP)", cfg.server_ip.c_str(), 64);

    char port_str[6];
    sprintf(port_str, "%d", cfg.server_port);
    WiFiManagerParameter custom_server_port("port", "服务器端口", port_str, 6);

    // 4. 配置 WiFiManager
    _wm.setSaveConfigCallback(saveConfigCallback);
    _wm.addParameter(&custom_server_addr);
    _wm.addParameter(&custom_server_port);

    _wm.setTitle("Goster 设备配网"); // 页面标题

    // 设置菜单内容: 扫描配网, 退出
    std::vector<const char *> menu = {"wifi", "exit"};
    _wm.setMenu(menu);

    // 优化：过滤信号强度低于 10% 的网络，减少列表杂乱
    _wm.setMinimumSignalQuality(10);

    // 移除调试输出以提高一点性能
    // _wm.setDebugOutput(false); 

    _wm.setConnectTimeout(30);
    _wm.setConfigPortalTimeout(180);

    // 5. 启动配网逻辑
    // 检查是否需要进入配网模式：
    // 条件1: 服务器IP未配置 (默认值或空)
    // 条件2: 没有保存的 WiFi 凭据 (通过检查 SDK 内部存储的 SSID)
    // WiFi.begin() 会尝试读取持久化存储，如果 SSID 为空说明未配置。
    // 注意：ESP32 启动时 WiFi 不一定立即初始化，但读取配置通常没问题。
    
    // 预先初始化 WiFi 以读取保存的配置
    WiFi.mode(WIFI_STA); 
    
    bool is_default_server = (cfg.server_ip == "192.168.1.100" || cfg.server_ip.isEmpty());
    // 检查 ConfigManager 中是否有保存的 WiFi SSID
    bool has_wifi_creds = !cfg.wifi_ssid.isEmpty();

    bool connected = false;

    if (is_default_server || !has_wifi_creds) {
        Serial.println("检测到未配置服务器或 WiFi，强制进入 AP 配网模式...");
        // startConfigPortal 是阻塞的，直到用户保存配置或超时
        if (!has_wifi_creds) {
             Serial.println("原因: 无 WiFi 凭据");
        } else {
             Serial.println("原因: 服务器地址未配置");
        }
        
        connected = _wm.startConfigPortal("Goster-Setup");
        
        if (!connected) {
            Serial.println("配网超时或失败，系统将重启...");
            delay(3000);
            ESP.restart();
        }
    } else {
        // 已有配置，尝试直接连接，不启动 AP
        Serial.printf("发现已保存的配置 (SSID: %s)，正在连接...\n", cfg.wifi_ssid.c_str());
        WiFi.begin(cfg.wifi_ssid.c_str(), cfg.wifi_pass.c_str()); // 使用保存的凭据连接

        // 等待连接，但这不应该是死循环阻塞，可以给一点时间尝试
        // 如果失败，就让 loop() 去处理重连，而不是开 AP
        int retry = 0;
        while (WiFi.status() != WL_CONNECTED && retry < 20) { // 等待约 10 秒
            delay(500);
            Serial.print(".");
            retry++;
        }
        Serial.println();

        if (WiFi.status() == WL_CONNECTED) {
            Serial.println("WiFi 连接成功!");
            connected = true;
        } else {
            Serial.println("WiFi 连接超时，将在后台继续尝试...");
            // 不设置 connected = true，但也不重启，让系统在 loop 中运行
        }
    }

    if (connected) {
        // 连接成功后尝试同步时间
        syncTime();
    }

    // 6. 保存自定义参数
    if (shouldSaveConfig) {
        AppConfig newCfg = _configMgr.loadConfig();

        newCfg.wifi_ssid = WiFi.SSID();
        newCfg.wifi_pass = WiFi.psk();
        newCfg.server_ip = String(custom_server_addr.getValue());
        newCfg.server_port = atoi(custom_server_port.getValue());

        Serial.printf("保存配置: Server=%s, Port=%d\n", newCfg.server_ip.c_str(), newCfg.server_port);
        _configMgr.saveConfig(newCfg);
    }
}

void NetworkManager::startConfigPortal() {
    _wm.startConfigPortal("Goster-Setup");
}

void NetworkManager::loop() {
    // WiFiManager 内部处理重连 (如果使用了 autoConnect)，但现在我们手动管理
    // 如果 WiFi 断开，ESP32 SDK 也会尝试重连
    static unsigned long last_check = 0;
    if (millis() - last_check > 5000) {
        last_check = millis();
        if (WiFi.status() != WL_CONNECTED) {
            // Serial.println("WiFi 未连接，正在尝试重连..."); 
            // WiFi.reconnect(); // 通常不需要频繁调用，SDK 会自动处理
        }
    }
}

bool NetworkManager::connectServer(const String &ip, uint16_t port) {
    if (_client.connected()) return true;
    if (!isConnected()) return false;

    _client.setTimeout(5); // 设置超时为 5 秒
    Serial.printf("[Net] 正在连接 TCP 到 %s:%d ...\n", ip.c_str(), port);

    int ret = _client.connect(ip.c_str(), port);
    if (ret == 1) {
        _client.setNoDelay(true); // 禁用 Nagle 算法，减少延迟
        Serial.println("[Net] TCP 连接成功");
        return true;
    } else {
        Serial.printf("[Net] TCP 连接失败。返回值: %d\n", ret);
        return false;
    }
}

bool NetworkManager::checkInternet() {
    // 仅检查 WiFi 是否连接成功并获取到了 IP
    if (WiFi.status() == WL_CONNECTED && WiFi.localIP() != INADDR_NONE) {
        return true;
    }
    return false;
}

void NetworkManager::syncTime() {
    time_t now;
    time(&now);

    bool needSync = false;

    if (!isTimeValid()) {
        Serial.println("系统时间无效，发起 NTP 异步同步...");
        needSync = true;
    } else {
        // 检查距离上次同步是否超过 24 小时 (24 * 3600 秒)
        if (difftime(now, g_last_ntp_sync_time) > (24 * 3600)) {
            Serial.printf("距离上次同步已过 24h (上次: %ld, 当前: %ld)，准备更新...\n", g_last_ntp_sync_time, now);
            needSync = true;
        } else {
            Serial.println("系统时间有效且未过期，跳过 NTP 同步。");
        }
    }

    if (needSync) {
        // 启动 SNTP 时间同步 (异步，非阻塞)
        configTime(GMT_OFFSET_SEC, DAYLIGHT_OFFSET_SEC, NTP_SERVER1, NTP_SERVER2, NTP_SERVER3);

        // 只有在时间确实被更新后，才应该更新 g_last_ntp_sync_time
        // 但由于 configTime 是异步的，这里做一个近似处理：
        // 如果当前时间已经有效，我们假设这次同步会成功刷新
        if (isTimeValid()) {
            time(&g_last_ntp_sync_time);
        }
    }

    // 打印当前时间用于确认
    if (isTimeValid()) {
        struct tm timeinfo;
        if (getLocalTime(&timeinfo)) {
            Serial.println(&timeinfo, "当前系统时间: %Y-%m-%d %H:%M:%S");
        }
    }
}

bool NetworkManager::isTimeValid() {
    time_t now;
    time(&now);
    return now > 1667526096;
}

int64_t NetworkManager::getTimestamp() {
    if (!isTimeValid()) {
        return 0; // 时间无效
    }
    auto now = std::chrono::system_clock::now();
    return std::chrono::duration_cast<std::chrono::seconds>(now.time_since_epoch()).count();
}
