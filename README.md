# GCFeed

这是一个面向短视频场景的 Feed 系统工程，目标是用最小可行架构承载完整业务闭环，并为后续扩展提供稳定基础。

新读者可以先看 [如何快速读懂 GCFeed](docs/quickread.md)，按推荐顺序理解项目结构、核心链路和代码阅读入口。

## 项目定位

- 以视频信息流为核心场景，覆盖内容供给、分发、消费与治理的全链路能力。
- 采用模块化设计，强调业务边界清晰、依赖方向稳定、便于协作开发与后续拆分服务。
- 优先保证可演进性，在保持架构简洁的前提下支持逐步扩容。

## 当前实现状态

- ✅ 后端分层结构：Domain、Application、Infrastructure、Interfaces
- ✅ Gin HTTP 服务入口
- ✅ MySQL + GORM 持久化
- ✅ JWT 登录态
- ✅ Feed scene 策略注册表
- ✅ 复杂 Feed 查询入口：`POST /api/feed-queries`
- ✅ 后端 API 测试
- ✅ React + Vite Web 客户端
- ✅ Web 生产构建

## 启动流程

### Docker Compose 启动

前置依赖：

- Docker
- Docker Compose

在项目根目录执行：

```bash
cd apps
docker compose up --build
```

服务启动后访问：

| 服务 | 地址 |
| --- | --- |
| Web | `http://127.0.0.1:5173` |
| API 健康检查 | `http://127.0.0.1:8080/health` |
| MySQL | `127.0.0.1:3307` |
| Redis | `127.0.0.1:6379` |
| RabbitMQ | `http://127.0.0.1:15672` |

Compose 会启动 `mysql`、`redis`、`rabbitmq`、`api`、`web` 五个服务。API 容器使用 `apps/api/configs/config.docker.yaml`，数据库地址为 `mysql:3306`，Redis 地址为 `redis:6379`，RabbitMQ 地址为 `rabbitmq:5672`。

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

停止服务：

```bash
cd apps
docker compose down
```

清理数据库、Redis 和上传文件数据卷：

```bash
cd apps
docker compose down -v
```

### 本地开发启动

本地开发脚本会分别启动 Go API 和 Vite Web：

```bash
./scripts/start.sh
```

默认地址：

| 服务 | 地址 |
| --- | --- |
| Web | `http://127.0.0.1:5173` |
| API | `http://127.0.0.1:8080` |

## 业务模块范围

- 用户域：账户、关系、消息
- 内容域：视频、互动、审核、后台运营
- 分发域：推荐、Feed、播放优化
- 稳定性域：系统治理、监控告警

## 功能清单

### P0 首发闭环

目标：可登录、可发视频、可刷 Feed、可互动、可审核、可稳定运行。

| 状态 | 模块 | 方法 | 接口路径 | 功能 |
| --- | --- | --- | --- | --- |
| ✅ | 账户 | POST | `/api/users` | 注册 |
| ✅ | 账户 | POST | `/api/sessions` | 登录并获取 Token |
| ✅ | 账户 | DELETE | `/api/sessions/current` | 退出登录 |
| ✅ | 账户 | GET | `/api/users/me` | 获取当前用户信息 |
| ✅ | 视频 | POST | `/api/videos` | 发布视频 |
| ✅ | 视频 | GET | `/api/videos/{videoId}` | 视频详情 |
| ✅ | 视频 | GET | `/api/users/me/videos` | 我的作品列表 |
| ✅ | Feed | GET | `/api/feed-items` | 拉取视频流，支持 scene 和游标分页 |
| ✅ | Feed | POST | `/api/video-view-events` | 上报曝光和观看事件 |
| ✅ | 推荐 | POST | `/internal/recommendation-candidates` | 召回、排序、打散推荐候选 |
| ✅ | 推荐 | POST | `/internal/exposures` | 写入曝光记录 |
| ✅ | 互动 | PUT | `/api/videos/{videoId}/like` | 点赞 |
| ✅ | 互动 | DELETE | `/api/videos/{videoId}/like` | 取消点赞 |
| ✅ | 互动 | POST | `/api/videos/{videoId}/comments` | 发表评论 |
| ✅ | 互动 | GET | `/api/videos/{videoId}/comments` | 评论列表 |
| [ ] | 审核 | POST | `/internal/review/tasks` | 创建审核任务 |
| [ ] | 审核 | PUT | `/api/review/tasks/{taskId}/decision` | 人工审核通过或驳回 |
| [ ] | 审核 | PATCH | `/api/videos/{videoId}` | 违规内容下架 |
| [ ] | 系统治理 | POST | `/internal/rate-limit-decisions` | 限流放行检查 |
| [ ] | 监控告警 | POST | `/internal/metric-points` | 核心指标写入 |

