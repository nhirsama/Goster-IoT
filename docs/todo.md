# Goster-IoT TODO

更新时间：2026-03-28

## 架构说明

- `frontend/` 与 `go/` 保持平级，前端只通过 OpenAPI 和 HTTP API 与后端交互，不直接依赖 Go 包或模板。
- 短期目标不是直接微服务化，而是先把当前 Go 单体收敛成“可稳定部署到 Kubernetes 的单体”。
- 中期再拆为独立工作负载：
  - `frontend`
  - `api`
  - `device-gateway`
  - `worker`
- 生产环境默认以 PostgreSQL 为主存储，SQLite 仅保留给本地开发和轻量测试。
- 设备在线状态、下行队列、连接归属、token cache 等运行时状态，不能继续长期依赖单进程内存。

## 开发约束

- 涉及鉴权模型变更时，必须同步更新 Go、OpenAPI、前端和测试，不能只改其中一层。
- 新的前后端接口以 `contracts/openapi.yaml` 为准，避免手写分叉契约。
- 前端继续独立构建和部署，不回退到 Go 内嵌页面模式。
- 与云原生相关的改造优先做状态外置、配置治理、探针和可观测性，再考虑更大规模的服务拆分。
- 安全改造优先级高于部署形态优化，先补认证、凭证存储和握手安全，再做横向扩容。

## P0 认证与安全

### 1. 初始化管理员账号并强制首次改密

- 项目初始化时直接生成管理员账号和初始密码，不再依赖匿名注册。
- 首次登录后必须强制修改密码，未改密前不允许进入敏感管理操作。
- 改密后失效旧会话和 remember token。
- 相关位置：
  - `go/src/storage/identity/repository.go`
  - `go/src/identity`
  - `go/src/web/v1/auth.go`
  - `frontend/src/app/login`

### 2. 移除公开注册入口

- 删除公开注册路由和“首用户自动 admin”逻辑。
- 从 OpenAPI 和前端移除注册入口。
- 管理员创建用户改为后台受控能力。
- 相关位置：
  - `go/src/web/v1/api.go`
  - `go/src/storage/identity/repository.go`
  - `contracts/openapi.yaml`
  - `frontend/src/app/register/page.tsx`
  - `frontend/src/lib/api-types.ts`

### 3. 长期凭证不能明文存储

- 设备 token 改为哈希存储。
- remember token 改为哈希存储，并保持一次性消费。
- OAuth access token / refresh token 不再明文落库，优先不存，必须存时做应用层加密。
- 补数据库迁移、旧 token 轮转和清理方案。
- 相关位置：
  - `go/db/schema/postgres.sql`
  - `go/src/storage/device/repository.go`
  - `go/src/storage/identity/repository.go`
  - `go/src/device_manager/device_registry_service.go`

### 4. 设备握手增加服务端身份认证

- 设备端不能再盲目信任服务端返回的 X25519 公钥。
- 第一阶段采用 pinned public key 或服务端长期签名公钥校验。
- 第二阶段再评估证书链或设备证书体系。
- 相关位置：
  - `go/src/iot_gateway/gateway.go`
  - `cpp/src/GosterProtocol.cpp`
  - `cpp/src/CryptoLayer.cpp`

### 5. ECDH 共享密钥改为 HKDF 派生

- 不能继续直接把原始 ECDH 输出作为 AES 会话密钥。
- 使用 HKDF-SHA256 派生 `aes_key`、`session_id`、nonce/base material。
- 保证 Go 端和设备端派生逻辑一致，并设计协议兼容切换。
- 相关位置：
  - `go/src/protocol/protocol_impl.go`
  - `cpp/src/CryptoLayer.cpp`

## P1 Web 与认证治理

### 6. CORS 改为严格白名单

- `Access-Control-Allow-Credentials=true` 时禁止 `APICORSAllowOrigins=*`。
- 白名单只允许显式配置的 Origin，不能继续回显任意 Origin。
- 增加启动期配置校验。
- 相关位置：
  - `go/src/web/v1/origin_policy.go`
  - `go/src/web/v1/middleware.go`

### 7. 为 Cookie 会话写操作补 CSRF 防护

- 对登录后所有状态变更接口增加 CSRF token 校验。
- 至少覆盖用户管理、设备管理、登出、改密等写操作。
- 相关位置：
  - `go/src/web/v1`
  - `go/src/identity`

### 8. 统一真实客户端 IP 解析

- 验证码校验、登录限流、审计日志统一使用同一套 `RealClientIP()`。
- 只信任来自可信代理的 `X-Forwarded-For` / `X-Real-IP`。
- 相关位置：
  - `go/src/web/v1/auth.go`
  - `go/src/web/v1/login_guard.go`

