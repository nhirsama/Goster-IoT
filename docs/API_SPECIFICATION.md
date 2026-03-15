# Goster-WY 协议规范

- 文档版本：v0.4.0
- 发布日期：2026-03-12
- 适用范围：设备网关与云端之间的 TCP 二进制协议，以及 STM32 与 ESP32 间的串口桥接封装

---

## 1. 目的与范围

本文档定义 Goster-WY 协议的线级格式、握手鉴权流程、指令语义与错误处理规则。

当历史资料与实现行为冲突时，本文档以当前实现行为为准并将其规范化。

---

## 2. 术语与约定

- `MUST`：强制要求。
- `SHOULD`：建议要求。
- `MAY`：可选行为。
- `D`：设备/网关侧（Device）。
- `S`：服务端（Server）。

编码约定：

- 多字节整数 MUST 使用 Little-Endian。
- `float32` MUST 使用 IEEE-754 单精度。
- 文本字段默认 UTF-8。

---

## 3. 传输模型（TCP）

- 协议运行于 TCP 长连接。
- 服务端监听端口：`8081`。
- 每条 TCP 连接对应一个独立会话（鉴权状态、会话密钥、发送序列号）。
- 接收超时（当前行为）为 60 秒，超时/EOF/解包失败后连接关闭。

---

## 4. 协议帧格式

每帧固定结构：

`Header(32B) + Payload(NB) + Footer(16B)`

### 4.1 Header（32 字节）

| 偏移 | 字段 | 类型 | 约束 | 说明 |
|---|---|---|---|---|
| 0 | Magic | uint16 | MUST = `0x5759` | 协议魔数 |
| 2 | Version | uint8 | MUST = `0x01` | 协议版本 |
| 3 | Flags | uint8 | 位图 | 包标志 |
| 4 | Status | uint16 | 当前固定 0 | 预留 |
| 6 | CmdID | uint16 | 见第 7 章 | 指令号 |
| 8 | KeyID | uint32 | 明文通常为 0 | 密钥标识 |
| 12 | Length | uint32 | `0..1,048,576` | Payload 长度 |
| 16 | Nonce | byte[12] | 同 key 下不可重复 | GCM Nonce |
| 28 | H_CRC16 | uint16 | CRC16/MODBUS 覆盖 `[0..27]` | 头校验 |
| 30 | Padding | uint16 | 当前固定 0 | 对齐填充 |

### 4.2 Flags 位定义

- bit0 (`0x01`)：ACK（响应包）
- bit1 (`0x02`)：ENCRYPTED（Payload 已加密）
- bit2 (`0x04`)：COMPRESSED（保留，当前未启用）

### 4.3 Footer（16 字节）

- 明文包（`ENCRYPTED=0`）：
  - `Footer[0..3]`：CRC32(IEEE) over `Header + Payload`
  - `Footer[4..15]`：全 0
- 密文包（`ENCRYPTED=1`）：
  - `Footer[0..15]`：AES-GCM Tag

---

## 5. 加密与完整性

### 5.1 密钥协商

- MUST 使用 X25519（ECDH）。
- 握手时双方交换 32 字节公钥并计算共享密钥。

### 5.2 对称加密

- MUST 使用 AES-GCM。
- 当前实现行为为 AES-256-GCM（使用 32 字节会话密钥）。
- AAD MUST 为 Header 前 28 字节（`Header[0..27]`）。

### 5.3 Nonce 规则

- Nonce 长度固定 12 字节。
- 同一会话密钥下 Nonce MUST 不重复。
- 推荐格式：`Salt(4B) + Seq(8B)`。

---

## 6. 会话状态机

### 6.1 未鉴权阶段允许指令

连接建立后，在未鉴权状态仅允许：

- `HANDSHAKE_INIT (0x0001)`
- `AUTH_VERIFY (0x0003)`
- `DEVICE_REGISTER (0x0005)`

发送其他指令，服务端 MUST 关闭连接。

### 6.2 标准流程

1. `D -> S` `HANDSHAKE_INIT`（明文，携带设备公钥 32B）
2. `S -> D` `HANDSHAKE_RESP`（明文，ACK=1，携带服务端公钥 32B）
3. `D -> S` `AUTH_VERIFY` 或 `DEVICE_REGISTER`
4. `S -> D` `AUTH_ACK`（加密）
5. 成功后进入业务阶段

### 6.3 AUTH_ACK 状态语义

`AUTH_ACK.Payload[0]`：

- `0x00`：成功
- `0x01`：失败/拒绝
- `0x02`：待审核（Pending）

当设备已审核通过且通过注册路径接入时，`AUTH_ACK.Payload[1..]` 可携带 token。

---

## 7. CmdID 指令注册表（TCP）

### 7.1 系统指令

| 名称 | CmdID | 方向 | 说明 |
|---|---:|---|---|
| HANDSHAKE_INIT | `0x0001` | D -> S | 握手初始化，设备公钥 |
| HANDSHAKE_RESP | `0x0002` | S -> D | 握手响应，服务端公钥 |
| AUTH_VERIFY | `0x0003` | D -> S | Token 鉴权 |
| AUTH_ACK | `0x0004` | S -> D | 鉴权/注册结果 |
| DEVICE_REGISTER | `0x0005` | D -> S | 设备注册申请 |
| ERROR_REPORT | `0x00FF` | 双向 | 错误上报 |

