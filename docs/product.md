# GCFeed 产品范围

本文定义 GCFeed 的业务范围、模块边界和功能优先级。README 只保留项目入口，产品状态以本文为准。

## 1. 产品定位

GCFeed 是一个短视频 Feed 系统，目标是用最小可行架构承载完整业务闭环：

```text
注册登录 -> 发布视频 -> 审核治理 -> Feed 分发 -> 浏览互动 -> 消息通知 -> 运营监控
```

首发目标是让用户完成登录、发布、刷 Feed、点赞收藏评论和基础审核；后续补齐消息、后台运营、播放优化、治理和监控。

## 2. 模块地图

| 领域 | 模块 | 职责 |
| --- | --- | --- |
| 用户 | 账户 | 注册、登录、资料、登录态 |
| 用户 | 关系 | 关注、取关、粉丝和关注列表 |
| 内容 | 视频 | 发布、上传、详情、删除 |
| 内容 | 互动 | 点赞、收藏、评论、计数 |
| 分发 | Feed | Timeline、Hot、游标分页、卡片组装 |
| 分发 | 推荐 | 召回、排序、打散、曝光去重 |
| 治理 | 审核 | Agent 初审、人审判定、违规下架 |
| 治理 | 后台运营 | 内容查询、审核分配、配置管理 |
| 体验 | 消息 | 通知、未读数、已读状态 |
| 体验 | 播放优化 | 播放参数、预加载、QoS 上报 |
| 稳定性 | 系统治理 | 限流、降级、死信重试 |
| 稳定性 | 监控告警 | 指标采集、看板、告警事件 |

## 3. 当前实现状态

| 模块 | 状态 | 说明 |
| --- | --- | --- |
| 账户 | 已实现 | 注册、登录、登出、当前用户、公开资料、资料更新 |
| 视频 | 已实现 | 发布、详情、我的作品、作者作品、软删除、上传入口 |
| Feed | 已实现 | Timeline、Hot、复杂 Feed 查询、曝光观看上报 |
| 推荐 | 已实现 | 候选召回、曝光判断、曝光写入 |
| 互动 | 已实现 | 点赞、收藏、评论、删除评论、异步落库 |
| 关系 | 已实现 | 关注、取关、关注列表、粉丝列表 |
| 消息 | 规划中 | 站内通知和未读状态 |
| 审核 | 规划中 | Agent 初审和人工复审 |
| 后台运营 | 规划中 | 视频查询、审核任务、配置管理 |
| 播放优化 | 规划中 | 播放参数、预加载、QoS |
| 系统治理 | 规划中 | 限流、降级、死信重试 |
| 监控告警 | 规划中 | 指标写入、看板、告警 |

## 4. P0 首发闭环

P0 目标是完整跑通用户端主链路和基础稳定性链路。

| 状态 | 模块 | 方法 | 接口路径 | 功能 |
| --- | --- | --- | --- | --- |
| 已实现 | 账户 | POST | `/api/users` | 注册 |
| 已实现 | 账户 | POST | `/api/sessions` | 登录并获取 Token |
| 已实现 | 账户 | DELETE | `/api/sessions/current` | 退出登录 |
| 已实现 | 账户 | GET | `/api/users/me` | 获取当前用户信息 |
| 已实现 | 视频 | POST | `/api/videos` | 发布视频 |
| 已实现 | 视频 | GET | `/api/videos/{videoId}` | 视频详情 |
| 已实现 | 视频 | GET | `/api/users/me/videos` | 我的作品列表 |
| 已实现 | Feed | GET | `/api/feed-items` | 拉取视频流，支持 scene 和游标分页 |
| 已实现 | Feed | POST | `/api/video-view-events` | 上报曝光和观看事件 |
| 已实现 | 推荐 | POST | `/internal/recommendation-candidates` | 召回、排序、打散推荐候选 |
| 已实现 | 推荐 | POST | `/internal/exposures` | 写入曝光记录 |
| 已实现 | 互动 | PUT | `/api/videos/{videoId}/like` | 点赞 |
| 已实现 | 互动 | DELETE | `/api/videos/{videoId}/like` | 取消点赞 |
| 已实现 | 互动 | POST | `/api/videos/{videoId}/comments` | 发表评论 |
| 已实现 | 互动 | GET | `/api/videos/{videoId}/comments` | 评论列表 |
| 规划中 | 审核 | POST | `/internal/review/tasks` | 创建审核任务 |
| 规划中 | 审核 | PUT | `/api/review/tasks/{taskId}/decision` | 人工审核通过或驳回 |
| 规划中 | 审核 | PATCH | `/api/videos/{videoId}` | 违规内容下架 |
| 规划中 | 系统治理 | POST | `/internal/rate-limit-decisions` | 限流放行检查 |
| 规划中 | 监控告警 | POST | `/internal/metric-points` | 核心指标写入 |

