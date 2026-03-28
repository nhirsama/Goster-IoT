# Goster-IoT TODO

更新时间：2026-03-28

## 已确定方向

### 1. 初始化管理员账号，禁用公网自注册

- 不再依赖匿名注册创建首个管理员。
- 在项目初始化阶段自动生成管理员账号和初始密码，输出方式必须可控，不能写入普通日志。
- 首次登录后必须进入修改密码流程；未完成改密前，不允许进入管理后台的其他敏感操作。
- 如需保留注册能力，只允许管理员在后台主动创建用户，不能继续开放匿名注册。

### 2. 打通修改密码流程

- 增加“当前密码 + 新密码”的修改密码接口和页面。
- 初始化生成的管理员账号必须支持首次登录强制改密。
- 改密成功后应失效旧的 remember token 和现有会话，避免旧凭据继续可用。

## P0

### 3. 长期凭证不能再明文存储

- 设备 token 改为哈希存储，认证时不能再直接 `WHERE token = ?` 查询原文。
- remember token 改为哈希存储，使用后仍保持一次性消费语义。
- OAuth access token / refresh token 不能明文落库；优先考虑不存，必须存时至少做应用层加密。
- 需要补数据库迁移脚本和旧数据清理/轮转方案。
- 受影响位置：
- `go/db/schema/postgres.sql`
- `go/src/storage/device/repository.go`
- `go/src/storage/identity/repository.go`
- `go/src/device_manager/device_registry_service.go`
- 验收标准：
- 数据库泄露后不能直接复用设备接入 token、remember token、OAuth token。
- 认证、remember-me、OAuth 登录流程回归通过。

### 4. 设备握手增加服务端身份认证

- 当前 X25519 握手缺少服务端身份校验，设备不能再盲目信任服务端返回的公钥。
- 第一阶段可采用 pinned public key 或服务端长期签名公钥校验。
- 第二阶段再评估是否升级为证书链或更完整的设备证书体系。
- 受影响位置：
- `go/src/iot_gateway/gateway.go`
- `cpp/src/GosterProtocol.cpp`
- `cpp/src/CryptoLayer.cpp`
- 验收标准：
- 中间人替换服务端临时公钥时，设备端必须拒绝建立会话。
- 握手失败时不能继续发送认证 token 和业务数据。

### 5. ECDH 共享密钥改为 HKDF 派生

- 不能继续直接把原始 ECDH 输出当作 AES 会话密钥使用。
- 使用 HKDF-SHA256 从共享密钥派生 `aes_key`、`session_id`、必要的 nonce/base material。
- 需要保证 Go 端和设备端派生逻辑一致，并保留协议兼容切换方案。
- 受影响位置：
- `go/src/protocol/protocol_impl.go`
- `cpp/src/CryptoLayer.cpp`
- 验收标准：
- 新旧端协商行为可控，启用新协议后加解密互通正常。
- 单元测试覆盖派生结果一致性。

## P1

### 6. CORS 改为严格白名单

- `Access-Control-Allow-Credentials=true` 时必须禁止 `APICORSAllowOrigins=*`。
- 白名单只允许显式配置的 Origin，不能继续回显任意 Origin。
- 需要补启动期配置校验，错误配置应直接报错而不是带病运行。
- 受影响位置：
- `go/src/web/v1/origin_policy.go`
- `go/src/web/v1/middleware.go`
- 验收标准：
- 非白名单 Origin 返回拒绝。
- 配置 `*` 且允许凭据时，服务启动失败或配置校验失败。

### 7. 为 Cookie 会话写操作补 CSRF 防护

- 当前仅依赖 `SameSite=Lax` 不够稳妥。
- 对登录后状态变更接口增加 CSRF token 校验，至少覆盖用户管理、设备管理、登出、改密等写操作。
- 如果未来前后端完全分离并改用 bearer token，可再按客户端类型细化策略。
- 受影响位置：
- `go/src/web/v1`
- `go/src/identity`
- 验收标准：
- 缺少或伪造 CSRF token 的跨站写请求被拒绝。
- 正常浏览器会话流程不受影响。

### 8. 统一真实客户端 IP 解析

- 验证码校验、登录限流、审计日志必须使用同一套 `RealClientIP()` 逻辑。
- 明确信任代理名单；只有来自可信代理的 `X-Forwarded-For` / `X-Real-IP` 才能被采信。
- 避免出现“验证码看转发头、限流看 RemoteAddr”的分裂行为。
- 受影响位置：
- `go/src/web/v1/auth.go`
- `go/src/web/v1/login_guard.go`
- 验收标准：
- 反向代理部署和直连部署下，限流与验证码都能拿到一致且可信的客户端 IP。

### 9. 会话密钥改为外部配置

- Session/remember cookie 签名密钥不能每次启动随机生成。
- 统一改为从环境变量或密钥管理组件加载，支持多副本和重启后保持登录态。
- 同时需要准备密钥轮转方案。
- 受影响位置：
- `go/src/identity/authboss.go`
- `go/src/config`
- 验收标准：
- 服务重启后，未过期会话保持有效。
- 多实例部署下，会话可在不同实例间正常校验。

### 10. 补标准安全响应头

- 增加 `X-Frame-Options`、`X-Content-Type-Options`、`Referrer-Policy`。
- 在 HTTPS 场景增加 `Strict-Transport-Security`。
- 评估并逐步补 `Content-Security-Policy`，避免一上来设成过宽白名单。
- 受影响位置：
- `go/src/web/v1/middleware.go`
- 验收标准：
- 主要 API 和后台页面响应头满足基础 Web 安全基线。

## P2

### 11. 设备标识与产测流程重做

- 去掉硬编码序列号 `SN123456`。
- 设备出厂时注入真实序列号、设备密钥或唯一标识，不要在运行时拼默认值。
- 不建议继续把 `SN + MAC` 直接哈希为对外主身份，最好引入独立的 opaque device ID。
- 受影响位置：
- `cpp/src/GosterProtocol.cpp`
- `go/src/device_manager/device_registry_service.go`
- 验收标准：
- 同批设备不会因默认值导致身份冲突或可预测。
- 新设备注册流程和产测流程文档齐备。

### 12. 设备侧敏感信息保护

- WiFi 密码和 device token 不能继续以明文方式存入 NVS。
- 评估启用 ESP32 NVS encryption、flash encryption、secure boot。
- 如果硬件条件受限，至少要区分“调试固件”和“生产固件”的存储策略。
- 受影响位置：
- `cpp/src/ConfigManager.cpp`
- 验收标准：
- 生产固件在物理获取 flash dump 的情况下，攻击成本显著高于直接明文读取。

### 13. 板内串口链路补认证保护

- 当前 STM32/Rust 与 ESP32 之间只有 CRC/COBS，缺少密码学完整性校验。
- 如果威胁模型包含本地物理接触和调试口攻击，需要给串口帧增加 MAC 或会话认证。
- 这项优先级低于公网暴露面的修复，但不能永远不做。
- 受影响位置：
- `cpp/src/SerialBridge.cpp`
- `src/goster_serial.rs`
- `src/esp_bridge.rs`
- 验收标准：
- 未持有链路认证材料时，不能伪造有效串口业务帧。

## 备注

- 以上事项优先按 `P0 -> P1 -> P2` 顺序推进。
- 建议每一项独立提交，并附回归测试或最小复现用例。
- 在云原生改造之前，先完成这些安全基线，否则只是把问题更快地部署到更多节点。
