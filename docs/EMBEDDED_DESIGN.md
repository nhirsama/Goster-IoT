# Goster-IoT 下位机硬件与通信设计文档

## 1. 系统架构概述 (低功耗设计)

本系统采用 **双 MCU 协同 + 间歇唤醒** 架构，以最大程度降低功耗：

* **数据采集端 (Always-On/Sleep): STM32F103C8T6**
    * **职责**: 维护本地时钟，定时采集传感器数据并存入缓冲区 (RAM/Flash)。
    * **动作**: 当缓冲区满时，通过 GPIO 唤醒 ESP32，并通过串口批量发送数据。
* **通信网关端 (Deep Sleep): ESP32-C3**
    * **职责**: 平时处于深度睡眠 (Deep Sleep) 状态。
    * **动作**: 被唤醒后，联网、获取 NTP 时间（校准 STM32）、接收批量数据并转发至云端，随后再次休眠。

```mermaid
graph LR
    Sensors -->|ADC/I2C| STM32
    STM32 -->|WakeUp Pin| ESP32
    STM32 <-->|UART (Data/Time)| ESP32
    ESP32 -->|WiFi/TCP| Server
```

## 2. 硬件连接与元件清单

### 2.1 推荐传感器 (家用场景)

(同上，略)

### 2.2 核心接线图

| STM32 Pin | 功能          | ESP32-C3 Pin            | 说明                            |
|:----------|:------------|:------------------------|:------------------------------|
| **PA9**   | USART1_TX   | **RX** (GPIO20)         | 数据发送 (STM32 -> ESP32)         |
| **PA10**  | USART1_RX   | **TX** (GPIO21)         | 时间同步 (ESP32 -> STM32)         |
| **PA1**   | GPIO_Output | **WakeUp** (GPIO9/Boot) | **唤醒引脚** (STM32 拉低/高唤醒 ESP32) |
| **GND**   | 地           | **GND**                 | 必须共地                          |

### 2.3 MCU 最小系统与关键外部元件需求

为了实现低功耗与精确授时，以下外部元件是**必须**的：

#### A. STM32F103 (数据采集 & 时钟维持)

1. **外部低速晶振 (LSE)**: **32.768 kHz**
    * **原因**: 极其重要。STM32 需要使用 RTC (实时时钟) 来维护 ESP32 同步过来的时间戳。内部 LSI 振荡器误差极大，必须使用外部晶振以保证数据时间准确。
2. **外部高速晶振 (HSE)**: **8 MHz**
    * **原因**: 提供稳定的系统主频。
3. **复位电路**: NRST 引脚需接 10kΩ 上拉电阻和 100nF 下地电容。
4. **去耦电容**: 每个 VDD/GND 引脚对附近需放置 100nF 电容。

#### B. ESP32-C3 (无线通信)

1. **电源稳定性**: **3.3V LDO (至少 500mA)**
    * **原因**: WiFi 射频发射瞬间会有高电流脉冲。推荐在 3.3V 输入端并联 **10uF + 100nF** 电容，防止电压跌落导致复位。
2. **Strapping Pins (启动模式)**:
    * GPIO 8: 需确保启动时为高电平 (通常内置上拉，避免外部下拉)。
    * GPIO 9: 启动时需为高电平 (Boot模式为低)。
3. **唤醒引脚**:
    * STM32 的 WakeUp 输出应连接到 ESP32 的 **RTC GPIO** (如 GPIO 0-5)，以便在 Deep Sleep 模式下能被电平变化唤醒。

## 3. 板间通信协议

### 3.1 交互流程

1. **Cold Boot**: ESP32 启动 -> 连网获取 NTP -> 发送 `TimeSync` 包给 STM32 -> ESP32 Deep Sleep。
2. **Sampling**: STM32 定时采集，存入 `BatchBuffer`。
3. **Upload**: Buffer 满 -> STM32 拉动 WakeUp Pin -> ESP32 醒来 -> STM32 发送 `MetricReport` -> ESP32 转发 -> ESP32 Deep
   Sleep。

### 3.2 数据结构定义 (Rust)

```rust
// 对应 docs/API_SPECIFICATION.md 中的 0x0101 指令
#[derive(Serialize, Deserialize)]
pub struct MetricReport {
    pub start_timestamp: u64, // ms
    pub sample_interval: u32, // ms
    pub data_type: u8,        // 1=Temp, 2=Hum, 3=PM2.5
    pub count: u32,
    pub data_blob: heapless::Vec<f32, 20>, // 每次上传20个点
}

// 时间同步包 (ESP32 -> STM32)
#[derive(Serialize, Deserialize)]
pub struct TimeSync {
    pub current_timestamp: u64,
}
```

## 4. ESP32 功能逻辑

### 4.1 配置持久化 (Preferences)

ESP32 负责存储以下关键信息，避免重启后重新配网：

* **WiFi 凭据**: `ssid`, `password`。
* **通信凭据**: `device_token` (鉴权成功后存储), `server_addr`。

### 4.2 协议桥接

1. **接收**: 解析来自 STM32 的 UART 原始数据包。
2. **封装**: 将 `HomeMetrics` 填入 `Goster-WY` 协议的 `METRICS_REPORT (0x0101)` 荷载中。
3. **加密**: 使用 AES-128-GCM 加密后通过 TCP 发送至上位机。

### 4.3 配网与恢复出厂设置 (Factory Reset)

由于物理复位键 (RST) 按下时芯片停止工作，无法检测长按。因此使用 **BOOT 键 (GPIO 9)** 复用为用户功能键。

1. **恢复出厂设置 (Runtime)**:
    * 在设备正常运行时，如果检测到 GPIO 9 被拉低（按下）超过 **5秒**：
        * **动作 1**: 闪烁 LED 提示。
        * **动作 2**: 调用 `Preferences.clear()` 清空所有保存的配置 (WiFi, Token)。
        * **动作 3**: 系统自动重启。

2. **配网模式 (Boot Check)**:
    * ESP32 启动时，先读取 NVS。
    * **无配置**: 如果读取不到 WiFi SSID，自动进入 **AP 模式 (热点模式)**。
        * 热点名: `Goster-Setup-XXXX`
        * 用户连入后，通过 Web 页面配置 WiFi 和服务器地址。
    * **有配置**: 尝试连接 WiFi。若连续连接失败 (如 10 次)，自动回退到 AP 模式。

### 4.4 状态指示灯 (LED)

* **慢闪**: 正在尝试连接 WiFi。
* **快闪**: 处于 AP 配网模式。
* **常亮**: 连接成功，正常工作。

## 5. 开发建议

1. **STM32**: 优先实现定时触发的传感器轮询逻辑，并将结果通过 `postcard` 打印到串口。
2. **ESP32**: 实现 NVS 读写测试，确保能保存并在启动时读取 WiFi 配置。
3. **调试**: ESP32 可通过串口回显功能，将接收到的 STM32 数据转发到 USB 串口供电脑查看。