### 9. 会话密钥改为外部配置

- Session/remember cookie 签名密钥不能每次启动随机生成。
- 改为从环境变量或密钥管理组件加载，并支持密钥轮转。
- 相关位置：
  - `go/src/identity/authboss.go`
  - `go/src/config`

### 10. 补标准安全响应头

- 增加 `X-Frame-Options`、`X-Content-Type-Options`、`Referrer-Policy`。
- HTTPS 场景增加 `Strict-Transport-Security`。
- 逐步补 `Content-Security-Policy`。
- 相关位置：
  - `go/src/web/v1/middleware.go`

## P1 云原生与部署

### 11. 生产默认切换到 PostgreSQL

- SQLite 仅保留给本地开发和轻量测试。
- 测试、预发、生产统一按 PostgreSQL 设计和验证。
- 数据库初始化统一走迁移脚本。
- 相关位置：
  - `go/src/config/app_config.go`
  - `go/src/persistence`
  - `go/src/dbschema`
  - `go/db/migrations/postgres`

### 12. 外置运行时状态

- 以下状态不能继续只保存在 Pod 内存：
  - 设备在线状态
  - 下行命令队列
  - 设备 token cache
  - 连接归属关系
  - 幂等去重状态
- 第一阶段可考虑 Redis；中期再评估 NATS/Kafka。
- 相关位置：
  - `go/src/core/services.go`
  - `go/src/device_manager/device_presence_service.go`
  - `go/src/device_manager/message_queue.go`
  - `go/src/device_manager/device_registry_service.go`
  - `go/src/device_manager/downlink_command_service.go`

### 13. 拆分工作负载

- 将当前单进程拆为独立工作负载：
  - `frontend`
  - `api`
  - `device-gateway`
  - `worker`
- 第一步先从 `cmd` 入口拆分开始。
- 相关位置：
  - `go/cli/cli.go`
  - `go/src/web`
  - `go/src/iot_gateway`
  - `go/src/core`

### 14. 增加健康探针与运维端点

- 增加 `/healthz`、`/readyz`、`/metrics`。
- `readyz` 需要反映关键依赖和接流状态，不只是进程存活。
- 相关位置：
  - `go/src/web`
  - `go/src/iot_gateway`

### 15. 收敛容器与配置资产

- 清理仓库中的 `frontend/.env.local`，改为环境注入或样例文件。
- Go 镜像和前端镜像分离构建。
- Secret、ConfigMap、运行参数按环境区分。
- 当前 `go/Dockerfile` 后续需要按拆分后的工作负载重写。
- 相关位置：
  - `frontend/.env.local`
  - `go/Dockerfile`
  - `frontend`

### 16. 补可观测性基线

- 补 Prometheus 指标。
- 中期补 tracing，至少覆盖 HTTP 请求、设备握手、下行命令投递、数据库耗时。
- 相关位置：
  - `go/src/web`
  - `go/src/iot_gateway`
  - `go/src/storage`

## P2 设备侧与边缘侧治理

### 17. 设备标识与产测流程重做

- 去掉硬编码序列号 `SN123456`。
- 设备出厂时注入真实序列号、设备密钥或唯一标识。
- 不再直接用 `SN + MAC` 作为对外主身份来源，改为独立 opaque device ID。
- 相关位置：
  - `cpp/src/GosterProtocol.cpp`
  - `go/src/device_manager/device_registry_service.go`

### 18. 设备侧敏感信息保护

- WiFi 密码和 device token 不能继续明文存入 NVS。
- 评估 ESP32 NVS encryption、flash encryption、secure boot。
- 相关位置：
  - `cpp/src/ConfigManager.cpp`

### 19. 板内串口链路补认证保护

- STM32/Rust 与 ESP32 之间的串口链路补 MAC 或会话认证。
- 目标是避免物理接触和调试口场景下的伪造帧注入。
- 相关位置：
  - `cpp/src/SerialBridge.cpp`
  - `src/goster_serial.rs`
  - `src/esp_bridge.rs`

## P2 平台演进

### 20. 统一身份层

- 短期继续使用现有会话体系，但先补齐 secret 管理和改密能力。
- 中期评估 OIDC/Keycloak，支持浏览器登录、服务账户、角色映射和多租户身份治理。

### 21. 设备接入协议演进

- 评估是否迁移到 MQTT over TLS。
- 如果继续保留自定义 TCP 协议，明确“接入层”和“平台核心”的边界。

### 22. 时序与异步能力演进

- 当前指标可继续依赖 PostgreSQL。
- 中期按数据量评估 TimescaleDB、ClickHouse、独立 worker / rule engine。
