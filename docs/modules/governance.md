# 系统治理模块设计（MVP）

## 1. 模块职责
负责限流、降级开关和失败任务重试，保障高峰期服务可用。

## 2. 接口设计（最小实现）

| 方法 | 接口路径 | 作用 | 鉴权 | 幂等键 |
| --- | --- | --- | --- | --- |
| POST | `/internal/governance/rate-limit/check` | 判断当前请求是否放行 | 服务鉴权 | 支持 |
| GET | `/internal/governance/degrade-switches` | 获取降级开关状态 | 服务鉴权 | - |
| PATCH | `/api/admin/governance/degrade-switches/{key}` | 更新降级开关 | Bearer JWT(管理员) | 支持 |
| POST | `/internal/governance/dead-letter/retry` | 重试死信任务 | 服务鉴权 | 支持 |

## 3. 数据表设计（最小实现）

### 3.1 `governance_degrade_switch`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 记录ID |
| `switch_key` | VARCHAR(64) | UNIQUE, NOT NULL | 开关键 |
| `enabled` | TINYINT | NOT NULL, DEFAULT 0 | 0关/1开 |
| `updated_by` | BIGINT | NOT NULL | 更新人 |
| `updated_at` | DATETIME | NOT NULL | 更新时间 |

索引建议：`uk_switch_key(switch_key)`。

### 3.2 `governance_dead_letter`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 任务ID |
| `task_type` | VARCHAR(64) | NOT NULL | 任务类型 |
| `payload` | JSON | NOT NULL | 任务参数 |
| `retry_count` | INT | NOT NULL, DEFAULT 0 | 重试次数 |
| `status` | TINYINT | NOT NULL, DEFAULT 1 | 1待重试/2已完成/3放弃 |
| `last_error` | VARCHAR(512) | NULLABLE | 最后错误 |
| `updated_at` | DATETIME | NOT NULL | 更新时间 |
| `created_at` | DATETIME | NOT NULL | 创建时间 |

索引建议：`idx_status_updated(status, updated_at)`。
