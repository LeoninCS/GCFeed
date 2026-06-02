# 系统治理模块设计

## 1. 模块职责

系统治理模块负责限流、降级开关和失败任务重试，保障高峰期服务可用。

## 2. 接口设计

| 方法 | 接口路径 | 作用 | 鉴权 | 幂等键 |
| --- | --- | --- | --- | --- |
| POST | `/internal/rate-limit-decisions` | 判断当前请求是否放行 | 服务鉴权 | 支持 |
| GET | `/internal/governance/degrade-switches` | 获取降级开关状态 | 服务鉴权 | 无 |
| PATCH | `/api/admin/governance/degrade-switches/{key}` | 更新降级开关 | Bearer JWT(管理员) | 支持 |
| POST | `/internal/dead-letter-retries` | 重试死信任务 | 服务鉴权 | 支持 |

## 3. 数据表设计

### 3.1 `governance_degrade_switch`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 记录 ID |
| `switch_key` | VARCHAR(64) | UNIQUE, NOT NULL | 开关键 |
| `enabled` | TINYINT | NOT NULL, DEFAULT 0 | 0 关 / 1 开 |
| `updated_by` | BIGINT | NOT NULL | 更新人 |
| `updated_at` | DATETIME | NOT NULL | 更新时间 |

索引建议：`uk_switch_key(switch_key)`。

### 3.2 `governance_dead_letter`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 任务 ID |
| `task_type` | VARCHAR(64) | NOT NULL | 任务类型 |
| `payload` | JSON | NOT NULL | 任务参数 |
| `retry_count` | INT | NOT NULL, DEFAULT 0 | 重试次数 |
| `status` | TINYINT | NOT NULL, DEFAULT 1 | 1 待重试 / 2 已完成 / 3 放弃 |
| `last_error` | VARCHAR(512) | NULLABLE | 最后错误 |
| `updated_at` | DATETIME | NOT NULL | 更新时间 |
| `created_at` | DATETIME | NOT NULL | 创建时间 |

索引建议：`idx_status_updated(status, updated_at)`。

## 4. 业务规则

| 规则 | 说明 |
| --- | --- |
| 限流按资源维度决策 | 可按用户、IP、接口或场景组合判断 |
| 降级开关全局可查 | 内部服务读取开关后决定是否关闭非核心能力 |
| 管理员更新开关 | 更新动作写后台审计日志 |
| 死信重试有次数上限 | 超过上限后任务进入放弃状态 |
| 重试操作保持幂等 | 同一死信任务重复重试不会重复产生业务事实 |

## 5. 测试建议

| 场景 | 期望 |
| --- | --- |
| 请求限流决策 | 返回 allow 和原因 |
| 更新降级开关 | 开关状态变化 |
| 查询降级开关 | 返回当前开关集合 |
| 重试死信任务 | retry_count 增加，成功后状态完成 |
| 超过重试上限 | 状态变为放弃 |

## 6. 前端接入点

| 页面 | 接入能力 |
| --- | --- |
| 治理控制台 | 查看和更新降级开关 |
| 死信任务页 | 查看失败任务和触发重试 |
| 监控看板 | 展示限流和降级状态 |
