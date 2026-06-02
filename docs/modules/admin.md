# 后台运营模块设计

## 1. 模块职责

后台运营模块负责视频查询、审核任务查询、审核分配、运营配置和操作审计。

## 2. 接口设计

| 方法 | 接口路径 | 作用 | 鉴权 | 幂等键 |
| --- | --- | --- | --- | --- |
| GET | `/api/admin/videos` | 按条件查询视频 | Bearer JWT(运营角色) | 无 |
| GET | `/api/admin/review/tasks` | 查询审核任务 | Bearer JWT(运营角色) | 无 |
| PUT | `/api/admin/review/tasks/{taskId}/assignee` | 分配审核员 | Bearer JWT(运营角色) | 支持 |
| PATCH | `/api/admin/configs/{configKey}` | 更新配置项 | Bearer JWT(管理员) | 支持 |

## 3. 数据表设计

### 3.1 `admin_config`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 配置 ID |
| `config_key` | VARCHAR(128) | UNIQUE, NOT NULL | 配置键 |
| `config_value` | JSON | NOT NULL | 配置值 |
| `updated_by` | BIGINT | NOT NULL | 更新人 |
| `updated_at` | DATETIME | NOT NULL | 更新时间 |

索引建议：`uk_config_key(config_key)`。

### 3.2 `admin_action_log`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 日志 ID |
| `operator_id` | BIGINT | NOT NULL | 操作人 |
| `action_type` | VARCHAR(64) | NOT NULL | 操作类型 |
| `target_type` | VARCHAR(32) | NOT NULL | `VIDEO` / `REVIEW` / `CONFIG` |
| `target_id` | VARCHAR(64) | NOT NULL | 目标 ID |
| `detail` | JSON | NULLABLE | 操作详情 |
| `created_at` | DATETIME | NOT NULL | 操作时间 |

索引建议：`idx_operator_time(operator_id, created_at)`。

## 4. 业务规则

| 规则 | 说明 |
| --- | --- |
| 后台接口需要运营权限 | 普通用户不能访问后台接口 |
| 管理员才能改配置 | 配置更新需要管理员角色 |
| 关键操作写审计日志 | 分配审核员、下架视频、更新配置都写 `admin_action_log` |
| 查询接口支持筛选 | 视频和审核任务可按状态、作者、时间查询 |
| 配置以 key 为唯一入口 | 相同 `config_key` 更新同一配置 |

## 5. 测试建议

| 场景 | 期望 |
| --- | --- |
| 运营查询视频 | 返回分页结果 |
| 普通用户访问后台 | 返回 403 |
| 分配审核员 | 审核任务 assignee 更新并写审计 |
| 管理员更新配置 | 配置值变更并写审计 |
| 非管理员更新配置 | 返回 403 |

## 6. 前端接入点

| 页面 | 接入能力 |
| --- | --- |
| 后台视频列表 | 条件筛选、状态查看、下架入口 |
| 审核任务列表 | 查询、分配、进入审核详情 |
| 运营配置页 | 配置查看和更新 |
| 操作日志页 | 审计记录查询 |
