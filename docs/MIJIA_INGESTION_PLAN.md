# 米家数据接入 Goster-IoT 计划

## 1. 目标与边界

### 目标
- 将米家设备数据接入当前平台（不是把你的设备接入米家）。
- 首期只做数据接入与展示，控制能力放到第二阶段。

### 边界
- 优先个人项目可落地，不走复杂企业接入流程。
- 使用现有 Go 后端与前端，不重构核心架构。

## 2. 研究结论（可行路径）

### 推荐路径（P1）
- 采用 Home Assistant 作为“米家数据网关”：
  - 米家设备 -> Xiaomi Home Integration（HA 官方小米集成）-> HA 事件流 -> Goster-IoT 接收与落库。
- 原因：
  - 小米官方维护的 HA 集成，账号接入成本最低。
  - HA 提供稳定外部 API（WebSocket/REST），方便你平台消费。

### 备选路径（P2）
- 使用 `python-miio` 直接连部分局域网设备（社区方案）。
- 仅作为兜底：兼容性受设备型号影响大，维护成本高。

## 3. 总体架构

### 数据流
1. 在 HA 中登录小米账号并导入米家设备。
2. 新建 `ha-bridge` 适配器服务（建议 Python/Go 均可）。
3. `ha-bridge` 通过 HA WebSocket 订阅 `state_changed`。
4. `ha-bridge` 把标准化后的数据 POST 到 Goster 新增“集成入口”。
5. Goster 写入新增的外部实体时序表（保留现有 `devices/metrics` 兼容旧设备）。

### 推荐最小实现
- 第一版优先接入高价值实体：
  - `switch.*`（智能插座开关状态）
  - `sensor.*`（功率/电压/电流/电量/Wi-Fi 信号）
  - `binary_sensor.*`（在线状态/连接状态）

## 4. 可能的数据类型（米家 -> HA -> Goster）

### 实体域（按优先级）
- P0: `switch`, `sensor`, `binary_sensor`
- P1: `light`, `climate`, `fan`, `cover`, `humidifier`
- P2: `select`, `number`, `text`, `event`, `notify`, `media_player`, `vacuum`, `water_heater`

### 值类型
- 布尔值：`on/off`, `true/false`, `connected/disconnected`
- 数值：`W`, `V`, `A`, `kWh`, `dBm`, `%`, `°C`
- 枚举字符串：运行模式、风速档位、工作状态
- 文本/JSON：设备扩展属性、原始状态属性 `attributes`

### 典型示例（智能插座）
- `switch.plug_xxx` -> 开关状态（布尔）
- `sensor.plug_xxx_power` -> 实时功率（数值）
- `sensor.plug_xxx_voltage` -> 电压（数值）
- `sensor.plug_xxx_current` -> 电流（数值）
- `sensor.plug_xxx_energy` -> 累计耗电（数值）
- `sensor.xxx_rssi` -> Wi-Fi 信号（数值）

## 5. 对现有仓库的改造点

## 5.1 后端（Go）
- 新增内部集成 API（建议仅内网可访问）：
  - `POST /api/v1/integrations/ha/sync-device`
  - `POST /api/v1/integrations/ha/observations/batch`
- 鉴权建议：
  - Header API Key（`X-Integration-Key`）+ 可选 HMAC 时间戳签名。
- 新增外部实体表（建议）：
  - `integration_external_entities`
  - 字段：`source`, `entity_id`, `domain`, `goster_uuid`, `value_type`, `unit`, `attributes_json`, `last_state_*`, `last_seen_ts`
- 新增外部观测值表（建议）：
  - `integration_external_observations`
  - 字段：`source`, `entity_id`, `ts`, `value_num/value_text/value_bool/value_json`, `unit`, `value_sig`, `raw_event_json`
- 设备 UUID 生成策略：
  - `sha256("ha:xiaomi:"+entity_id)`，保证幂等。

## 5.2 适配器（ha-bridge）
- 职责：
  - 维护 HA WebSocket 连接与重连。
  - 筛选目标实体（`sensor.*` / `binary_sensor.*` 等）。
  - 单位转换和类型映射（W/V/A/kWh/dBm 等）。
  - 批量上报与失败重试（本地队列）。
- 配置项：
  - `HA_URL`, `HA_TOKEN`, `GOSTER_API_URL`, `GOSTER_INTEGRATION_KEY`, `ENTITY_ALLOWLIST`.

## 5.3 前端
- 复用现有 Dashboard，不必大改。
- 增加数据来源标识（`source = native | mijia`）即可。

## 6. 分阶段计划

### Phase 0（0.5 天）需求冻结
- 明确首批接入的米家设备清单（型号/实体）。
- 定义是否只读（建议先只读）。
- 确认部署位置：HA 与 Goster 是否同网段。

### Phase 1（1.5 天）最小链路打通
- 在 HA 完成 Xiaomi Home Integration 配置。
- 创建 `ha-bridge` 原型，能收到 `state_changed` 并打印标准结构。
- 在 Goster 新增 `/integrations/ha/observations/batch`，写入外部观测值表。
- 验收：插座开关状态 + 功率/电压/电流至少两项可查询。

### Phase 2（1.5 天）设备映射与幂等
- 新增 `integration_external_entities` 设备映射。
- 实现“首次发现设备 -> 自动建档”。
- 做重复事件去重（同 `entity_id+ts+value_sig`）。
- 验收：重启桥接后不重复建档，数据连续。

### Phase 3（1 天）稳定性与观测
- 增加桥接重连、退避重试、死信队列。
- 增加指标：接收速率、失败率、延迟。
- 增加后端审计日志（集成来源、请求 ID、错误码）。
- 验收：断网/重连后 10 分钟内自动恢复。

### Phase 4（可选，1 天）控制回写
- 增加 `POST /api/v1/integrations/ha/command`。
- 平台下发 -> `ha-bridge` -> HA Service Call。
- 验收：可控制开关类设备。

## 7. 风险与规避

### 风险
- 米家设备并非全部支持 HA 官方集成（部分 BLE/红外/虚拟设备不支持）。
- 无中枢时部分控制经云，实时性可能波动。
- HA 配置文件含 token 等敏感信息，需要加固。

### 规避
- 首批仅接入已验证实体，建立 allowlist。
- 桥接服务加本地缓冲与重试，避免瞬时丢数据。
- HA 与 Goster 部署在内网，token 使用最小权限并定期轮换。

## 8. 验收标准（首期）

- 可导入至少 3 台米家设备到平台。
- 插座状态+功率类数据延迟 < 5 秒（局域网正常情况下）。
- 桥接进程异常退出后自动恢复，恢复后 5 分钟内回到稳定接入。
- 无重复设备建档、无明显重复点爆量。

## 9. 下一步执行建议

1. 先做 Phase 0：确定设备清单与指标映射。
2. 我按此计划先落地 Phase 1（最小可运行版本）。
3. 再根据你真实设备表现迭代 Phase 2/3。
