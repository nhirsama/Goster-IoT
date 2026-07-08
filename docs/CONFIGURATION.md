# 配置说明

本文档只列当前代码实际读取的环境变量。

- Core 配置入口：`go/src/config/app_config.go`，读取优先级为 `环境变量 > 默认值`。
- protocol-ingress 配置入口：`protocol-ingress/internal/config/config.go`，只从环境变量读取并校验。
- duration 字段使用 Go duration 格式，例如 `300ms`、`5s`、`10m`。

## 0. 根目录 Docker Compose

根目录的 `.env.example` 和 `docker-compose.example.yml` 是当前推荐的容器部署入口；旧的 `go/.env.example`、`go/docker-compose.example.yml` 已不再使用。

```bash
cp .env.example .env
docker compose -f docker-compose.example.yml up -d
```

默认会启动：

| 服务 | 容器内端口 | 宿主机默认绑定 | 说明 |
|---|---:|---|---|
| `core` | `8080` | `0.0.0.0:8080` | 管理 API / Core RPC。 |
| `protocol-ingress` | `8081` | `0.0.0.0:8081` | Goster-WY TCP 接入。 |
| `protocol-ingress` | `1883` | `0.0.0.0:1883` | MQTT embedded broker，需设置 `PROTOCOL_INGRESS_MQTT_ENABLED=true` 才启用。 |
| `protocol-ingress` | `8090` | `127.0.0.1:8090` | ingress 管理健康检查。 |

SQLite 数据默认落在仓库根目录：

```text
./data/core/data.db
```

## 1. Core 配置

Core 入口：`go/my.go` -> `go/cli/cli.go`。默认 HTTP 监听 `:8080`。

### 1.1 数据库

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `DB_DRIVER` | `sqlite` | `sqlite`、`postgres` 或 `postgresql`；归一化后 postgres 使用 `postgres`。 |
| `DB_PATH` | `./data.db` | SQLite 文件路径；Docker Compose 示例中设置为容器内 `/data/data.db`。 |
| `DB_DSN` | 空 | PostgreSQL DSN。 |
| `DB_SCHEMA_MODE` | SQLite: `bootstrap`；Postgres: `managed` | 只接受 `bootstrap` 或 `managed`。 |

### 1.2 Web / API

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `WEB_HTTP_ADDR` | `:8080` | Core HTTP 监听地址。 |
| `API_CORS_ALLOW_ORIGINS` | `http://localhost:3000,http://127.0.0.1:3000` | CORS Origin 白名单，逗号分隔。 |
| `WEB_API_MAX_BODY_BYTES` | `1048576` | JSON 请求体大小上限。 |
| `WEB_DEVICE_LIST_DEFAULT_PAGE_SIZE` | `100` | 设备列表默认分页。 |
| `WEB_DEVICE_LIST_MAX_PAGE_SIZE` | `1000` | 设备列表分页上限。 |
| `WEB_METRICS_MIN_VALID_TIMESTAMP_MS` | `1672531200000` | 指标查询最小有效毫秒时间戳。 |
| `WEB_METRICS_DEFAULT_RANGE_LABEL` | `1h` | 只接受 `1h`、`6h`、`24h`、`7d`、`all`。 |
| `WEB_LOGIN_MAX_FAILURES` | `5` | 登录失败锁定阈值。 |
| `WEB_LOGIN_WINDOW` | `10m` | 登录失败统计窗口。 |
| `WEB_LOGIN_LOCKOUT` | `15m` | 登录锁定时间。 |

Core 管理端健康检查：

| 路径 | 说明 |
|---|---|
| `GET /health` | 进程存活。 |
| `GET /readiness` | 就绪检查，会访问运行时存储。 |

### 1.3 认证

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `AUTHBOSS_ROOT_URL` | `http://localhost:8080` | Authboss 根 URL。 |
| `AUTH_COOKIE_SECURE` | 自动推导 | 显式设置优先；否则 `APP_ENV=prod/production` 或 root URL 为 HTTPS 时为 true。 |
| `GITHUB_CLIENT_ID` | 空 | GitHub OAuth Client ID。 |
| `GITHUB_CLIENT_SECRET` | 空 | GitHub OAuth Client Secret。 |
| `AUTH_SESSION_COOKIE_MAX_AGE_SECONDS` | `0` | Session Cookie 生命周期；0 表示浏览器会话。 |
| `AUTH_REMEMBER_COOKIE_MAX_AGE_SECONDS` | `2592000` | Remember Cookie 生命周期。 |
| `APP_ENV` | 空 | 用于推导 cookie secure，也可作为日志环境回退。 |

