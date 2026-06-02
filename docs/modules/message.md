# 消息模块设计

## 1. 模块职责

消息模块负责通知消息生成、消息列表、未读计数和已读状态更新。消息来源包括点赞、评论、关注、系统通知和审核结果。

## 2. 接口设计

| 方法 | 接口路径 | 作用 | 鉴权 | 幂等键 |
| --- | --- | --- | --- | --- |
| GET | `/api/messages` | 拉取消息列表 | Bearer JWT | 无 |
| GET | `/api/message-stats/unread` | 获取未读数 | Bearer JWT | 无 |
| PATCH | `/api/messages` | 批量标记已读 | Bearer JWT | 支持 |
| POST | `/internal/messages` | 消费互动、关注、系统事件并入消息 | 服务鉴权 | 支持 |

## 3. 数据表设计

### 3.1 `user_message`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 消息 ID |
| `user_id` | BIGINT | NOT NULL | 接收用户 |
| `type` | VARCHAR(16) | NOT NULL | `LIKE` / `COMMENT` / `FOLLOW` / `SYSTEM` |
| `title` | VARCHAR(128) | NOT NULL | 标题 |
| `content` | VARCHAR(1024) | NOT NULL | 内容 |
| `event_id` | VARCHAR(64) | NULLABLE | 事件 ID |
| `is_read` | TINYINT | NOT NULL, DEFAULT 0 | 是否已读 |
| `created_at` | DATETIME | NOT NULL | 创建时间 |
| `read_at` | DATETIME | NULLABLE | 已读时间 |

索引建议：`idx_user_read_created(user_id, is_read, created_at)`、`uk_user_event(user_id, event_id)`。

## 4. 业务规则

| 规则 | 说明 |
| --- | --- |
| 消息归属接收用户 | 查询和已读操作只作用于当前登录用户 |
| 事件生成保持幂等 | `user_id + event_id` 保证同一事件只生成一条消息 |
| 未读数按用户聚合 | 只统计 `is_read=0` 的消息 |
| 批量已读安全重复 | 已读消息再次标记保持已读状态 |
| 列表游标分页 | 按 `created_at DESC, id DESC` 返回 |

## 5. 测试建议

| 场景 | 期望 |
| --- | --- |
| 内部事件生成消息 | 写入 `user_message` |
| 重复事件生成消息 | 只保留一条消息 |
| 查询消息列表 | 按创建时间倒序返回 |
| 查询未读数 | 返回当前用户未读数量 |
| 批量标记已读 | 未读数减少 |
| 标记他人消息 | 返回权限错误或忽略 |

## 6. 前端接入点

| 页面 | 接入能力 |
| --- | --- |
| 导航栏 | 展示未读角标 |
| 消息中心 | 消息列表、筛选、已读状态 |
| Feed 页 | 互动成功后可触发消息链路 |
| 个人主页 | 关注通知入口 |
