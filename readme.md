# Goster-IoT
这是一个课设项目，是一个从 MCU 到云端的 IoT 的双 MCU 低功耗多语言课程设计。

## 系统分层架构
本项目使用 STM32F103C8T8 MCU 作为主控芯片，作为感知层负责传感器数据采集、数据清洗、临时缓存等功能，主要使用 Rust 语言编写驱动程序。

ESP32C3 MCU 作为网关层，负责与云端上位机进行数据传输和 NTP 时钟同步 ，通过 UART 与 STM32 通信，主要使用 C++ 语言编写驱动程序。

数据层、应用层、业务层均由使用 Golang 语言编写的云原生程序负责。程序暴露 8080 端口提供 http 服务，用于提供设备管理、用户管理、数据可视化的 web 界面。
暴露 8081 端口提供 TCP 链接，通过自定义协议与网关层安全通信。

## 云端快速启动

根目录提供统一的 Docker Compose 和环境变量示例：

```bash
cp .env.example .env
docker compose -f docker-compose.example.yml up -d
```

默认端口：

- `8080`：Core HTTP / 管理 API
- `8081`：Goster-WY TCP 接入
- `1883`：MQTT embedded broker，需在 `.env` 中启用 `PROTOCOL_INGRESS_MQTT_ENABLED=true`
- `8090`：protocol-ingress 管理健康检查，默认只绑定 `127.0.0.1`

## TODO List
[ ] 实现精确到 API 路由级别和设备分组级别的颗粒度多租户鉴权  
[ ] 支持云端控制指令下发  
[x] 统一配置信息模块  
[ ] 引入 Redis Pub/Sub 或轻量消息队列实现跨实例的指令路由  
[ ] 重构设备管理模型，支持米家设备采集信息上传  
[ ] 感知层引入压缩感知算法，压缩采样数据  
[ ] 实现压缩感知数据重构模块，承接硬件端的高比例压缩数据  

## 固件
嵌入式固件已迁移到 [Goster-Iot-Firmware](https://github.com/nhirsama/Goster-Iot-Firmware)。
