# 审核模块设计（MVP）

## 1. 模块职责
负责 Agent 初审、人审判定和违规下架。

## 2. 接口设计（最小实现）

| 方法 | 接口路径 | 作用 | 鉴权 | 幂等键 |
| --- | --- | --- | --- | --- |
| POST | `/internal/review/tasks` | 创建审核任务（提交待审） | 服务鉴权 | 支持 |
| POST | `/internal/review/tasks/{taskId}/agent-result` | 回传 Agent 初审结果 | 服务鉴权 | 支持 |
| POST | `/api/review/tasks/{taskId}/decision` | 人工审核通过/驳回 | Bearer JWT(运营角色) | 支持 |
| POST | `/api/review/videos/{videoId}/offline` | 强制下架视频 | Bearer JWT(运营角色) | 支持 |

## 3. 数据表设计（最小实现）

### 3.1 `review_task`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 任务ID |
| `video_id` | BIGINT | NOT NULL | 视频ID |
| `status` | TINYINT | NOT NULL, DEFAULT 1 | 1待初审/2待人审/3通过/4拒绝 |
| `agent_score` | DECIMAL(6,3) | NULLABLE | Agent 风险分 |
| `final_decision` | VARCHAR(16) | NULLABLE | PASS/REJECT |
| `reason` | VARCHAR(255) | NULLABLE | 原因 |
| `updated_by` | BIGINT | NULLABLE | 审核人 |
| `created_at` | DATETIME | NOT NULL | 创建时间 |
| `updated_at` | DATETIME | NOT NULL | 更新时间 |

索引建议：`idx_status_created(status, created_at)`、`idx_video(video_id)`。

### 3.2 `review_decision_log`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 日志ID |
| `task_id` | BIGINT | NOT NULL | 任务ID |
| `operator_type` | VARCHAR(16) | NOT NULL | AGENT/HUMAN |
| `decision` | VARCHAR(16) | NOT NULL | PASS/REJECT |
| `reason` | VARCHAR(255) | NULLABLE | 原因 |
| `created_at` | DATETIME | NOT NULL | 创建时间 |

索引建议：`idx_task_id(task_id)`。
