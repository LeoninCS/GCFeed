# 监控告警模块设计（MVP）

## 1. 模块职责
负责指标采集、看板查询和告警规则触发。

## 2. 接口设计（最小实现）

| 方法 | 接口路径 | 作用 | 鉴权 | 幂等键 |
| --- | --- | --- | --- | --- |
| POST | `/internal/metrics/ingest` | 写入业务指标点 | 服务鉴权 | 支持 |
| GET | `/api/admin/metrics/dashboard` | 查询核心看板数据 | Bearer JWT(运营角色) | - |
| POST | `/api/admin/alerts/rules` | 新建告警规则 | Bearer JWT(管理员) | 支持 |
| GET | `/api/admin/alerts/events` | 查询告警事件 | Bearer JWT(运营角色) | - |

## 3. 数据表设计（最小实现）

### 3.1 `monitor_metric_point`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 记录ID |
| `metric_name` | VARCHAR(128) | NOT NULL | 指标名 |
| `labels` | JSON | NULLABLE | 标签 |
| `value` | DOUBLE | NOT NULL | 指标值 |
| `ts` | DATETIME | NOT NULL | 指标时间 |

索引建议：`idx_metric_ts(metric_name, ts)`。

### 3.2 `monitor_alert_rule`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 规则ID |
| `rule_name` | VARCHAR(128) | UNIQUE, NOT NULL | 规则名 |
| `metric_name` | VARCHAR(128) | NOT NULL | 指标名 |
| `condition_expr` | VARCHAR(255) | NOT NULL | 告警条件 |
| `enabled` | TINYINT | NOT NULL, DEFAULT 1 | 是否启用 |
| `updated_at` | DATETIME | NOT NULL | 更新时间 |

索引建议：`idx_metric_enabled(metric_name, enabled)`。

### 3.3 `monitor_alert_event`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 事件ID |
| `rule_id` | BIGINT | NOT NULL | 规则ID |
| `status` | VARCHAR(16) | NOT NULL | FIRING/RESOLVED |
| `trigger_value` | DOUBLE | NOT NULL | 触发值 |
| `triggered_at` | DATETIME | NOT NULL | 触发时间 |
| `resolved_at` | DATETIME | NULLABLE | 恢复时间 |

索引建议：`idx_status_triggered(status, triggered_at)`。
