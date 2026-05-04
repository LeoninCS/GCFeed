# 视频模块设计（MVP）

## 1. 模块职责
负责视频发布、详情读取和删除（软删除）。

## 2. 接口设计（最小实现）

| 方法 | 接口路径 | 作用 | 鉴权 | 幂等键 |
| --- | --- | --- | --- | --- |
| POST | `/api/videos` | 创建并发布视频 | Bearer JWT | 支持 |
| GET | `/api/videos/{videoId}` | 查询视频详情 | 可匿名 | - |
| DELETE | `/api/videos/{videoId}` | 删除视频（软删除） | Bearer JWT | 支持 |
| GET | `/api/users/{userId}/videos` | 根据用户ID查询作品列表 | 可匿名 | - |

## 3. 数据表设计（最小实现）

### 3.1 `video`

`video` 保存视频主体信息和生命周期状态。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 视频ID |
| `author_id` | BIGINT | NOT NULL | 作者ID |
| `title` | VARCHAR(128) | NOT NULL | 标题 |
| `description` | VARCHAR(512) | NULLABLE | 视频简介 |
| `media_url` | VARCHAR(512) | NOT NULL | 视频地址 |
| `cover_url` | VARCHAR(512) | NOT NULL | 封面地址 |
| `status` | TINYINT | NOT NULL, DEFAULT 2 | 1草稿/2上线/3下架/4删除 |
| `published_at` | DATETIME | NULLABLE | 发布时间 |
| `created_at` | DATETIME | NOT NULL | 创建时间 |
| `updated_at` | DATETIME | NOT NULL | 更新时间 |

索引建议：`idx_author_status(author_id, status, created_at)`、`idx_status_published(status, published_at)`。

### 3.2 `video_stat`

`video_stat` 保存视频高频统计数据，供视频详情和 Feed 列表读取。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `video_id` | BIGINT | PK | 视频ID，对应 `video.id` |
| `like_count` | INT | NOT NULL, DEFAULT 0 | 点赞数 |
| `comment_count` | INT | NOT NULL, DEFAULT 0 | 评论数 |
| `favorite_count` | INT | NOT NULL, DEFAULT 0 | 收藏数 |
| `created_at` | DATETIME | NOT NULL | 创建时间 |
| `updated_at` | DATETIME | NOT NULL | 更新时间 |

索引建议：`pk_video_id(video_id)`。

写入规则：

1. 创建视频时同步创建 `video_stat`，三个计数字段初始值为 0。
2. 点赞、收藏、评论写入时，由互动模块在同一事务内更新 `video_stat`。
3. 视频详情、作品列表和 Feed 列表读取时关联 `video_stat` 返回计数。
