# 互动模块设计（MVP）

## 1. 模块职责
负责点赞、评论、收藏三类互动写入和查询。

## 2. 接口设计（最小实现）

| 方法 | 接口路径 | 作用 | 鉴权 | 幂等键 |
| --- | --- | --- | --- | --- |
| POST | `/api/interactions/likes/toggle` | 点赞/取消点赞 | Bearer JWT | 支持 |
| POST | `/api/interactions/favorites/toggle` | 收藏/取消收藏 | Bearer JWT | 支持 |
| POST | `/api/interactions/comments` | 发表评论 | Bearer JWT | 支持 |
| DELETE | `/api/interactions/comments/{commentId}` | 删除评论 | Bearer JWT | 支持 |
| GET | `/api/interactions/comments` | 获取评论列表 | 可匿名 | - |

## 3. 数据表设计（最小实现）

### 3.1 `interaction_action`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 记录ID |
| `user_id` | BIGINT | NOT NULL | 用户ID |
| `video_id` | BIGINT | NOT NULL | 视频ID |
| `action_type` | VARCHAR(16) | NOT NULL | LIKE/FAVORITE |
| `status` | TINYINT | NOT NULL, DEFAULT 1 | 1有效/2取消 |
| `updated_at` | DATETIME | NOT NULL | 更新时间 |
| `created_at` | DATETIME | NOT NULL | 创建时间 |

索引建议：`uk_user_video_type(user_id, video_id, action_type)`、`idx_video_type(video_id, action_type, status)`。

### 3.2 `interaction_comment`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 评论ID |
| `video_id` | BIGINT | NOT NULL | 视频ID |
| `user_id` | BIGINT | NOT NULL | 评论用户 |
| `content` | VARCHAR(1000) | NOT NULL | 评论内容 |
| `status` | TINYINT | NOT NULL, DEFAULT 1 | 1正常/2删除 |
| `created_at` | DATETIME | NOT NULL | 创建时间 |
| `updated_at` | DATETIME | NOT NULL | 更新时间 |

索引建议：`idx_video_created(video_id, created_at)`。
