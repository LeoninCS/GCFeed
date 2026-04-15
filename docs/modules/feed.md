# Feed 模块设计（MVP）

## 1. 模块职责
负责刷视频主链路，输出游标分页结果并记录观看行为。

## 2. 接口设计（最小实现）

| 方法 | 接口路径 | 作用 | 鉴权 | 幂等键 |
| --- | --- | --- | --- | --- |
| GET | `/api/feed` | 获取视频流（cursor + limit） | Bearer JWT | - |
| GET | `/api/feed/refresh` | 下拉刷新并返回新游标 | Bearer JWT | - |
| POST | `/api/feed/view-events` | 上报曝光/观看事件 | Bearer JWT | 支持 |

## 3. 数据表设计（最小实现）

### 3.1 `feed_cursor`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 记录ID |
| `user_id` | BIGINT | NOT NULL | 用户ID |
| `scene` | VARCHAR(32) | NOT NULL | 场景 |
| `cursor_token` | VARCHAR(255) | NOT NULL | 当前游标 |
| `expired_at` | DATETIME | NOT NULL | 过期时间 |
| `created_at` | DATETIME | NOT NULL | 创建时间 |

索引建议：`uk_user_scene(user_id, scene)`、`uk_cursor_token(cursor_token)`。

### 3.2 `feed_view_event`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 记录ID |
| `user_id` | BIGINT | NOT NULL | 用户ID |
| `video_id` | BIGINT | NOT NULL | 视频ID |
| `event_type` | VARCHAR(16) | NOT NULL | IMPRESSION/VIEW/COMPLETE |
| `watch_ms` | INT | NOT NULL, DEFAULT 0 | 观看时长 |
| `created_at` | DATETIME | NOT NULL | 事件时间 |

索引建议：`idx_user_time(user_id, created_at)`、`idx_video_time(video_id, created_at)`。
