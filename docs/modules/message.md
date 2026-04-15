# 消息模块设计（MVP）

## 1. 模块职责
负责通知消息生成、未读查询和已读状态更新。

## 2. 接口设计（最小实现）

| 方法 | 接口路径 | 作用 | 鉴权 | 幂等键 |
| --- | --- | --- | --- | --- |
| GET | `/api/messages` | 拉取消息列表 | Bearer JWT | - |
| GET | `/api/messages/unread-count` | 获取未读数 | Bearer JWT | - |
| PATCH | `/api/messages/read` | 批量标记已读 | Bearer JWT | 支持 |
| POST | `/internal/messages/consume-event` | 消费互动/关注事件并入消息 | 服务鉴权 | 支持 |

## 3. 数据表设计（最小实现）

### 3.1 `user_message`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 消息ID |
| `user_id` | BIGINT | NOT NULL | 接收用户 |
| `type` | VARCHAR(16) | NOT NULL | LIKE/COMMENT/FOLLOW/SYSTEM |
| `title` | VARCHAR(128) | NOT NULL | 标题 |
| `content` | VARCHAR(1024) | NOT NULL | 内容 |
| `event_id` | VARCHAR(64) | NULLABLE | 事件ID（去重用） |
| `is_read` | TINYINT | NOT NULL, DEFAULT 0 | 是否已读 |
| `created_at` | DATETIME | NOT NULL | 创建时间 |
| `read_at` | DATETIME | NULLABLE | 已读时间 |

索引建议：`idx_user_read_created(user_id, is_read, created_at)`、`uk_user_event(user_id, event_id)`。
