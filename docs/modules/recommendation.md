# 推荐模块设计

## 1. 模块职责

推荐模块负责候选召回、排序打散和曝光去重，为 Feed 提供可下发的视频列表。

## 2. 接口设计

| 方法 | 接口路径 | 作用 | 鉴权 | 幂等键 |
| --- | --- | --- | --- | --- |
| POST | `/internal/recommendation-candidates` | 一次完成召回、排序、打散 | 服务鉴权 | 支持 |
| POST | `/internal/exposure-decisions` | 判断候选是否近期曝光 | 服务鉴权 | 支持 |
| POST | `/internal/exposures` | 写入曝光记录 | 服务鉴权 | 支持 |

## 3. 数据表设计

### 3.1 `reco_rule`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 规则 ID |
| `scene` | VARCHAR(32) | NOT NULL | 场景，如 `feed` |
| `config_json` | JSON | NOT NULL | 召回、排序、打散参数 |
| `enabled` | TINYINT | NOT NULL, DEFAULT 1 | 是否启用 |
| `updated_at` | DATETIME | NOT NULL | 更新时间 |

索引建议：`uk_scene(scene)`。

### 3.2 `reco_exposure_log`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 记录 ID |
| `user_id` | BIGINT | NOT NULL | 用户 ID |
| `video_id` | BIGINT | NOT NULL | 视频 ID |
| `scene` | VARCHAR(32) | NOT NULL | 场景 |
| `request_id` | VARCHAR(64) | NOT NULL | 请求 ID |
| `exposed_at` | DATETIME | NOT NULL | 曝光时间 |

索引建议：`idx_user_scene_time(user_id, scene, exposed_at)`、`idx_request_id(request_id)`。

## 4. 业务规则

| 规则 | 说明 |
| --- | --- |
| 候选只返回上线视频 | 下架、删除和异常视频不进入候选 |
| 曝光去重按用户生效 | 同一用户近期曝光过的视频降低或移除优先级 |
| 打散避免同作者集中 | 同一作者的视频在单页中保持间隔 |
| 请求携带 scene | 不同 Feed 场景可使用不同策略 |
| 内部接口支持幂等 | 重复曝光写入不会产生重复事实 |

## 5. 测试建议

| 场景 | 期望 |
| --- | --- |
| 请求推荐候选 | 返回上线视频列表 |
| 同一作者候选过多 | 结果被打散 |
| 判断近期曝光视频 | 返回已曝光状态 |
| 写入曝光记录 | 记录 request_id 和曝光时间 |
| 重复写入曝光 | 结果稳定 |

## 6. 前端接入点

推荐模块主要服务后端 Feed。前端通过 Feed 接口间接使用推荐结果。
