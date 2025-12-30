#include "NetworkManager.h"

// Global flag
bool shouldSaveConfig = false;

void saveConfigCallback() {
    Serial.println("准备保存配置");
    shouldSaveConfig = true;
}

NetworkManager::NetworkManager(ConfigManager& configMgr) : _configMgr(configMgr) {}

void NetworkManager::begin() {
    // 1. 加载配置
    AppConfig cfg = _configMgr.loadConfig();

    // 2. 自定义 CSS 和 HTML
    // 注入 CSS: 更加现代的卡片式设计
    const char* custom_css = R"(
<style>
  body { font-family: "Microsoft YaHei", sans-serif; background-color: #f0f2f5; color: #333; }
  h1 { color: #1a73e8; margin-bottom: 20px; }
  .c { max-width: 400px; margin: 30px auto; padding: 20px; background: #fff; border-radius: 12px; box-shadow: 0 4px 12px rgba(0,0,0,0.1); text-align: center; }
  input { width: 100%; padding: 12px; margin: 8px 0; border: 1px solid #ddd; border-radius: 6px; box-sizing: border-box; font-size: 16px; }
  input:focus { border-color: #1a73e8; outline: none; }
  button { width: 100%; padding: 12px; margin-top: 15px; background-color: #1a73e8; color: white; border: none; border-radius: 6px; font-size: 16px; cursor: pointer; transition: background 0.3s; }
  button:hover { background-color: #1557b0; }
  .q { float: right; font-size: 12px; color: #888; } /* Signal quality */
  div, form { text-align: left; }
  .btn { display: block; text-decoration: none; padding: 12px; background: #e8f0fe; color: #1a73e8; border-radius: 6px; margin-bottom: 10px; text-align: center; font-weight: 500; }
  .btn:hover { background: #d2e3fc; }
</style>
)";
    _wm.setCustomHeadElement(custom_css);

    // 3. 定义中文参数
    // id, label, default, length
    WiFiManagerParameter custom_server_addr("server", "服务器地址 (域名或IP)", cfg.server_ip.c_str(), 64);
    
    char port_str[6];
    sprintf(port_str, "%d", cfg.server_port);
    WiFiManagerParameter custom_server_port("port", "服务器端口", port_str, 6);

    // 4. 配置 WiFiManager
    _wm.setSaveConfigCallback(saveConfigCallback);
    _wm.addParameter(&custom_server_addr);
    _wm.addParameter(&custom_server_port);

    // 界面文本中文化
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
    // 检查：如果服务器 IP 是默认值，说明从未配置过（或者被重置了）
    // 此时强制进入 AP 模式，让用户配置服务器信息
    bool is_default_config = (cfg.server_ip == "192.168.1.100" || cfg.server_ip.isEmpty());
    
    bool connected = false;

    if (is_default_config) {
        Serial.println("检测到未配置服务器信息，强制进入 AP 配网模式...");
        // startConfigPortal 是阻塞的，直到用户保存配置或超时
        connected = _wm.startConfigPortal("Goster-Setup");
    } else {
        // 正常尝试自动连接
        connected = _wm.autoConnect("Goster-Setup");
    }

    if (!connected) {
        Serial.println("配网失败或超时，系统将重启...");
        delay(3000);
        ESP.restart(); 
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
    // WiFiManager handles reconnection internally
}

bool NetworkManager::connectServer(const String& ip, uint16_t port) {
    if (_client.connected()) return true;
    if (!isConnected()) return false;
    
    int ret = _client.connect(ip.c_str(), port);
    if (ret == 1) {
        return true;
    } else {
        Serial.printf("Client Connect Failed. Ret: %d\n", ret);
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
