# Goster-IoT 项目结构文档

- 文档目的：描述仓库目录结构、模块职责、入口文件与构建产物，便于新成员快速定位代码。
- 分析基准：当前仓库目录（2026-03-12）。

---

## 1. 仓库定位

Goster-IoT 是一个多语言 IoT 项目，包含三段主链路：

1. STM32 固件（Rust）
2. ESP32 网关固件（C++/Arduino + PlatformIO）
3. 云端服务（Go，含 TCP 协议服务 + Web 管理后台）

同时配有协议/设计文档与少量自动化发布配置。

---

## 2. 顶层目录结构

```text
Goster-IoT/
├── src/                 # STM32 Rust 固件源码
├── cpp/                 # ESP32 C++ 固件源码（PlatformIO）
├── go/                  # Go 云端服务源码（TCP + Web）
├── docs/                # 项目文档（协议、设计、报告）
├── proto/               # 协议草案 Proto 文件
├── .github/             # CI/CD（Docker 镜像发布）
├── Cargo.toml           # Rust 工程配置
├── platformio.ini       # PlatformIO 工程配置
├── readme.md            # 项目说明
└── memory.x             # STM32 链接脚本相关配置
```
---

## 3. STM32 固件（`src/`）

### 3.1 入口与主流程

- 入口：`src/main.rs`
- 核心职责：
  - 初始化时钟、RTC、ADC、UART
  - 采集传感器数据
  - 批量缓存与序列化
  - 通过串口桥接发送到 ESP32

### 3.2 关键模块

- `sensor_manager.rs`：采样、缓存、批量报告组装
- `protocol_structs.rs`：串口链路使用的数据结构与常量
- `goster_serial.rs`：Goster 帧编码与 COBS 编解码
- `esp_bridge.rs`：STM32 与 ESP32 串口交互状态机
- `dht11_sensor.rs` / `mh_sensor.rs` / `ntc_sensor.rs`：传感器驱动适配
- `device_meta.rs`：设备元信息（UID 等）
- `storage.rs`：存储抽象（目前为辅助模块）

### 3.3 Rust 构建相关

- `Cargo.toml`：依赖与 profile 配置
- `.cargo/config.toml`：目标芯片与 runner 配置
- `memory.x`：嵌入式链接布局

---

## 4. ESP32 网关固件（`cpp/`）

### 4.1 入口与主流程

- 入口：`cpp/src/main.cpp`
- 核心职责：
  - 处理串口收包（来自 STM32）
  - 维护 WiFi/TCP 连接
  - 执行握手/鉴权与数据转发
  - 时间同步与低功耗策略（深度睡眠）

### 4.2 目录职责

- `cpp/src/`：模块实现
- `cpp/include/`：模块头文件
- `cpp/lib/`：第三方/本地库占位目录
- `cpp/test/`：测试占位目录（当前为说明）

### 4.3 关键模块

- `GosterProtocol.*`：网关侧协议状态机与收发逻辑
- `CryptoLayer.*`：X25519 + AES-GCM 相关加解密
- `NetworkManager.*`：WiFi、NTP、TCP 管理
- `SerialBridge.*`：串口帧桥接
- `ConfigManager.*`：网关配置持久化
- `Hardware.*`：按键、LED、硬件抽象
- `ProtocolUtils.*`：CRC 等底层工具

### 4.4 构建配置

- 顶层 `platformio.ini`：
  - 指定板卡 `adafruit_qtpy_esp32c3`
  - 指定 `cpp/src`, `cpp/include` 等目录映射
  - 指定 Arduino 框架与第三方库依赖

---

## 5. Go 云端服务（`go/`）

### 5.1 启动入口

- `go/my.go`：程序入口
- `go/cli/cli.go`：组装并启动服务
  - Web 服务：`:8080`
  - TCP 协议服务：`:8081`

### 5.2 目录分层

- `go/src/inter/`：接口层（Api/DataStore/DeviceManager/Web/Protocol 抽象）
- `go/src/protocol/`：TCP 协议编解码实现
- `go/src/api/`：设备接入服务与业务处理（握手、鉴权、上报）
- `go/src/device_manager/`：设备生命周期与运行态管理
- `go/src/datastore/`：SQLite 存储实现（设备、指标、日志、用户）
- `go/src/web/`：管理后台、鉴权、中间件、模板渲染
- `go/html/`：Web 模板与静态资源
- `go/cli/`：启动编排与模拟测试

### 5.3 部署相关

- `go/Dockerfile`：Go 服务容器化定义
- `go/go.mod`, `go/go.sum`：Go 模块依赖

---

## 6. 文档与规范（`docs/` + `proto/`）

### 6.1 `docs/`

- `API_SPECIFICATION.md`：WY 协议规范文档
- `EMBEDDED_DESIGN.md`：嵌入式硬件与系统设计说明

### 6.2 `proto/`

- `proto/goster.proto`：协议消息的 Proto 草案

注意：`proto/goster.proto` 与当前线上代码协议并非严格一一对应，使用前需先确认与 `docs/API_SPECIFICATION.md` 一致性。

---

## 7. 自动化与工程化配置

### 7.1 GitHub 工作流

- `.github/workflows/docker-publish.yml`
  - 触发条件：`v*` tag push
  - 行为：构建并推送 Go 服务镜像
  - 上下文：`./go`

### 7.2 其他配置

- `.cargo/`：Rust 目标配置

---

## 8. 运行与构建入口速查

### 8.1 Rust（STM32）

- 入口：`src/main.rs`
- 主要配置：`Cargo.toml`, `.cargo/config.toml`, `memory.x`

### 8.2 C++（ESP32）

- 入口：`cpp/src/main.cpp`
- 主要配置：`platformio.ini`

### 8.3 Go（Server）

- 入口：`go/my.go`
- 主要配置：`go/go.mod`, `go/Dockerfile`
