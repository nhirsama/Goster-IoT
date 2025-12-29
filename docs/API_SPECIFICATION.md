# Goster-IoT 接口开发文档

**版本**: v0.2.1

---

## 1. Goster-WY 嵌入式加密通信协议

**Goster-WY** 是专为资源受限的 MCU (如 ESP32, STM32) 设计的 TCP 长连接私有协议。它在保证 **TLS 级安全性** (AES-GCM +
X25519) 的前提下，极大地优化了握手开销和头部冗余。

### 1.1 协议帧结构 (Frame Structure)

所有多字节整数均采用 **Little-Endian (小端序)**。
基本结构：`Header (32B) + Payload (N Bytes) + Footer (16B)`

#### 1.1.1 头部定义 (Header - 32 Bytes)

| 偏移 (Offset) | 字段名       | 类型         | 说明                                          |
|:------------|:----------|:-----------|:--------------------------------------------|
| **0**       | `Magic`   | `uint16`   | 固定值 **0x5759** (ASCII "WY")                 |
| **2**       | `Version` | `uint8`    | 当前版本 **0x01**                               |
| **3**       | `Flags`   | `uint8`    | 标志位控制                                       |
| **4**       | `Status`  | `uint16`   | 状态位                                         |
| **6**       | `CmdID`   | `uint16`   | 指令 ID                                       |
| **8**       | `KeyID`   | `uint32`   | 密钥指纹,现已废弃                                   |
| **12**      | `Length`  | `uint32`   | **Payload** 的长度 (不含 Header/Footer)          |
| **16**      | `Nonce`   | `byte[12]` | AES-GCM IV。建议格式：`Salt(4B) + Sequence(8B)`   |
| **28**      | `H_CRC16` | `uint16`   | **头部校验和**。算法: CRC-16/MODBUS，范围: Offset 0~27 |
| **30**      | `Padding` | `uint16`   | 填充对齐，固定为 0                                  |

**Flags 定义**:

* `Bit 0 (0x01)`: **ACK** (1=响应包, 0=请求包)
* `Bit 1 (0x02)`: **ENCRYPTED** (1=Payload 已加密, 0=明文)
* `Bit 2 (0x04)`: **COMPRESSED** (1=Payload 已压缩, 0=原始数据)

**Status 定义**

*

#### 1.1.2 尾部定义 (Footer - 16 Bytes)

* **加密模式 (`ENCRYPTED == 1`)**: 存放 **AES-GCM Tag** (16 Bytes)。
* **明文模式 (`ENCRYPTED == 0`)**: 存放 **CRC32 (4 Bytes) + Padding (12 Bytes)**。CRC32 计算范围为 Header + Payload。

#### 1.1.3 握手与鉴权流程

1. **握手初始化 (Handshake)**:
   * 连接建立后，客户端（设备）发送第一帧 `HANDSHAKE_INIT` (0x0001)。
   * **Payload**: 客户端 X25519 公钥 (32 Bytes)。此帧明文发送。

2. **握手响应**:
   * 服务端收到后，回复 `HANDSHAKE_RESP` (0x0002)。
   * **Payload**: 服务端 X25519 公钥 (32 Bytes)。此帧明文发送。
   * 双方基于交换的公钥计算出共享密钥 (Session Key)，后续所有数据帧（包括第二帧）均使用 AES-128-GCM 加密。

3. **身份认证或注册**:
   客户端根据自身是否持有有效 Token 选择发送以下两种指令之一：

   * **情况 A：已有 Token (鉴权)**
     * 指令: `AUTH_VERIFY` (0x0003)
     * **Payload**: 直接发送 Token 字符串。

   * **情况 B：无 Token (注册申请)**
     * 指令: `DEVICE_REGISTER` (0x0005)
     * **Payload**: 发送设备元数据进行认证申请。
     * **格式**: 依次发送 `设备名称`、`序列号`、`MAC地址`、`硬件版本`、`固件版本`、`配置文件版本`。
     * **分隔符**: 每个参数之间使用 ASCII 码 `30` (RS - Record Separator) 分割。

4. **服务端响应**:
   * 服务端回复 `AUTH_ACK` (0x0004)。
   * **Payload**: 认证状态。
     * `0x00`: **鉴权成功**。设备可继续发送后续指令。若为注册后的首次登录，Payload 中将附带新生成的 Token。
     * `0x01`: **鉴权失败/拒绝**。连接将被关闭。
     * `0x02`: **注册申请已提交/待审核**。连接将被关闭。管理员审核通过后，设备下次使用 0x0005 指令（或直接尝试 0x0003）时可获取权限。

