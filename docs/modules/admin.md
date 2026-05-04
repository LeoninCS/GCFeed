# 后台运营模块设计（MVP）

## 1. 模块职责
负责运营后台的审核分配、内容查询和关键配置管理。

## 2. 接口设计（最小实现）

| 方法 | 接口路径 | 作用 | 鉴权 | 幂等键 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/videos` | 按条件查询视频 | Bearer JWT(运营角色) | - |
| GET | `/api/admin/review/tasks` | 查询待处理审核任务 | Bearer JWT(运营角色) | - |
| PUT | `/api/admin/review/tasks/{taskId}/assignee` | 分配审核员 | Bearer JWT(运营角色) | 支持 |
| PATCH | `/api/admin/configs/{configKey}` | 更新配置项 | Bearer JWT(管理员) | 支持 |

## 3. 数据表设计（最小实现）

### 3.1 `admin_config`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 配置ID |
| `config_key` | VARCHAR(128) | UNIQUE, NOT NULL | 配置键 |
| `config_value` | JSON | NOT NULL | 配置值 |
| `updated_by` | BIGINT | NOT NULL | 更新人 |
| `updated_at` | DATETIME | NOT NULL | 更新时间 |

索引建议：`uk_config_key(config_key)`。

### 3.2 `admin_action_log`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 日志ID |
| `operator_id` | BIGINT | NOT NULL | 操作人 |
| `action_type` | VARCHAR(64) | NOT NULL | 操作类型 |
| `target_type` | VARCHAR(32) | NOT NULL | VIDEO/REVIEW/CONFIG |
| `target_id` | VARCHAR(64) | NOT NULL | 目标ID |
| `detail` | JSON | NULLABLE | 操作详情 |
| `created_at` | DATETIME | NOT NULL | 操作时间 |

索引建议：`idx_operator_time(operator_id, created_at)`。
