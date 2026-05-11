# Feed 模块设计（MVP）

## 1. 模块职责
负责刷视频主链路，输出游标分页结果并记录观看行为。

## 2. 接口设计（最小实现）

| 方法 | 接口路径 | 作用 | 鉴权 | 幂等键 |
| --- | --- | --- | --- | --- |
| GET | `/api/feed-items` | 获取按 scene 排序的视频流 | 可匿名 | - |

### 2.1 Feed Items API

用于返回已上线视频，支持时间线和热榜 Feed。

#### GET `/api/feed-items`

请求参数：

| 参数 | 位置 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- | --- |
| `cursor` | query | string | 否 | - | 上一页返回的游标 |
| `limit` | query | int | 否 | 10 | 返回数量，最大 100 |
| `scene` | query | string | 否 | `timeline` | Feed 场景：`timeline`、`hot` |

响应：

```json
{
  "scene": "timeline",
  "items": [
    {
      "video_id": 1001,
      "author_id": 12,
      "author_nickname": "tester",
      "author_avatar_url": "https://example.com/avatar.png",
      "title": "first video",
      "description": "hello timeline",
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

`scene=timeline`：

| 排序字段 | 方向 | 说明 |
| --- | --- | --- |
| `published_at` | DESC | 发布时间越新越靠前 |
| `id` | DESC | 同一发布时间下按视频ID倒序 |

`scene=hot`：

| 排序字段 | 方向 | 说明 |
| --- | --- | --- |
| `hot_score` | DESC | `like_count * 3 + comment_count * 5 + favorite_count * 4` |
| `published_at` | DESC | 同分时发布时间越新越靠前 |
| `id` | DESC | 同分同发布时间下按视频ID倒序 |

游标内容：

`scene=timeline`：

| 字段 | 说明 |
| --- | --- |
| `published_at` | 当前页最后一条视频的发布时间 |
| `video_id` | 当前页最后一条视频ID |

`scene=hot`：

| 字段 | 说明 |
| --- | --- |
| `hot_score` | 当前页最后一条视频的热度分 |
| `published_at` | 当前页最后一条视频的发布时间 |
| `video_id` | 当前页最后一条视频ID |

## 3. 数据表设计（最小实现）

Feed 依赖已有 `video` 和 `video_stat` 表读取已上线视频与互动计数。
