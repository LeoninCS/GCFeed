# 视频模块设计

## 1. 模块职责

视频模块负责视频发布、详情读取、作品列表、上传入口和软删除。互动计数由互动模块维护，视频模块负责读取展示。

## 2. 接口设计

| 方法 | 接口路径 | 作用 | 鉴权 | 幂等键 |
| --- | --- | --- | --- | --- |
| POST | `/api/videos` | 创建并发布视频 | Bearer JWT | 支持 |
| GET | `/api/videos/{videoId}` | 查询视频详情 | 可匿名 | 无 |
| DELETE | `/api/videos/{videoId}` | 删除视频 | Bearer JWT | 支持 |
| GET | `/api/users/{userId}/videos` | 查询用户公开视频列表 | 可匿名 | 无 |
| GET | `/api/users/me/videos` | 查询我的作品列表 | Bearer JWT | 无 |
| POST | `/api/uploads` | 上传媒体文件 | Bearer JWT | 支持 |

## 3. 数据表设计

### 3.1 `video`

`video` 保存视频主体信息和生命周期状态。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 视频 ID |
| `author_id` | BIGINT | NOT NULL | 作者 ID |
| `title` | VARCHAR(128) | NOT NULL | 标题 |
| `description` | VARCHAR(512) | NULLABLE | 视频简介 |
| `media_url` | VARCHAR(512) | NOT NULL | 视频地址 |
| `cover_url` | VARCHAR(512) | NOT NULL | 封面地址 |
| `status` | TINYINT | NOT NULL, DEFAULT 2 | 1 草稿 / 2 上线 / 3 下架 / 4 删除 |
| `published_at` | DATETIME | NULLABLE | 发布时间 |
| `created_at` | DATETIME | NOT NULL | 创建时间 |
| `updated_at` | DATETIME | NOT NULL | 更新时间 |

索引建议：

| 索引 | 字段 | 说明 |
| --- | --- | --- |
| `idx_author_status` | `author_id, status, created_at` | 作者作品列表 |
| `idx_status_published` | `status, published_at, id` | Timeline Feed |

### 3.2 `video_stat`

`video_stat` 保存视频高频统计数据。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `video_id` | BIGINT | PK | 视频 ID |
| `like_count` | INT | NOT NULL, DEFAULT 0 | 点赞数 |
| `comment_count` | INT | NOT NULL, DEFAULT 0 | 评论数 |
| `favorite_count` | INT | NOT NULL, DEFAULT 0 | 收藏数 |
| `created_at` | DATETIME | NOT NULL | 创建时间 |
| `updated_at` | DATETIME | NOT NULL | 更新时间 |

## 4. 业务规则

| 规则 | 说明 |
| --- | --- |
| 创建即发布 | 当前发布成功后状态为上线 |
| 创建统计行 | 发布视频时同步创建 `video_stat` |
| 作者可删除 | 作者删除视频时更新为删除状态 |
| 公开视频可读 | 匿名用户可查看上线视频 |
| 作品列表过滤状态 | 公开列表只返回上线视频，我的作品可返回作者自己的有效视频 |
| Feed 只读取上线视频 | Feed 查询只使用 `status=2` 视频 |
| 上传格式校验 | 视频限制扩展名、MIME、大小、时长、分辨率和编码 |
| MP4 首帧优化 | MP4/MOV 上传后执行 faststart，让播放元数据前置 |

## 5. 测试建议

| 场景 | 期望 |
| --- | --- |
| 登录用户发布视频 | 返回视频详情并创建 `video_stat` |
| 未登录发布视频 | 返回 401 |
| 查询上线视频详情 | 返回视频和计数 |
| 查询不存在视频 | 返回 404 |
| 作者删除视频 | 状态变为删除 |
| 非作者删除视频 | 返回权限错误 |
| 查询作者作品 | 只返回公开可见视频 |
| 上传非法媒体 | 返回 400 并清理失败文件 |
| 上传 MP4 视频 | 校验元数据并执行 faststart |

## 6. 前端接入点

| 页面 | 接入能力 |
| --- | --- |
| 发布页 | 上传文件、填写标题和简介、发布视频 |
| Feed 页 | 展示视频标题、作者、封面、播放地址和计数 |
| 视频详情页 | 展示单条视频详情 |
| 我的作品 | 展示当前用户发布的视频 |
| 公开主页 | 展示作者公开视频 |
