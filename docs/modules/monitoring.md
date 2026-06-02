# 监控告警模块设计

## 1. 模块职责

监控告警模块负责业务指标采集、核心看板、告警规则和告警事件查询。

## 2. 接口设计

| 方法 | 接口路径 | 作用 | 鉴权 | 幂等键 |
| --- | --- | --- | --- | --- |
| POST | `/internal/metric-points` | 写入业务指标点 | 服务鉴权 | 支持 |
| GET | `/api/admin/metric-dashboard` | 查询核心看板数据 | Bearer JWT(运营角色) | 无 |
| POST | `/api/admin/alerts/rules` | 新建告警规则 | Bearer JWT(管理员) | 支持 |
| GET | `/api/admin/alerts/events` | 查询告警事件 | Bearer JWT(运营角色) | 无 |

## 3. 数据表设计

### 3.1 `monitor_metric_point`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 记录 ID |
| `metric_name` | VARCHAR(128) | NOT NULL | 指标名 |
| `labels` | JSON | NULLABLE | 标签 |
| `value` | DOUBLE | NOT NULL | 指标值 |
| `ts` | DATETIME | NOT NULL | 指标时间 |

索引建议：`idx_metric_ts(metric_name, ts)`。

### 3.2 `monitor_alert_rule`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 规则 ID |
| `rule_name` | VARCHAR(128) | UNIQUE, NOT NULL | 规则名 |
| `metric_name` | VARCHAR(128) | NOT NULL | 指标名 |
| `condition_expr` | VARCHAR(255) | NOT NULL | 告警条件 |
| `enabled` | TINYINT | NOT NULL, DEFAULT 1 | 是否启用 |
| `updated_at` | DATETIME | NOT NULL | 更新时间 |

索引建议：`idx_metric_enabled(metric_name, enabled)`。

### 3.3 `monitor_alert_event`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 事件 ID |
| `rule_id` | BIGINT | NOT NULL | 规则 ID |
| `status` | VARCHAR(16) | NOT NULL | `FIRING` / `RESOLVED` |
| `trigger_value` | DOUBLE | NOT NULL | 触发值 |
| `triggered_at` | DATETIME | NOT NULL | 触发时间 |
| `resolved_at` | DATETIME | NULLABLE | 恢复时间 |

索引建议：`idx_status_triggered(status, triggered_at)`。

## 4. 业务规则

| 规则 | 说明 |
| --- | --- |
| 指标点支持标签 | 标签保存接口、场景、实例等维度 |
| 看板按时间窗口聚合 | 支持最近分钟、小时和天级查询 |
| 告警规则可启停 | 停用规则不产生新事件 |
| 告警事件保留状态 | 触发和恢复都可查询 |
| 内部写入保持幂等 | 同一批指标重复写入结果稳定 |

## 5. 测试建议

| 场景 | 期望 |
| --- | --- |
| 写入指标点 | 数据入库 |
| 查询看板 | 返回聚合指标 |
| 创建告警规则 | 规则启用 |
| 查询告警事件 | 按状态和时间返回 |
| 非管理员创建规则 | 返回 403 |

## 6. 前端接入点

| 页面 | 接入能力 |
| --- | --- |
| 监控看板 | Feed、互动、播放、队列和数据库指标 |
| 告警规则页 | 创建和启停规则 |
| 告警事件页 | 查看触发和恢复记录 |
