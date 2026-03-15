# 配置管理说明

本文档说明后端统一配置模块（`go/src/config`）支持的配置字段、环境变量映射与默认值。

当前配置通过 `viper` 统一加载，启动入口在 `go/cli/cli.go`：

- 使用 `config.Load()` 一次性读取配置。
- 读取优先级：`环境变量 > 默认值`。
- 字段按模块分组，避免在业务代码中分散 `os.Getenv`。

## 字段清单

| 配置字段 | 环境变量 | 默认值 | 用途 |
|---|---|---|---|
| `db.path` | `DB_PATH` | `./data.db` | SQLite 数据库文件路径。 |
| `web.http_addr` | `WEB_HTTP_ADDR` | `:8080` | HTTP 管理接口监听地址。 |
| `web.api_cors_allow_origins` | `API_CORS_ALLOW_ORIGINS` | `http://localhost:5173,http://127.0.0.1:5173` | API CORS 白名单，逗号分隔。 |
| `web.max_api_body_bytes` | `WEB_API_MAX_BODY_BYTES` | `1048576` | API JSON 请求体最大字节数。 |
| `web.device_list.default_page_size` | `WEB_DEVICE_LIST_DEFAULT_PAGE_SIZE` | `100` | 设备列表默认分页大小。 |
| `web.device_list.max_page_size` | `WEB_DEVICE_LIST_MAX_PAGE_SIZE` | `1000` | 设备列表分页大小上限。 |
| `web.metrics.min_valid_timestamp_ms` | `WEB_METRICS_MIN_VALID_TIMESTAMP_MS` | `1672531200000` | 指标查询的最小有效时间戳（毫秒）。 |
| `web.metrics.default_range_label` | `WEB_METRICS_DEFAULT_RANGE_LABEL` | `1h` | 指标查询默认时间范围标签。 |
| `api.tcp_addr` | `API_TCP_ADDR` | `:8081` | 设备 TCP 协议服务监听地址。 |
| `api.read_timeout` | `API_READ_TIMEOUT` | `60s` | 设备 TCP 连接读超时。 |
| `api.register_ack_grace_delay` | `API_REGISTER_ACK_GRACE_DELAY` | `100ms` | 注册失败/待审后关闭连接前的 ACK 等待时间。 |
| `auth.root_url` | `AUTHBOSS_ROOT_URL` | `http://localhost:8080` | Authboss 根地址，用于认证流程 URL 构建。 |
| `auth.cookie_secure` | `AUTH_COOKIE_SECURE` | 自动推导 | Cookie `Secure` 开关。若未显式设置，会按 `APP_ENV` 或 `auth.root_url` 推导。 |
| `auth.session_cookie_max_age_seconds` | `AUTH_SESSION_COOKIE_MAX_AGE_SECONDS` | `0` | 会话 Cookie 生命周期（秒）。`0` 表示浏览器关闭即失效。 |
| `auth.remember_cookie_max_age_seconds` | `AUTH_REMEMBER_COOKIE_MAX_AGE_SECONDS` | `2592000` | Remember Cookie 生命周期（秒）。 |
| `auth.github_client_id` | `GITHUB_CLIENT_ID` | 空 | GitHub OAuth Client ID。 |
| `auth.github_client_secret` | `GITHUB_CLIENT_SECRET` | 空 | GitHub OAuth Client Secret。 |
| `captcha.provider` | `CAPTCHA_PROVIDER` | 空 | 验证码提供商；`turnstile` 时启用 Turnstile。 |
| `captcha.site_key` | `CF_SITE_KEY` | 空 | Turnstile Site Key。 |
| `captcha.secret_key` | `CF_SECRET_KEY` | 空 | Turnstile Secret Key。 |
| `captcha.verify_timeout` | `CAPTCHA_VERIFY_TIMEOUT` | `5s` | Turnstile 服务端校验超时。 |
| `device_manager.queue_capacity` | `DM_QUEUE_CAPACITY` | `100` | 每个设备下行消息队列容量。 |
| `device_manager.heartbeat_deadline` | `DM_HEARTBEAT_DEADLINE` | `60s` | 心跳超时判定阈值。 |
| `device_manager.external_list.default_size` | `DM_EXTERNAL_LIST_DEFAULT_SIZE` | `100` | 外部实体列表默认分页大小。 |
| `device_manager.external_list.max_size` | `DM_EXTERNAL_LIST_MAX_SIZE` | `1000` | 外部实体列表分页上限。 |
| `device_manager.external_observation.default_limit` | `DM_EXTERNAL_OBS_DEFAULT_LIMIT` | `1000` | 外部观测查询默认上限。 |
| `device_manager.external_observation.max_limit` | `DM_EXTERNAL_OBS_MAX_LIMIT` | `10000` | 外部观测查询最大上限。 |
| `logger.level` | `LOG_LEVEL` | `info` | 日志级别：`debug/info/warn/error`。 |
| `logger.format` | `LOG_FORMAT` | `text` | 日志格式：`text/json`。 |
| `logger.add_source` | `LOG_ADD_SOURCE` | `false` | 是否输出源码位置信息。 |
| `logger.service` | `LOG_SERVICE` | `goster-iot` | 日志中的服务名。 |
| `logger.env` | `LOG_ENV` | `dev` | 日志环境标识。 |
| `app.env` | `APP_ENV` | 空 | 通用运行环境（用于回退设置 `logger.env` 和推导 `auth.cookie_secure`）。 |

