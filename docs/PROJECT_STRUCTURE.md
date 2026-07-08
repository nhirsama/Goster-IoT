# 项目结构说明

本文档描述当前仓库的实际结构和维护边界。

## 1. 顶层目录

| 路径 | 作用 | 说明 |
|---|---|---|
| `go/` | Core 后端 | Go module：`github.com/nhirsama/Goster-IoT`。提供管理 API、认证、存储、设备注册表、下行队列和 protocol-ingress RPC 服务端。 |
| `protocol-ingress/` | 协议接入服务 | Go module：`github.com/nhirsama/Goster-IoT/protocol-ingress`。接入 Custom TCP 和 MQTT，再通过 Connect RPC 调 Core。 |
| `proto/` | Protobuf 契约 | Go module：`github.com/nhirsama/Goster-IoT/proto`。包含 Core ↔ protocol-ingress 契约和生成代码。 |
| `contracts/` | 管理端 OpenAPI | `contracts/openapi.yaml` 是前后端 HTTP API 契约。 |
| `frontend/` | Web 前端 | Next.js / React 项目，类型可由 OpenAPI 生成。 |
| `docs/` | 精简项目文档 | 保留协议、配置、结构、嵌入式联调边界说明。 |
| `.github/workflows/` | GitHub Actions | 当前包含 Docker 发布工作流。 |
| `.env.example` | 根目录环境变量示例 | Docker Compose 推荐从此复制为 `.env`。 |
| `docker-compose.example.yml` | 根目录容器编排示例 | 启动 Core 与 protocol-ingress，数据默认写入 `./data/core`。 |

嵌入式固件已迁出至独立仓库 `Goster-Iot-Firmware`，本仓库不再包含 STM32/ESP32 固件源码和固件构建配置。

## 2. Core 后端：`go/`

入口：

- `go/my.go`
- `go/cli/cli.go`

当前命令：

```bash
cd go
go run . serve
go run . db init
go run . db migrate
go test ./...
```

主要模块：

| 路径 | 作用 |
|---|---|
| `go/src/config` | Core 环境变量配置。 |
| `go/src/persistence` | 根据配置打开认证存储、运行时存储并确保 schema。 |
| `go/src/storage` | SQLite/Postgres 存储仓储实现。 |
| `go/src/identity` | Authboss 集成、用户/会话相关能力。 |
| `go/src/core` | 装配设备注册、心跳、遥测、下行命令等核心服务。 |
| `go/src/device_manager` | 设备注册表、在线状态、下行队列、遥测写入服务。 |
| `go/src/web` | HTTP 服务、健康检查、v1 API 模块、protocol-ingress RPC handler。 |
| `go/src/web/v1` | `/api/v1` 管理端 API。 |
| `go/src/web/ingress` | `ProtocolIngressCoreService` 的 Core 实现。 |
| `go/db` | Atlas 相关数据库配置。 |
| `go/tests` | Core 集成测试。 |

Core 当前只启动 HTTP 服务；设备协议接入已放在 `protocol-ingress/`。

## 3. protocol-ingress：`protocol-ingress/`

入口：

```bash
cd protocol-ingress
go run ./cmd/protocol-ingress
go test ./...
```

主要模块：

| 路径 | 作用 |
|---|---|
| `cmd/protocol-ingress` | 进程入口。 |
| `internal/config` | 环境变量配置和校验。 |
| `internal/server` | 管理端 `/healthz`、`/readyz`、`/metrics`。 |
| `internal/app` | 装配 server、adapter、normalizer、core client。 |
| `internal/coreclient` | Connect RPC 客户端，支持 Bearer Token。 |
| `internal/normalizer` | adapter 事件/命令与 Protobuf canonical model 的转换。 |
| `internal/adapter/customtcp` | Goster-WY TCP adapter。 |
| `internal/protocol/gosterwy` | Goster-WY 帧编解码和载荷解析。 |
| `internal/adapter/mqtt` | MQTT / Zigbee2MQTT adapter。 |
| `test/e2e` | MQTT 相关端到端测试。 |

