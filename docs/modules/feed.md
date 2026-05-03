# Feed 模块设计（MVP）

## 1. 模块职责
负责刷视频主链路，输出游标分页结果并记录观看行为。

## 2. 接口设计（最小实现）

| 方法 | 接口路径 | 作用 | 鉴权 | 幂等键 |
| --- | --- | --- | --- | --- |
| GET | `/api/feed/time` | 获取按发布时间排序的视频流 | 可匿名 | - |
| GET | `/api/feed/refresh` | 下拉刷新并返回新游标 | 可匿名 | - |
| POST | `/api/feed/view-events` | 上报曝光/观看事件 | 可匿名 | 支持 |

### 2.1 Time Feed API

用于按视频发布时间倒序返回已上线视频，作为 Feed 策略的基础实现。

#### GET `/api/feed/time`

请求参数：

| 参数 | 位置 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- | --- |
| `cursor` | query | string | 否 | - | 上一页返回的游标 |
| `limit` | query | int | 否 | 20 | 返回数量，最大 100 |

响应：

```json
{
  "items": [
    {
      "video_id": 1001,
      "author_id": 12,
      "title": "first video",
      "media_url": "https://example.com/video.mp4",
      "cover_url": "https://example.com/cover.jpg",
      "like_count": 0,
      "comment_count": 0,
      "favorite_count": 0,
      "published_at": "2026-05-03T12:00:00Z"
    }
  ],
  "next_cursor": "eyJwdWJsaXNoZWRfYXQiOiIyMDI2LTA1LTAzVDEyOjAwOjAwWiIsInZpZGVvX2lkIjoxMDAxfQ",
  "has_more": true
}
```

排序规则：

| 排序字段 | 方向 | 说明 |
| --- | --- | --- |
| `published_at` | DESC | 发布时间越新越靠前 |
| `id` | DESC | 同一发布时间下按视频ID倒序 |

游标内容：

| 字段 | 说明 |
| --- | --- |
| `published_at` | 当前页最后一条视频的发布时间 |
| `video_id` | 当前页最后一条视频ID |

### 2.2 View Event API

用于记录匿名游客或登录用户的曝光、观看、完播行为。

#### POST `/api/feed/view-events`

请求头：

| 名称 | 必填 | 说明 |
| --- | --- | --- |
| `Idempotency-Key` | 否 | 幂等键，建议每次事件上报使用唯一值 |

请求体：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `visitor_id` | string | 否 | 游客标识 |
| `video_id` | int64 | 是 | 视频ID |
| `event_type` | string | 是 | `IMPRESSION` / `VIEW` / `COMPLETE` |
| `watch_ms` | int | 否 | 观看时长，单位毫秒 |

响应：

```json
{
  "id": 1,
  "visitor_id": "visitor-001",
  "video_id": 1001,
  "event_type": "VIEW",
  "watch_ms": 3000,
  "created_at": "2026-05-03T12:00:05Z"
}
```

## 3. 数据表设计（最小实现）

### 3.1 `feed_cursor`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 记录ID |
| `user_id` | BIGINT | NULLABLE | 登录用户ID |
| `visitor_id` | VARCHAR(128) | NULLABLE | 游客标识 |
| `scene` | VARCHAR(32) | NOT NULL | 场景 |
| `cursor_token` | VARCHAR(255) | NOT NULL | 当前游标 |
| `expired_at` | DATETIME | NOT NULL | 过期时间 |
| `created_at` | DATETIME | NOT NULL | 创建时间 |

索引建议：`uk_user_scene(user_id, visitor_id, scene)`、`uk_cursor_token(cursor_token)`。

### 3.2 `feed_view_event`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 记录ID |
| `user_id` | BIGINT | NULLABLE | 登录用户ID |
| `visitor_id` | VARCHAR(128) | NULLABLE | 游客标识 |
| `video_id` | BIGINT | NOT NULL | 视频ID |
| `event_type` | VARCHAR(16) | NOT NULL | IMPRESSION/VIEW/COMPLETE |
| `watch_ms` | INT | NOT NULL, DEFAULT 0 | 观看时长 |
| `idempotency_key` | VARCHAR(128) | NULLABLE, UNIQUE | 幂等键 |
| `created_at` | DATETIME | NOT NULL | 事件时间 |

索引建议：`idx_user_time(user_id, created_at)`、`idx_visitor_time(visitor_id, created_at)`、`idx_video_time(video_id, created_at)`。