### 7.2 上行指令

| 名称 | CmdID | 方向 | 说明 |
|---|---:|---|---|
| METRICS_REPORT | `0x0101` | D -> S | 指标批量上报 |
| LOG_REPORT | `0x0102` | D -> S | 日志上报 |
| EVENT_REPORT | `0x0103` | D -> S | 事件上报 |
| HEARTBEAT | `0x0104` | D -> S | 心跳 |
| KEY_EXCHANGE_UPLINK | `0x0105` | D -> S | 会话密钥重协商请求 |

### 7.3 下行指令

| 名称 | CmdID | 方向 | 说明 |
|---|---:|---|---|
| CONFIG_PUSH | `0x0201` | S -> D | 配置下发 |
| OTA_DATA | `0x0202` | S -> D | OTA 数据块 |
| ACTION_EXEC | `0x0203` | S -> D | 动作执行 |
| SCREEN_WY | `0x0204` | S -> D | 屏幕/UI 控制（预留） |
| KEY_EXCHANGE_DOWNLINK | `0x0205` | S -> D | 重协商响应 |

---

## 8. 业务 Payload 定义

### 8.1 DEVICE_REGISTER (`0x0005`)

载荷格式：UTF-8 文本，字段以 `0x1E`（RS）分隔。字段顺序 MUST 为：

1. `name`
2. `serial_number`
3. `mac_address`
4. `hw_version`
5. `sw_version`
6. `config_version`

若字段数 < 6，服务端返回 `AUTH_ACK=0x01`。

### 8.2 METRICS_REPORT (`0x0101`)

二进制布局：

- `StartTimestamp`：`uint64`（ms）
- `SampleInterval`：`uint32`（ms）
- `DataType`：`uint8`
- `Count`：`uint32`
- `DataBlob`：`Count * float32`（LE）

当前有效 `DataType`：

- `1`：温度
- `2`：湿度
- `4`：光照

校验约束：

- Payload 长度 MUST >= 17
- `len(DataBlob)` MUST == `Count * 4`

### 8.3 LOG_REPORT (`0x0102`)

二进制布局：

- `Timestamp`：`uint64`（ms）
- `Level`：`uint8`（0=DEBUG, 1=INFO, 2=WARN, 3=ERROR）
- `MsgLen`：`uint16`
- `Message`：`MsgLen` 字节 UTF-8 文本

### 8.4 EVENT_REPORT (`0x0103`)

- Payload 为 UTF-8 文本，具体语义由设备侧定义。

### 8.5 HEARTBEAT (`0x0104`)

- Payload 为空。

### 8.6 ERROR_REPORT (`0x00FF`)

- Payload 为错误描述文本。

---

## 9. ACK 与下行语义

- 服务端对 `METRICS_REPORT` / `LOG_REPORT` / `EVENT_REPORT` / `HEARTBEAT` 返回同 CmdID 的 ACK 包（`ACK=1`）。
- 服务端可在业务阶段主动下发队列消息（`ACK=0`）。
- 当前实现中，下行 ACK 到达后仅记录，不回写消息送达状态。

---

## 10. 错误处理

### 10.1 协议层错误（任一满足即判失败）

- Magic 不匹配
- Header CRC16 校验失败
- 明文 CRC32 校验失败
- GCM 认证失败
- Payload 声明长度超过 1 MiB

当前行为：关闭连接。

### 10.2 业务层错误

- 未鉴权发送非白名单指令：关闭连接。
- 鉴权失败：回 `AUTH_ACK` 后关闭连接。
- 注册 Pending：回 `AUTH_ACK=0x02` 后关闭连接。
- 未知指令：当前行为为记录日志，连接可继续。

---

## 11. 版本与兼容性规则

- `Version` 当前为 `0x01`。
- 新增指令 MUST 在第 7 章注册。
- 现有指令的 Payload 若发生不兼容变更，MUST 采用新 CmdID 或显式版本字段。
- 保留位（`Status`, `COMPRESSED`）后续启用时，必须保证旧实现可安全拒绝。

---

## 12. 安全建议

- 会话密钥 SHOULD 定期轮换（可使用 `KEY_EXCHANGE_*`）。
- 生产环境 SHOULD 增加重放防护窗口与序列号检查。
- 对 `ERROR_REPORT` 文本应限制长度并做日志转义。

---

## 附录 A：UART-Bridge（STM32 <-> ESP32）

> 本附录定义板间通信封装，不与第 7 章 TCP CmdID 语义共享。

### A.1 封装规则

- 载荷帧采用 32+N+16 结构。
- 外层 MUST 使用 COBS 编码，帧尾 `0x00`。

### A.2 唤醒与就绪

- STM32 发送单字节 `0x00` 作为唤醒请求。
- ESP32 可发送 `0x52`（字符 `R`）表示就绪。

### A.3 串口命令

- `CMD_METRICS_REPORT = 0x0101`
- `CMD_TIME_SYNC = 0x0204`

说明：`0x0204` 在 TCP 协议中定义为 `SCREEN_WY`；串口链路与 TCP 链路命令空间逻辑隔离。