## 关键规则

1. `auth.cookie_secure` 推导规则：
   - 若设置了 `AUTH_COOKIE_SECURE` 且可解析为布尔值，优先使用该值。
   - 否则若 `APP_ENV` 为 `prod` 或 `production`，取 `true`。
   - 否则若 `AUTHBOSS_ROOT_URL` 以 `https://` 开头，取 `true`。
   - 其余情况取 `false`。

2. `logger.env` 回退规则：
   - 优先 `LOG_ENV`。
   - 若为空则回退到 `APP_ENV`。
   - 若仍为空，使用默认值 `dev`。

3. Duration 类型字段（如 `API_READ_TIMEOUT`）遵循 Go duration 格式：
   - 例如：`100ms`、`5s`、`2m`。

## 本地开发示例

```bash
export DB_PATH=./data.db
export WEB_HTTP_ADDR=:8080
export API_TCP_ADDR=:8081
export API_CORS_ALLOW_ORIGINS=http://localhost:5173,http://127.0.0.1:5173
export WEB_API_MAX_BODY_BYTES=1048576
export WEB_DEVICE_LIST_DEFAULT_PAGE_SIZE=100
export WEB_DEVICE_LIST_MAX_PAGE_SIZE=1000
export WEB_METRICS_MIN_VALID_TIMESTAMP_MS=1672531200000
export WEB_METRICS_DEFAULT_RANGE_LABEL=1h

export AUTHBOSS_ROOT_URL=http://localhost:8080
export AUTH_COOKIE_SECURE=false
export AUTH_SESSION_COOKIE_MAX_AGE_SECONDS=0
export AUTH_REMEMBER_COOKIE_MAX_AGE_SECONDS=2592000

export API_READ_TIMEOUT=60s
export API_REGISTER_ACK_GRACE_DELAY=100ms

export DM_QUEUE_CAPACITY=100
export DM_HEARTBEAT_DEADLINE=60s
export DM_EXTERNAL_LIST_DEFAULT_SIZE=100
export DM_EXTERNAL_LIST_MAX_SIZE=1000
export DM_EXTERNAL_OBS_DEFAULT_LIMIT=1000
export DM_EXTERNAL_OBS_MAX_LIMIT=10000

export CAPTCHA_VERIFY_TIMEOUT=5s
export LOG_LEVEL=debug
export LOG_FORMAT=text
export LOG_SERVICE=goster-iot
export LOG_ENV=dev
```
