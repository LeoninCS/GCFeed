# 关系模块设计（MVP）

## 1. 模块职责
负责关注、取关和粉丝/关注列表查询。

## 2. 接口设计（最小实现）

| 方法 | 接口路径 | 作用 | 鉴权 | 幂等键 |
| --- | --- | --- | --- | --- |
| POST | `/api/relations/follows` | 关注用户 | Bearer JWT | 支持 |
| DELETE | `/api/relations/follows/{targetUserId}` | 取消关注 | Bearer JWT | 支持 |
| GET | `/api/relations/following` | 我的关注列表 | Bearer JWT | - |
| GET | `/api/relations/followers` | 我的粉丝列表 | Bearer JWT | - |

## 3. 数据表设计（最小实现）

### 3.1 `user_follow`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 关系ID |
| `user_id` | BIGINT | NOT NULL | 发起关注用户 |
| `target_user_id` | BIGINT | NOT NULL | 被关注用户 |
| `status` | TINYINT | NOT NULL, DEFAULT 1 | 1关注中/2已取关 |
| `created_at` | DATETIME | NOT NULL | 创建时间 |
| `updated_at` | DATETIME | NOT NULL | 更新时间 |

索引建议：`uk_user_target(user_id, target_user_id)`、`idx_target_status(target_user_id, status)`。
