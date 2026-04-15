# 播放优化模块设计（MVP）

## 1. 模块职责
负责播放策略下发、预加载建议和播放质量上报。

## 2. 接口设计（最小实现）

| 方法 | 接口路径 | 作用 | 鉴权 | 幂等键 |
| --- | --- | --- | --- | --- |
| GET | `/api/playback/config` | 获取端侧播放参数 | Bearer JWT | - |
| POST | `/api/playback/preload` | 获取预加载视频列表 | Bearer JWT | 支持 |
| POST | `/internal/playback/qos/report` | 上报首帧/卡顿质量数据 | 服务鉴权 | 支持 |

## 3. 数据表设计（最小实现）

### 3.1 `playback_config`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 配置ID |
| `platform` | VARCHAR(16) | NOT NULL | iOS/Android/Web |
| `network_type` | VARCHAR(16) | NOT NULL | WiFi/4G/5G |
| `preload_count` | INT | NOT NULL | 预加载条数 |
| `buffer_ms` | INT | NOT NULL | 缓冲阈值 |
| `updated_at` | DATETIME | NOT NULL | 更新时间 |

索引建议：`uk_platform_network(platform, network_type)`。

### 3.2 `playback_qos_log`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 记录ID |
| `user_id` | BIGINT | NULLABLE | 用户ID |
| `video_id` | BIGINT | NOT NULL | 视频ID |
| `first_frame_ms` | INT | NULLABLE | 首帧耗时 |
| `stutter_count` | INT | NOT NULL, DEFAULT 0 | 卡顿次数 |
| `watch_ms` | INT | NOT NULL, DEFAULT 0 | 观看时长 |
| `created_at` | DATETIME | NOT NULL | 上报时间 |

索引建议：`idx_video_time(video_id, created_at)`。
