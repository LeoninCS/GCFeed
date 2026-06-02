# 审核模块设计

## 1. 模块职责

审核模块负责 Agent 初审、人审判定、审核日志和违规内容下架，是内容发布进入公开 Feed 前的治理入口。

## 2. 接口设计

| 方法 | 接口路径 | 作用 | 鉴权 | 幂等键 |
| --- | --- | --- | --- | --- |
| POST | `/internal/review/tasks` | 创建审核任务 | 服务鉴权 | 支持 |
| PUT | `/internal/review/tasks/{taskId}/agent-result` | 回传 Agent 初审结果 | 服务鉴权 | 支持 |
| PUT | `/api/review/tasks/{taskId}/decision` | 人工审核通过或驳回 | Bearer JWT(运营角色) | 支持 |
| PATCH | `/api/videos/{videoId}` | 强制下架视频 | Bearer JWT(运营角色) | 支持 |

## 3. 数据表设计

### 3.1 `review_task`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 任务 ID |
| `video_id` | BIGINT | NOT NULL | 视频 ID |
| `status` | TINYINT | NOT NULL, DEFAULT 1 | 1 待初审 / 2 待人审 / 3 通过 / 4 拒绝 |
| `agent_score` | DECIMAL(6,3) | NULLABLE | Agent 风险分 |
| `final_decision` | VARCHAR(16) | NULLABLE | `PASS` / `REJECT` |
| `reason` | VARCHAR(255) | NULLABLE | 原因 |
| `updated_by` | BIGINT | NULLABLE | 审核人 |
| `created_at` | DATETIME | NOT NULL | 创建时间 |
| `updated_at` | DATETIME | NOT NULL | 更新时间 |

索引建议：`idx_status_created(status, created_at)`、`idx_video(video_id)`。

### 3.2 `review_decision_log`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 日志 ID |
| `task_id` | BIGINT | NOT NULL | 任务 ID |
| `operator_type` | VARCHAR(16) | NOT NULL | `AGENT` / `HUMAN` |
| `decision` | VARCHAR(16) | NOT NULL | `PASS` / `REJECT` |
| `reason` | VARCHAR(255) | NULLABLE | 原因 |
| `created_at` | DATETIME | NOT NULL | 创建时间 |

索引建议：`idx_task_id(task_id)`。

## 4. 业务规则

| 规则 | 说明 |
| --- | --- |
| 视频提交生成审核任务 | 发布或待审流程创建 `review_task` |
| Agent 结果进入日志 | 初审结果写入 `review_decision_log` |
| 高风险进入人审 | 风险分超过阈值时任务状态变为待人审 |
| 人审决定最终生效 | 人审通过或驳回写入最终决定 |
| 驳回可触发下架 | 审核拒绝时视频状态更新为下架 |
| 操作保留审计 | 每次 Agent 或人工决定都写日志 |

## 5. 测试建议

| 场景 | 期望 |
| --- | --- |
| 创建审核任务 | 状态为待初审 |
| Agent 通过 | 任务状态变为通过并写日志 |
| Agent 高风险 | 任务状态变为待人审 |
| 人工驳回 | 任务状态变为拒绝，视频下架 |
| 重复回传结果 | 返回同一审核状态 |
| 非运营用户人审 | 返回 403 |

## 6. 前端接入点

| 页面 | 接入能力 |
| --- | --- |
| 审核工作台 | 待审任务列表、风险分、通过、驳回 |
| 视频管理 | 强制下架和原因展示 |
| 消息中心 | 作者接收审核结果通知 |
