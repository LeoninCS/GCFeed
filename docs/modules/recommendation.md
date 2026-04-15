# 推荐模块设计（MVP）

## 1. 模块职责
负责候选召回、排序打散和曝光去重，为 Feed 提供可下发列表。

## 2. 接口设计（最小实现）

| 方法 | 接口路径 | 作用 | 鉴权 | 幂等键 |
| --- | --- | --- | --- | --- |
| POST | `/internal/reco/candidates` | 一次完成召回+排序+打散 | 服务鉴权 | 支持 |
| POST | `/internal/reco/exposure/check` | 校验候选是否近期曝光 | 服务鉴权 | 支持 |
| POST | `/internal/reco/exposure/commit` | 写入曝光记录 | 服务鉴权 | 支持 |

## 3. 数据表设计（最小实现）

### 3.1 `reco_rule`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 规则ID |
| `scene` | VARCHAR(32) | NOT NULL | 场景，如 feed |
| `config_json` | JSON | NOT NULL | 召回/排序参数 |
| `enabled` | TINYINT | NOT NULL, DEFAULT 1 | 是否启用 |
| `updated_at` | DATETIME | NOT NULL | 更新时间 |

索引建议：`uk_scene(scene)`。

### 3.2 `reco_exposure_log`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 记录ID |
| `user_id` | BIGINT | NOT NULL | 用户ID |
| `video_id` | BIGINT | NOT NULL | 视频ID |
| `scene` | VARCHAR(32) | NOT NULL | 场景 |
| `request_id` | VARCHAR(64) | NOT NULL | 请求ID |
| `exposed_at` | DATETIME | NOT NULL | 曝光时间 |

索引建议：`idx_user_scene_time(user_id, scene, exposed_at)`、`idx_request_id(request_id)`。
