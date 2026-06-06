# GCFeed

GCFeed 是一个面向短视频场景的 Feed 系统工程。项目用 Go API 单体、React Web 客户端、MySQL、Redis 和 RabbitMQ 承载内容供给、分发、消费、互动和治理链路。

## 当前状态

已实现能力：

- 后端分层结构：Domain、Application、Infrastructure、Interfaces。
- Gin HTTP 服务入口和 REST API 路由。
- MySQL + GORM 持久化。
- JWT 登录态。
- Redis Feed 缓存、热榜和互动计数。
- RabbitMQ 异步互动落库、视频发布事件和向量任务。
- React + Vite Web 客户端。
- 消息中心和播放优化接入。
- API 流程测试和 Web 生产构建。
- Prometheus 指标和 Grafana 监控面板。

重点待补能力：

- 审核后台。
- 后台运营。
- 系统治理。

## 快速启动

### Docker Compose

前置依赖：

- Docker
- Docker Compose

启动：

```bash
cd apps
docker compose up --build
```

后台启动：

```bash
cd apps
docker compose up -d --build
```

查看日志：

```bash
cd apps
docker compose logs -f api web
```

停止：

```bash
cd apps
docker compose down
```

清理数据库、Redis 和上传文件数据卷：

```bash
cd apps
docker compose down -v
```

服务地址：

| 服务 | 地址 |
| --- | --- |
| Web | `http://127.0.0.1:5173` |
| API 健康检查 | `http://127.0.0.1:8080/health` |
| API 指标 | `http://127.0.0.1:8080/metrics` |
| MySQL | `127.0.0.1:3307` |
| Redis | `127.0.0.1:6379` |
| RabbitMQ 管理台 | `http://127.0.0.1:15672` |
| Prometheus | `http://127.0.0.1:9090` |
| Grafana 面板 | `http://127.0.0.1:3000/d/gcfeed-overview/gcfeed-overview` |

### 本地开发

```bash
./scripts/start.sh
```

默认地址：

| 服务 | 地址 |
| --- | --- |
| Web | `http://127.0.0.1:5173` |
| API | `http://127.0.0.1:8080` |

## 验证与指标

### 自动化测试

后端测试：

```bash
cd apps/api
go test ./...
```

前端生产构建：

```bash
npm --prefix apps/web run build
```

Compose 配置校验：

```bash
cd apps
docker compose config
```

### Feed 压测

Feed 压测脚本位于 `scripts/feed-load.js`，项目启动后运行：

```bash
ACCOUNT="你的账号" PASSWORD="你的密码" k6 run scripts/feed-load.js
```

常用参数：

```bash
SCENE=timeline VUS=20 DURATION=60s k6 run scripts/feed-load.js
SCENE=hot VUS=20 DURATION=60s k6 run scripts/feed-load.js
ACCOUNT="你的账号" PASSWORD="你的密码" SCENE=recommend VUS=20 DURATION=60s k6 run scripts/feed-load.js
```

脚本会优先使用 `TOKEN`，传入 `ACCOUNT` 和 `PASSWORD` 时会先调用 `/api/sessions` 获取登录态。

重点查看 `http_req_duration` 的 `avg`、`p(95)`、`http_req_failed` 和 `feed_success_rate`。

### 监控面板

Docker Compose 会启动 Prometheus 和 Grafana：

```bash
cd apps
docker compose up -d --build
```

Grafana 默认账号密码：

```text
admin / admin
```

内置面板：`GCFeed / GCFeed Overview`

```text
http://127.0.0.1:3000/d/gcfeed-overview/gcfeed-overview
```

面板覆盖 API QPS、5xx 错误率、API P95、Feed P95、Feed 缓存命中率、上传处理耗时和 Worker 成功率。

Prometheus 抓取目标：

- `gcfeed-api`：`api:8080/metrics`
- `gcfeed-worker`：`worker:9091/metrics`

## 文档地图

| 文档 | 用途 |
| --- | --- |
| [docs/product.md](docs/product.md) | 产品范围、模块地图、P0/P1 功能清单 |
| [docs/quickread.md](docs/quickread.md) | 新读者代码阅读路线 |
| [docs/architecture.md](docs/architecture.md) | 系统架构、分层、核心链路、数据模型 |
| [docs/engineering.md](docs/engineering.md) | 工程规范、目录规则、API 风格、测试约定 |
| [docs/optimization.md](docs/optimization.md) | Feed 性能和稳定性专题 |
| [docs/uiux.md](docs/uiux.md) | Web 客户端 UI/UX 规格 |
| [docs/modules/](docs/modules/README.md) | 各业务模块设计 |
| [openspec/](openspec/) | OpenSpec 项目基线和变更规格 |

## 开发方式

新增功能优先按 OpenSpec 建 change，再按工程规范实现：

```bash
openspec list
openspec validate --all --strict
```

新增后端模块时参考 [docs/engineering.md](docs/engineering.md) 的分层模板和 [docs/modules/README.md](docs/modules/README.md) 的模块规格入口。
