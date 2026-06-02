# 如何快速读懂 GCFeed

这份文档给第一次接触 GCFeed 的读者使用。目标是帮助你建立项目理解框架，并知道代码应该按什么顺序读。

## 1. 先建立项目模型

GCFeed 是一个短视频 Feed 系统。你可以把它理解成四条主线：

| 主线 | 负责什么 | 当前代码里的模块 |
| --- | --- | --- |
| 用户 | 注册、登录、资料、关注关系 | `account`、`relation` |
| 内容 | 发布视频、上传文件、视频详情 | `video`、`upload` |
| 分发 | Timeline Feed、Hot Feed、分页、缓存 | `feed`、Redis |
| 互动 | 点赞、收藏、评论、热度、异步落库 | `interaction`、Redis、RabbitMQ |

读代码时优先围绕这四条主线理解项目。每条主线都有独立的 domain、application、infra、http 代码，模块边界比较清晰。

## 2. 先看目录分层

后端代码集中在 `apps/api`：

```text
apps/api/
  cmd/feed/main.go                 # 服务启动入口
  configs/                         # 配置文件
  internal/
    domain/                        # 领域层：实体、错误、仓储接口
    application/                   # 应用层：业务用例编排
    infra/                         # 基础设施：MySQL、Redis、RabbitMQ、JWT
    interfaces/http/               # HTTP 层：路由、Handler、中间件、DTO
  test/                            # API 流程测试
```

四层阅读方式：

| 层 | 先看什么 | 你会理解什么 |
| --- | --- | --- |
| `interfaces/http` | Handler 和 Router | 接口路径、请求参数、响应结构 |
| `application` | Service | 一个业务动作如何编排 |
| `domain` | Entity、Error、Repository | 业务规则和模块抽象 |
| `infra` | Persistence、Cache、MQ | MySQL、Redis、RabbitMQ 如何实现 |

推荐从 HTTP 层进入，再顺着 Service、Domain、Infra 往下追。

## 3. 第一条必读链路：服务怎么启动

先读启动链路，能快速知道依赖怎么装配。

阅读顺序：

1. `apps/api/cmd/feed/main.go`
2. `apps/api/internal/infra/config`
3. `apps/api/internal/infra/database`
4. `apps/api/internal/interfaces/http/router/router.go`

重点看 `router.Register`。它把 GORM 仓储、Redis 缓存、RabbitMQ、JWT、各模块 Service 和 Handler 组装到一起。

你需要记住这个装配顺序：

```text
Config -> DB/Redis/RabbitMQ/JWT -> Repository -> Service -> Handler -> Router
```

后续读任何接口，都可以回到 `router.Register` 找它的入口。

## 4. 按请求链路读代码

### 4.1 注册登录链路

先读账户模块，它最简单，适合熟悉分层方式。

入口：

```text
POST /api/users
POST /api/sessions
```

阅读顺序：

1. `interfaces/http/router/router.go` 找路由。
2. `interfaces/http/account/handler.go` 看参数解析和错误映射。
3. `application/account/service.go` 看注册、登录用例。
4. `domain/account/entity.go` 看用户规则。
5. `infra/persistence/account/gorm.go` 看 MySQL 写入和查询。

读完这条链路，你会理解项目的标准分层写法。

### 4.2 发布视频链路

发布视频能帮助你理解“主表 + 统计表”的设计。

入口：

```text
POST /api/videos
```

阅读顺序：

1. `interfaces/http/video/handler.go`
2. `application/video/service.go`
3. `domain/video/entity.go`
4. `infra/persistence/video/gorm.go`
5. `infra/persistence/video/model.go`

关键点：

- `video` 保存视频主体信息。
- `video_stat` 保存点赞数、收藏数、评论数。
- 发布视频时会同时创建 `video` 和 `video_stat`。
- 写接口通过 `Idempotency-Key` 支持客户端安全重试。

### 4.3 Timeline Feed 链路

Feed 是项目核心。读它时要关注分页、缓存和批量组装。

入口：

```text
GET /api/feed-items?scene=timeline
POST /api/feed-queries
```

阅读顺序：

1. `interfaces/http/feed/handler.go`
2. `application/feed/service.go`
3. `domain/feed/entity.go`
4. `domain/feed/repository.go`
5. `infra/persistence/feed/gorm.go`
6. `infra/cache/feed_cache.go`

关键点：

- Timeline 使用 `published_at DESC, id DESC` 排序。
- Cursor 保存最后一条的排序字段，翻页稳定。
- Redis 页缓存只保存轻量 Feed 页。
- 视频卡片和计数通过批量 MGET 获取。
- 缓存缺失时批量回源 MySQL。

读 Feed 代码时，重点看 `application/feed/service.go` 里的组装流程，它解释了从 `video_id` 到完整 Feed Item 的全过程。

### 4.4 Hot Feed 链路

Hot Feed 展示最近 1 小时热度排名。

入口：

```text
GET /api/feed-items?scene=hot
```

阅读顺序：

1. `application/feed/service.go`
2. `infra/cache/feed_cache.go`
3. `application/interaction/service.go`

关键点：

- Redis ZSET 作为热榜存储。
- 每分钟一个桶：`feed:hot:minute:v1:{yyyyMMddHHmm}`。
- 点赞、收藏、评论会写入当前分钟桶。
- 查询热榜时合并最近 60 个分钟桶。

### 4.5 点赞收藏异步落库链路

这条链路体现 Redis + RabbitMQ 的削峰设计。

入口：

