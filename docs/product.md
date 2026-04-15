# 视频Feed流系统
## 亮点概述

### 业务亮点

- **游标分页**：保证信息流稳定排序，避免重复和漏数
- **曝光去重**：降低内容重复曝光，优化浏览体验
- **召回打散**：提升内容多样性，避免同质内容集中出现
- **混合分发**：平衡大V与普通作者曝光，优化内容生态
- **排序优化**：支持时间、热度、智能排序，提升分发效果
- **双重审核**：先用Agent审核，再人工审核。

### 系统治理

- **播放优化**：通过预加载、播放器复用、缓存提升首帧与切换体验
- **异步削峰**：解耦非核心链路，缓解高峰压力
- **流量治理**：结合限流、熔断保障系统稳定性
- **云原生部署**：使用 Docker + Kubernetes 完成部署
- **监控告警**：基于 Prometheus + Grafana 实现服务监控

## 业务概述

### *用户业务*

实现用户注册登录、个人信息修改、关注他人、点赞评论收藏和消息通知等基础功能。

### *内容发布业务*

实现视频上传、填写视频信息、发布视频和删除视频等基础功能。

### *用户端的刷视频业务*

实现视频浏览、上下滑切换、点赞评论收藏等基础功能。

### 内容审核**与后台运营**

实现内容审核、违规内容下架和后台管理等基础功能。

### 系统治理与监控看板

实现服务部署、运行监控和基础稳定性保障功能。

## 模块概述

- **账户模块**：负责用户注册登录和个人信息管理
- **关系模块**：负责关注、取关、粉丝和关注列表管理
- **视频模块**：负责视频上传、发布、删除和内容管理
- **推荐模块**：负责曝光去重、召回打散、混合分发和排序优化
- **Feed 模块**：负责视频流浏览、上下滑切换和游标分页
- **互动模块**：负责点赞、评论、收藏等互动能力
- **消息模块**：负责点赞、评论、关注等消息通知
- **审核模块**：负责 Agent 初审、人工复审和违规内容下架
- **后台运营模块**：负责内容审核管理和违规处理
- **播放优化模块**：负责预加载、播放器复用和缓存优化
- **系统治理模块**：负责异步削峰、限流熔断和云原生部署
- **监控告警模块**：负责服务监控、指标展示和告警通知

## 上线排期（接口优先级 P0/P1）

### P0（首发必须）

目标：保证“可登录、可发视频、可刷、可互动、可审核、可稳定运行”的最小闭环。

| 模块 | 方法 | 接口路径 | 说明 |
| --- | --- | --- | --- |
| 账户 | POST | `/api/auth/register` | 注册 |
| 账户 | POST | `/api/auth/login/password` | 登录并获取 Token |
| 账户 | GET | `/api/users/me` | 获取当前用户信息 |
| 视频 | POST | `/api/videos` | 发布视频 |
| 视频 | GET | `/api/videos/{videoId}` | 视频详情 |
| 视频 | GET | `/api/videos/mine` | 我的作品列表 |
| Feed | GET | `/api/feed` | 拉取视频流（游标分页） |
| Feed | POST | `/api/feed/view-events` | 上报曝光/观看事件 |
| 推荐 | POST | `/internal/reco/candidates` | 召回+排序+打散 |
| 推荐 | POST | `/internal/reco/exposure/commit` | 写入曝光记录 |
| 互动 | POST | `/api/interactions/likes/toggle` | 点赞/取消点赞 |
| 互动 | POST | `/api/interactions/comments` | 发表评论 |
| 互动 | GET | `/api/interactions/comments` | 评论列表 |
| 审核 | POST | `/internal/review/tasks` | 创建审核任务 |
| 审核 | POST | `/api/review/tasks/{taskId}/decision` | 人工审核通过/驳回 |
| 审核 | POST | `/api/review/videos/{videoId}/offline` | 违规下架 |
| 系统治理 | POST | `/internal/governance/rate-limit/check` | 限流放行检查 |
| 监控告警 | POST | `/internal/metrics/ingest` | 核心指标写入 |

### P1（上线后补齐）

目标：补齐体验和运营效率能力，不阻塞首发上线。

| 模块 | 方法 | 接口路径 | 说明 |
| --- | --- | --- | --- |
| 账户 | PATCH | `/api/users/me` | 更新头像、昵称、简介 |
| 关系 | POST | `/api/relations/follows` | 关注 |
| 关系 | DELETE | `/api/relations/follows/{targetUserId}` | 取关 |
| 关系 | GET | `/api/relations/following` | 关注列表 |
| 关系 | GET | `/api/relations/followers` | 粉丝列表 |
| 视频 | DELETE | `/api/videos/{videoId}` | 删除视频（软删除） |
| Feed | GET | `/api/feed/refresh` | 下拉刷新 |
| 推荐 | POST | `/internal/reco/exposure/check` | 曝光去重校验 |
| 互动 | POST | `/api/interactions/favorites/toggle` | 收藏/取消收藏 |
| 互动 | DELETE | `/api/interactions/comments/{commentId}` | 删除评论 |
| 消息 | GET | `/api/messages` | 消息列表 |
| 消息 | GET | `/api/messages/unread-count` | 未读计数 |
| 消息 | PATCH | `/api/messages/read` | 批量已读 |
| 消息 | POST | `/internal/messages/consume-event` | 消费事件生成消息 |
| 审核 | POST | `/internal/review/tasks/{taskId}/agent-result` | Agent 初审回传 |
| 后台运营 | GET | `/api/admin/videos` | 运营查视频 |
| 后台运营 | GET | `/api/admin/review/tasks` | 运营查审核任务 |
| 后台运营 | POST | `/api/admin/review/tasks/{taskId}/assign` | 分配审核员 |
| 后台运营 | PATCH | `/api/admin/configs/{configKey}` | 更新运营配置 |
| 播放优化 | GET | `/api/playback/config` | 播放参数下发 |
| 播放优化 | POST | `/api/playback/preload` | 预加载建议 |
| 播放优化 | POST | `/internal/playback/qos/report` | 播放质量上报 |
| 系统治理 | GET | `/internal/governance/degrade-switches` | 查询降级开关 |
| 系统治理 | PATCH | `/api/admin/governance/degrade-switches/{key}` | 调整降级开关 |
| 系统治理 | POST | `/internal/governance/dead-letter/retry` | 死信任务重试 |
| 监控告警 | GET | `/api/admin/metrics/dashboard` | 监控看板查询 |
| 监控告警 | POST | `/api/admin/alerts/rules` | 告警规则创建 |
| 监控告警 | GET | `/api/admin/alerts/events` | 告警事件查询 |

### 建议里程碑

- 第 1 周：账户、视频发布、Feed、推荐候选、互动基础接口联调。
- 第 2 周：审核主流程、限流、核心指标采集，完成首发压测与灰度。
- 第 3 周：补齐关系、消息、后台运营和播放优化接口，进入稳定性迭代。