### 1.4 protocol-ingress 服务间鉴权

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `PROTOCOL_INGRESS_TOKEN` | 空 | Core 校验 protocol-ingress Connect RPC 的 Bearer Token。为空时不启用校验。 |

请求头格式：

```http
Authorization: Bearer <PROTOCOL_INGRESS_TOKEN>
```

### 1.5 Captcha

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `CAPTCHA_PROVIDER` | 空 | `turnstile` 时启用 Cloudflare Turnstile。 |
| `CF_SITE_KEY` | 空 | Turnstile Site Key。 |
| `CF_SECRET_KEY` | 空 | Turnstile Secret Key。 |
| `CAPTCHA_VERIFY_TIMEOUT` | `5s` | 服务端验证码校验超时。 |

### 1.6 Device Manager

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `DM_QUEUE_CAPACITY` | `100` | 单设备下行队列容量。 |
| `DM_HEARTBEAT_DEADLINE` | `60s` | 心跳超时判定阈值。 |
| `DM_EXTERNAL_LIST_DEFAULT_SIZE` | `100` | 外部实体列表默认分页。 |
| `DM_EXTERNAL_LIST_MAX_SIZE` | `1000` | 外部实体列表分页上限。 |
| `DM_EXTERNAL_OBS_DEFAULT_LIMIT` | `1000` | 外部观测查询默认条数。 |
| `DM_EXTERNAL_OBS_MAX_LIMIT` | `10000` | 外部观测查询上限。 |

### 1.7 日志

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `LOG_LEVEL` | `info` | `debug`、`info`、`warn`、`error`。 |
| `LOG_FORMAT` | `text` | `text` 或 `json`。 |
| `LOG_ADD_SOURCE` | `false` | 是否输出源码位置。 |
| `LOG_SERVICE` | `goster-iot` | 日志服务名。 |
| `LOG_ENV` | `dev` | 日志环境；为空时回退 `APP_ENV`，再回退 `dev`。 |

## 2. protocol-ingress 配置

入口：`protocol-ingress/cmd/protocol-ingress/main.go`。默认管理 HTTP 监听 `127.0.0.1:8090`。

### 2.1 服务、管理端和 Core 连接

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `PROTOCOL_INGRESS_SERVICE_NAME` | `protocol-ingress` | 服务名。 |
| `PROTOCOL_INGRESS_ENV` | `dev` | 运行环境。 |
| `PROTOCOL_INGRESS_INSTANCE_ID` | `ingress-local-01` | 实例 ID；未设置时会尝试使用 `HOSTNAME`。 |
| `PROTOCOL_INGRESS_HTTP_ADDR` | `127.0.0.1:8090` | 管理 HTTP 监听地址；未设置时可由 `PORT` 回退。 |
| `PROTOCOL_INGRESS_SHUTDOWN_TIMEOUT` | `10s` | 优雅关闭超时。 |
| `PROTOCOL_INGRESS_CORE_ENDPOINT` | `http://127.0.0.1:8080` | Core HTTP 地址。 |
| `PROTOCOL_INGRESS_CORE_TIMEOUT` | `5s` | Core RPC 超时。 |
| `PROTOCOL_INGRESS_CORE_TOKEN` | 空 | 发给 Core 的 Bearer Token；未设置时回退 `PROTOCOL_INGRESS_TOKEN`。 |

管理端点：

| 路径 | 说明 |
|---|---|
| `GET /healthz` | 存活检查。 |
| `GET /readyz` | 就绪检查。 |
| `GET /metrics` | 当前只暴露 `protocol_ingress_up`、`protocol_ingress_ready`、`protocol_ingress_uptime_seconds`。 |