## 4. 契约目录

| 路径 | 说明 |
|---|---|
| `contracts/openapi.yaml` | 管理端 `/api/v1` HTTP 契约。前端脚本 `pnpm gen-types` 从这里生成类型。 |
| `proto/goster/ingress/v1/ingress.proto` | Core ↔ protocol-ingress 的服务契约。 |
| `proto/gen/goster/ingress/v1` | 生成的 Go Protobuf / Connect 代码。 |
| `proto/goster.proto` | 旧协议文件，当前不是 Core ↔ protocol-ingress 的服务契约。 |

修改接口时先改契约，再同步实现和测试。

## 5. 前端：`frontend/`

当前脚本以 `frontend/package.json` 为准：

```bash
cd frontend
pnpm dev
pnpm build
pnpm test
pnpm gen-types
```

关键路径：

| 路径 | 作用 |
|---|---|
| `frontend/src/app` | Next.js app 入口。 |
| `frontend/src/lib/api-client.ts` | 管理 API 客户端。 |
| `frontend/src/lib/api-types.ts` | 由 OpenAPI 生成的类型文件。 |
| `frontend/src/hooks` | 前端 hooks。 |
| `frontend/src/components` | 前端组件。 |

## 6. 嵌入式固件边界

嵌入式固件已经从本仓库迁出，迁移范围包括：

| 原路径 | 迁移后归属 | 说明 |
|---|---|---|
| `src/` | 固件仓库 | STM32 Rust 固件源码。 |
| `Cargo.toml`、`Cargo.lock` | 固件仓库 | STM32 Rust crate 与依赖锁定文件。 |
| `memory.x`、`.cargo/` | 固件仓库 | STM32 链接脚本、目标平台和烧录 runner 配置。 |
| `cpp/` | 固件仓库 | ESP32 PlatformIO / Arduino 固件源码、头文件、本地库和测试目录。 |
| `platformio.ini` | 固件仓库 | ESP32 PlatformIO 构建配置。 |

迁移后，本仓库只负责云端、协议接入、前端和接口契约：

- 管理端 HTTP API 契约：`contracts/openapi.yaml`
- Core ↔ protocol-ingress RPC 契约：`proto/goster/ingress/v1/ingress.proto`
- 设备侧 TCP / UART-Bridge 协议说明：`docs/API_SPECIFICATION.md`
- 云端 Goster-WY 协议实现：`protocol-ingress/internal/protocol/gosterwy`

涉及设备通信和协议兼容时，以本仓库的契约文档和 `protocol-ingress` 当前实现为云端侧基准；固件仓库负责 STM32 采集逻辑、ESP32 网关逻辑、板间 UART 封装、硬件适配和固件构建。

后续不要在本仓库恢复 `src/`、`cpp/`、根目录 `Cargo.toml`、`memory.x` 或 `platformio.ini`。如果需要调整固件实现，应在固件仓库中完成；如果需要调整通信协议，应先更新本仓库契约，再同步固件仓库实现。

## 7. 文档维护规则

当前保留的项目文档：

| 文档 | 维护内容 |
|---|---|
| `docs/API_SPECIFICATION.md` | 管理 API 契约位置、Core ↔ protocol-ingress RPC、Goster-WY、MQTT 映射。 |
| `docs/CONFIGURATION.md` | Core 和 protocol-ingress 环境变量。 |
| `docs/PROJECT_STRUCTURE.md` | 仓库结构和模块边界。 |
| `docs/EMBEDDED_DESIGN.md` | 下位机硬件设计、板间通信流程和固件仓库边界。 |

不要再新增临时性的根目录 `docs.md`、长期 TODO 文档或与代码不一致的部署示例。需要记录新行为时，优先更新上述文档和对应契约文件。