## 5. P1 体验和运营能力

| 状态 | 模块 | 方法 | 接口路径 | 功能 |
| --- | --- | --- | --- | --- |
| 已实现 | 账户 | PATCH | `/api/users/me` | 更新头像、昵称、简介 |
| 已实现 | 账户 | GET | `/api/users/{userId}` | 查看公开用户资料 |
| 已实现 | 账户 | GET | `/api/users/{userId}/videos` | 查看用户公开视频列表 |
| 已实现 | 关系 | PUT | `/api/users/me/following/{targetUserId}` | 关注 |
| 已实现 | 关系 | DELETE | `/api/users/me/following/{targetUserId}` | 取关 |
| 已实现 | 关系 | GET | `/api/users/me/following` | 关注列表 |
| 已实现 | 关系 | GET | `/api/users/me/followers` | 粉丝列表 |
| 已实现 | 视频 | DELETE | `/api/videos/{videoId}` | 删除视频，软删除 |
| 已实现 | 上传 | POST | `/api/uploads` | 上传媒体文件 |
| 已实现 | Feed | POST | `/api/feed-queries` | 通过请求体查询复杂 Feed 场景 |
| 已实现 | 推荐 | POST | `/internal/exposure-decisions` | 曝光去重校验 |
| 已实现 | 互动 | PUT | `/api/videos/{videoId}/favorite` | 收藏 |
| 已实现 | 互动 | DELETE | `/api/videos/{videoId}/favorite` | 取消收藏 |
| 已实现 | 互动 | DELETE | `/api/comments/{commentId}` | 删除评论 |
| 规划中 | 消息 | GET | `/api/messages` | 消息列表 |
| 规划中 | 消息 | GET | `/api/message-stats/unread` | 未读计数 |
| 规划中 | 消息 | PATCH | `/api/messages` | 批量已读 |
| 规划中 | 消息 | POST | `/internal/messages` | 消费事件生成消息 |
| 规划中 | 审核 | PUT | `/internal/review/tasks/{taskId}/agent-result` | Agent 初审回传 |
| 规划中 | 后台运营 | GET | `/api/admin/videos` | 运营查视频 |
| 规划中 | 后台运营 | GET | `/api/admin/review/tasks` | 运营查审核任务 |
| 规划中 | 后台运营 | PUT | `/api/admin/review/tasks/{taskId}/assignee` | 分配审核员 |
| 规划中 | 后台运营 | PATCH | `/api/admin/configs/{configKey}` | 更新运营配置 |
| 规划中 | 播放优化 | GET | `/api/playback-config` | 播放参数下发 |
| 规划中 | 播放优化 | GET | `/api/preload-videos` | 预加载建议 |
| 规划中 | 播放优化 | POST | `/internal/playback-qos-reports` | 播放质量上报 |
| 规划中 | 系统治理 | GET | `/internal/governance/degrade-switches` | 查询降级开关 |
| 规划中 | 系统治理 | PATCH | `/api/admin/governance/degrade-switches/{key}` | 调整降级开关 |
| 规划中 | 系统治理 | POST | `/internal/dead-letter-retries` | 死信任务重试 |
| 规划中 | 监控告警 | GET | `/api/admin/metric-dashboard` | 监控看板查询 |
| 规划中 | 监控告警 | POST | `/api/admin/alerts/rules` | 告警规则创建 |
| 规划中 | 监控告警 | GET | `/api/admin/alerts/events` | 告警事件查询 |

## 6. Web 客户端范围

| 状态 | 页面/能力 | 说明 |
| --- | --- | --- |
| 已实现 | 登录/注册页 | 对接账户和会话接口 |
| 已实现 | Feed 页 | 拉取视频流，支持上下切换 |
| 已实现 | 互动面板 | 支持点赞、收藏、评论 |
| 已实现 | 关注操作 | 支持关注作者和取关 |
| 已实现 | 个人主页 | 展示资料、作品、关注、粉丝 |
| 已实现 | 公开主页 | 查看其他用户资料和作品 |
| 已实现 | 发布页 | 发布视频信息 |
| 规划中 | 消息页 | 展示通知和未读状态 |
| 规划中 | 审核后台 | 内容审核和违规处理 |
| 规划中 | 监控看板 | 指标、告警和治理状态 |

## 7. 里程碑

| 阶段 | 目标 | 重点模块 |
| --- | --- | --- |
| M1 | 完成用户端闭环 | 账户、视频、Feed、推荐、互动、关系 |
| M2 | 补齐内容治理 | 审核、后台运营、消息 |
| M3 | 提升体验与稳定性 | 播放优化、系统治理、监控告警 |
| M4 | 支持规模化演进 | 服务拆分、异步链路增强、指标体系完善 |