### 1.2 安全与加密方案

* **非对称加密**: 使用 **X25519** 进行 ECDH 密钥交换。
* **对称加密**: 使用 **AES-128-GCM**。
* **AAD (附加认证数据)**: 解密时必须将 Header 的前 28 字节作为 AAD 输入，以确保指令 ID 和长度等元数据不被篡改。
* ~~**KeyID 生成**: `KeyID = CRC32(Session_Key) ^ 0x57595759`。~~

### 1.3 指令集 (Command IDs)

| 类型     | 指令名称             | CmdID    | 方向     | 说明                      |
|:-------|:-----------------|:---------|:-------|:------------------------|
| **系统** | `HANDSHAKE_INIT` | `0x0001` | D -> S | 握手发起 (携带 Client PubKey) |
|        | `HANDSHAKE_RESP` | `0x0002` | S -> D | 握手响应 (携带 Server PubKey) |
|        | `AUTH_VERIFY`    | `0x0003` | D -> S | 鉴权验证 (携带加密 Token)       |
|        | `AUTH_ACK`       | `0x0004` | S -> D | 鉴权/注册确认                 |
|        | `DEVICE_REGISTER`| `0x0005` | D -> S | 设备注册申请 (携带元数据)        |
|        | `ERROR_REPORT`   | `0x00FF` | 双向     | 错误报告                    |
| **上行** | `METRICS_REPORT` | `0x0101` | D -> S | 传感器指标数据上报               |
|        | `LOG_REPORT`     | `0x0102` | D -> S | 设备运行日志上报                |
|        | `EVENT_REPORT`   | `0x0103` | D -> S | 关键事件上报                  |
|        | `HEARTBEAT`      | `0x0104` | D -> S | 心跳包                     |
| **下行** | `CONFIG_PUSH`    | `0x0201` | S -> D | 配置参数下发                  |
|        | `OTA_DATA`       | `0x0202` | S -> D | OTA 固件包数据块              |
|        | `ACTION_EXEC`    | `0x0203` | S -> D | 远程控制指令执行                |

---

## 2. Web 管理后台 API (HTTP)

所有接口均受鉴权中间件保护（除登录/注册外），基于 Cookie Session。

### 2.1 认证接口

* **POST `/login`**: 用户登录。
* **POST `/register`**: 用户注册（首位用户自动设为 Admin）。
* **POST `/logout`**: 退出登录。
* **GET `/api/captcha/new`**: 获取验证码 ID。

### 2.2 设备操作接口

| 接口路径                    | 方法     | 权限        | 说明                  |
|:------------------------|:-------|:----------|:--------------------|
| `/api/metrics/{uuid}`   | `GET`  | ReadOnly  | 获取设备的 JSON 格式时序指标数据 |
| `/device/approve`       | `POST` | ReadWrite | 批准待审核设备接入           |
| `/device/revoke`        | `POST` | ReadWrite | 拒绝或吊销设备权限（拉黑）       |
| `/device/unblock`       | `POST` | ReadWrite | 从黑名单移回待审核状态         |
| `/device/token/refresh` | `POST` | ReadWrite | 重新生成设备的访问令牌 (Token) |
| `/device/delete`        | `POST` | ReadWrite | 彻底删除设备及其所有历史数据      |

### 2.3 管理员接口

* **GET `/users`**: 获取系统所有用户列表。
* **POST `/user/permission`**: 修改用户权限等级 (`0:None, 1:ReadOnly, 2:ReadWrite, 3:Admin`)。

---

## 3. 业务数据荷载 (Payload) 规范

### 3.1 传感器数据 (CmdID: 0x0101)

采用紧凑二进制布局：

* `StartTimestamp` (8Bytes, int64): 起始时间戳 (ms)
* `SampleInterval` (4Bytes, uint32): 采样间隔 (ms)
* `DataType` (1Bytes): 数据类型枚举
* `Count` (4Bytes, uint32): 数据点个数
* `DataBlob` (N Bytes): 原始数据流

### 3.2 日志数据 (CmdID: 0x0102)

* `Timestamp` (8Bytes, int64): 日志产生时间
* `Level` (1Bytes): 0:Debug, 1:Info, 2:Warn, 3:Error
* `MsgLen` (2Bytes, uint16): 消息长度
* `Message` (N Bytes): UTF-8 字符串内容