### P1 体验和运营能力

| 状态 | 模块 | 方法 | 接口路径 | 功能 |
| --- | --- | --- | --- | --- |
| ✅ | 账户 | PATCH | `/api/users/me` | 更新头像、昵称、简介 |
| ✅ | 账户 | GET | `/api/users/{userId}` | 查看公开用户资料 |
| ✅ | 账户 | GET | `/api/users/{userId}/videos` | 查看用户公开视频列表 |
| ✅ | 关系 | PUT | `/api/users/me/following/{targetUserId}` | 关注 |
| ✅ | 关系 | DELETE | `/api/users/me/following/{targetUserId}` | 取关 |
| ✅ | 关系 | GET | `/api/users/me/following` | 关注列表 |
| ✅ | 关系 | GET | `/api/users/me/followers` | 粉丝列表 |
| ✅ | 视频 | DELETE | `/api/videos/{videoId}` | 删除视频，软删除 |
| ✅ | 上传 | POST | `/api/uploads` | 上传媒体文件 |
| ✅ | Feed | GET | `/api/feed-items` | 刷新 Feed |
| ✅ | Feed | POST | `/api/feed-queries` | 通过请求体查询复杂 Feed 场景 |
| ✅ | 推荐 | POST | `/internal/exposure-decisions` | 曝光去重校验 |
| ✅ | 互动 | PUT | `/api/videos/{videoId}/favorite` | 收藏 |
| ✅ | 互动 | DELETE | `/api/videos/{videoId}/favorite` | 取消收藏 |
| ✅ | 互动 | DELETE | `/api/comments/{commentId}` | 删除评论 |
| [ ] | 消息 | GET | `/api/messages` | 消息列表 |
| [ ] | 消息 | GET | `/api/message-stats/unread` | 未读计数 |
| [ ] | 消息 | PATCH | `/api/messages` | 批量已读 |
| [ ] | 消息 | POST | `/internal/messages` | 消费事件生成消息 |
| [ ] | 审核 | PUT | `/internal/review/tasks/{taskId}/agent-result` | Agent 初审回传 |
| [ ] | 后台运营 | GET | `/api/admin/videos` | 运营查视频 |
| [ ] | 后台运营 | GET | `/api/admin/review/tasks` | 运营查审核任务 |
| [ ] | 后台运营 | PUT | `/api/admin/review/tasks/{taskId}/assignee` | 分配审核员 |
| [ ] | 后台运营 | PATCH | `/api/admin/configs/{configKey}` | 更新运营配置 |
| [ ] | 播放优化 | GET | `/api/playback-config` | 播放参数下发 |
| [ ] | 播放优化 | GET | `/api/preload-videos` | 预加载建议 |
| [ ] | 播放优化 | POST | `/internal/playback-qos-reports` | 播放质量上报 |
| [ ] | 系统治理 | GET | `/internal/governance/degrade-switches` | 查询降级开关 |
| [ ] | 系统治理 | PATCH | `/api/admin/governance/degrade-switches/{key}` | 调整降级开关 |
| [ ] | 系统治理 | POST | `/internal/dead-letter-retries` | 死信任务重试 |
| [ ] | 监控告警 | GET | `/api/admin/metric-dashboard` | 监控看板查询 |
| [ ] | 监控告警 | POST | `/api/admin/alerts/rules` | 告警规则创建 |
| [ ] | 监控告警 | GET | `/api/admin/alerts/events` | 告警事件查询 |

### Web 客户端

| 状态 | 页面/能力 | 说明 |
| --- | --- | --- |
| ✅ | 登录/注册页 | 对接账户和会话接口 |
| ✅ | Feed 页 | 拉取视频流，支持上下切换 |
| ✅ | 互动面板 | 支持点赞、收藏、评论 |
| ✅ | 关注操作 | 支持关注作者和取关 |
| ✅ | 个人主页 | 展示资料、作品、关注、粉丝 |
| ✅ | 公开主页 | 查看其他用户资料和作品 |
| ✅ | 发布页 | 发布视频信息 |
| [ ] | 消息页 | 展示通知和未读状态 |
| [ ] | 审核后台 | 内容审核和违规处理 |
| [ ] | 监控看板 | 指标、告警和治理状态 |

## 文档入口

- 代码阅读导览：[docs/quickread.md](docs/quickread.md)
- 产品与模块设计：`docs/product.md`、`docs/modules/`
- 系统架构图：`docs/architecture.md`