### 2.2 Custom TCP adapter

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `PROTOCOL_INGRESS_CUSTOM_TCP_ENABLED` | `false` | 是否启用 Goster-WY TCP adapter。 |
| `PROTOCOL_INGRESS_CUSTOM_TCP_ADDR` | `127.0.0.1:8081` | TCP 监听地址。 |
| `PROTOCOL_INGRESS_CUSTOM_TCP_READ_TIMEOUT` | `60s` | 单次读超时。 |
| `PROTOCOL_INGRESS_CUSTOM_TCP_IDLE_TIMEOUT` | `5m` | 连接空闲超时。 |
| `PROTOCOL_INGRESS_CUSTOM_TCP_RPC_TIMEOUT` | `5s` | 调 Core RPC 超时。 |
| `PROTOCOL_INGRESS_CUSTOM_TCP_REGISTER_ACK_GRACE_DELAY` | `300ms` | 注册失败/待审核时写 ACK 后等待关闭的时间。 |
| `PROTOCOL_INGRESS_CUSTOM_TCP_DOWNLINK_MAX_BATCH` | `1` | 每轮下发最大命令数。 |

### 2.3 MQTT adapter

| 环境变量 | 默认值 | 说明 |
|---|---|---|
| `PROTOCOL_INGRESS_MQTT_ENABLED` | `false` | 是否启用 MQTT adapter。 |
| `PROTOCOL_INGRESS_MQTT_MODE` | `embedded` | Docker Compose 固定为 `embedded`，即 protocol-ingress 内嵌 broker。 |
| `PROTOCOL_INGRESS_MQTT_LISTEN_ADDR` | `:1883` | Docker Compose 固定为容器内 `:1883`。 |
| `PROTOCOL_INGRESS_MQTT_AUTH_MODE` | `client_password_token` | 鉴权方式：`client_id` 必须是设备 UUID，MQTT password 必须是设备 token。 |
| `PROTOCOL_INGRESS_MQTT_QOS` | `1` | MQTT QoS，只能是 0、1、2。 |
| `PROTOCOL_INGRESS_MQTT_MESSAGE_BUFFER` | `128` | 入站消息缓冲。 |
| `PROTOCOL_INGRESS_MQTT_RPC_TIMEOUT` | `5s` | 调 Core RPC 超时。 |
| `PROTOCOL_INGRESS_MQTT_BASE_TOPIC` | `goster/v1` | Goster MQTT topic 前缀。 |
| `PROTOCOL_INGRESS_MQTT_ZIGBEE2MQTT_BASE_TOPIC` | `zigbee2mqtt` | Zigbee2MQTT topic 前缀。 |
| `PROTOCOL_INGRESS_MQTT_SOURCE` | `mqtt` | 写入事件 labels 的 source。 |
| `PROTOCOL_INGRESS_MQTT_DOWNLINK_ENABLED` | `true` | 是否轮询并发布下行命令。 |
| `PROTOCOL_INGRESS_MQTT_DOWNLINK_TOPIC` | `goster/v1/{uuid}/downlink` | 下行发布 topic 模板。 |
| `PROTOCOL_INGRESS_MQTT_DOWNLINK_POLL_INTERVAL` | `2s` | 下行轮询间隔。 |
| `PROTOCOL_INGRESS_MQTT_DOWNLINK_DEVICE_TTL` | `10m` | 已记住设备的下行活跃 TTL。 |
| `PROTOCOL_INGRESS_MQTT_DOWNLINK_MAX_BATCH` | `1` | 每设备每轮最大下行命令数。 |
| `PROTOCOL_INGRESS_MQTT_DOWNLINK_RETAINED` | `false` | 下行消息 retained 标志。 |

`embedded` 模式下，设备连接参数建议如下：

```text
client_id = 设备 UUID
username  = 设备 UUID 或任意非空值（部分 MQTT 3.1.1 客户端要求 password 存在时 username 也存在）
password  = 设备 token
```

连接通过后，内置 broker 会用 Core 返回的 `uuid/tenant_id` 做 ACL：

- 允许 publish：`goster/v1/{uuid}/telemetry|heartbeat|event|ack|state|log`
- 允许 subscribe：`goster/v1/{uuid}/downlink`

## 3. 本地联调最小配置

两个进程使用同一个 token 即可启用服务间鉴权：

```bash
# Core 进程
export PROTOCOL_INGRESS_TOKEN='dev-shared-token'
cd go && go run . serve

# protocol-ingress 进程
export PROTOCOL_INGRESS_CORE_ENDPOINT='http://127.0.0.1:8080'
export PROTOCOL_INGRESS_CORE_TOKEN='dev-shared-token'
export PROTOCOL_INGRESS_CUSTOM_TCP_ENABLED=true
cd protocol-ingress && go run ./cmd/protocol-ingress
```

如需本地无鉴权联调，可让 Core 的 `PROTOCOL_INGRESS_TOKEN` 保持为空。