```text
PUT /api/videos/{videoId}/like
DELETE /api/videos/{videoId}/like
PUT /api/videos/{videoId}/favorite
DELETE /api/videos/{videoId}/favorite
```

阅读顺序：

1. `interfaces/http/interaction/handler.go`
2. `application/interaction/service.go`
3. `infra/cache/feed_cache.go`
4. `infra/mq/rabbitmq.go`
5. `application/interaction/worker.go`
6. `infra/persistence/interaction/gorm.go`

完整流程：

```text
HTTP Handler
  -> Interaction Service
  -> Redis 写行为状态和实时计数
  -> RabbitMQ 投递 ActionChangedEvent
  -> ActionWorker 消费事件
  -> MySQL 写 interaction_action 和 video_stat
```

关键 Redis key：

```text
interaction:action:v1:{user_id}:{video_id}:{action}
video:stat:counter:v1:{video_id}
video:stat:v1:{video_id}
```

RabbitMQ 配置：

```text
exchange: gcfeed.interaction
queue: gcfeed.interaction.action_changed
routing key: interaction.action_changed
```

读这条链路时，重点理解两个结果：

- 接口立即返回 Redis 里的最新计数。
- MySQL 通过 Worker 最终写入。

## 5. 用测试理解代码

`apps/api/test` 是很好的代码阅读入口。测试里用内存仓储搭出真实接口流程，可以快速理解每个模块的行为。

推荐阅读顺序：

| 测试文件 | 适合理解什么 |
| --- | --- |
| `account_api_test.go` | 注册、登录、JWT |
| `video_api_test.go` | 发布视频、幂等、软删除 |
| `feed_api_test.go` | Timeline、Hot Feed、缓存、分页 |
| `interaction_api_test.go` | 点赞、收藏、评论、异步落库 |
| `relation_api_test.go` | 关注关系 |

运行测试：

```bash
cd apps/api
go test ./...
```

想理解一个接口时，可以先看对应测试，再回到 Handler 和 Service。

## 6. 常见代码阅读入口

| 目标 | 入口文件 |
| --- | --- |
| 看服务启动 | `apps/api/cmd/feed/main.go` |
| 看所有路由 | `apps/api/internal/interfaces/http/router/router.go` |
| 看配置结构 | `apps/api/internal/infra/config/entity.go` |
| 看数据库连接 | `apps/api/internal/infra/database` |
| 看 JWT | `apps/api/internal/infra/jwt` |
| 看 Redis 缓存 | `apps/api/internal/infra/cache/feed_cache.go` |
| 看 RabbitMQ | `apps/api/internal/infra/mq/rabbitmq.go` |
| 看数据库模型 | `apps/api/internal/infra/persistence/*/model.go` |
| 看 API 测试 | `apps/api/test/*_api_test.go` |

## 7. 读模块时的固定方法

拿任意模块举例，比如 `interaction`：

1. 先看 `interfaces/http/interaction/handler.go`，确认接口形状。
2. 再看 `application/interaction/service.go`，确认业务流程。
3. 接着看 `domain/interaction/entity.go` 和 `errors.go`，确认领域规则。
4. 然后看 `domain/interaction/repository.go`，确认 Service 需要哪些能力。
5. 最后看 `infra/persistence/interaction/gorm.go`，确认数据库如何实现。
6. 配套看 `test/interaction_api_test.go`，确认预期行为。

每个模块都可以按这个路径读。

## 8. 读代码时重点关注这些设计

| 设计 | 代码位置 | 理解重点 |
| --- | --- | --- |
| 幂等 | Service + Repository | `Idempotency-Key` 如何保证重试稳定 |
| 游标分页 | Feed、Comment | cursor 如何保存排序字段 |
| 计数表 | `video_stat` | 高频计数如何集中更新 |
| Redis 缓存 | `infra/cache/feed_cache.go` | Feed 页、卡片、计数、热榜如何缓存 |
| RabbitMQ | `infra/mq/rabbitmq.go` | 事件如何发布、消费、确认 |
| Worker | `application/interaction/worker.go` | 异步事件如何落库 |
| 测试替身 | `apps/api/test` | 内存仓储如何模拟真实行为 |

## 9. 运行项目后怎么验证理解

启动：

```bash
cd apps
docker compose up -d --build
```

健康检查：

```bash
curl http://127.0.0.1:8080/health
```

推荐验证路径：

1. 注册用户。
2. 登录拿 token。
3. 发布视频。
4. 拉取 Timeline Feed。
5. 点赞视频。
6. 查看 Hot Feed。
7. 查看 RabbitMQ 队列是否消费完成。
8. 查 MySQL 的 `interaction_action` 和 `video_stat`。

这条路径能覆盖账号、视频、Feed、Redis、RabbitMQ、MySQL 的核心闭环。

## 10. 继续深入读哪些文档

| 文档 | 适合什么时候读 |
| --- | --- |
| `README.md` | 看启动方式和文档入口 |
| `docs/product.md` | 看产品范围、模块地图和功能状态 |
| `docs/architecture.md` | 看系统图、分层图、核心链路图 |
| `docs/engineering.md` | 新增代码前看工程规范 |
| `docs/modules/feed.md` | 深入 Feed、分页、热榜 |
| `docs/modules/interaction.md` | 深入点赞、收藏、评论、异步落库 |
| `docs/optimization.md` | 理解高并发优化路线 |

## 11. 当前最值得关注的下一步

下一步适合实现曝光/播放事件批量落库。建议按“接口批量接收 -> RabbitMQ 投递 -> Worker 批量写库 -> 聚合更新统计”的路径阅读和扩展现有代码。
