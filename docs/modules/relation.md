# 关系模块设计

## 1. 模块职责

关系模块负责用户之间的关注关系，提供关注、取关、关注列表和粉丝列表能力。

模块边界：

| 模块 | 职责 |
| --- | --- |
| `relation` | 保存关注关系，处理关注、取关、列表查询和计数维护 |
| `account` | 提供用户基础资料、账号状态和登录身份 |
| `message` | 消费关注事件，生成站内通知 |
| `feed` | 后续可基于关注关系扩展关注流 |
| `recommendation` | 后续可使用关注关系作为兴趣特征 |

## 2. 实现结构

```text
apps/api/internal/domain/relation/
apps/api/internal/application/relation/
apps/api/internal/infra/persistence/relation/
apps/api/internal/interfaces/http/relation/
```

## 3. 接口设计

| 方法 | 接口路径 | 作用 | 鉴权 | 幂等键 |
| --- | --- | --- | --- | --- |
| PUT | `/api/users/me/following/{targetUserId}` | 关注用户 | Bearer JWT | 支持 |
| DELETE | `/api/users/me/following/{targetUserId}` | 取消关注 | Bearer JWT | 支持 |
| GET | `/api/users/me/following` | 我的关注列表 | Bearer JWT | - |
| GET | `/api/users/me/followers` | 我的粉丝列表 | Bearer JWT | - |

### 3.1 关注用户

#### PUT `/api/users/me/following/{targetUserId}`

请求头：

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `Authorization` | 是 | `Bearer <access_token>` |
| `Idempotency-Key` | 否 | 客户端幂等键，最长 128 |

路径参数：

| 参数 | 类型 | 必填 | 说明 |
| --- | --- | --- | --- |
| `targetUserId` | int64 | 是 | 被关注用户ID |

请求体：空

响应：

```json
{
  "user_id": 1001,
  "target_user_id": 1002,
  "status": 1,
  "following": true,
  "following_count": 18,
  "follower_count": 42
}
```

处理规则：

| 场景 | 结果 |
| --- | --- |
| 首次关注 | 新增 `user_follow` 记录，状态为 `1关注中` |
| 再次关注同一用户 | 返回当前关注关系 |
| 曾经取关后重新关注 | 将状态更新为 `1关注中` |
| 目标用户异常 | 返回 `404` 或业务错误 |

关注成功后发布 `FOLLOW_CREATED` 领域事件，供消息模块生成关注通知。

### 3.2 取消关注

#### DELETE `/api/users/me/following/{targetUserId}`

请求头：

| 参数 | 必填 | 说明 |
| --- | --- | --- |
| `Authorization` | 是 | `Bearer <access_token>` |
| `Idempotency-Key` | 否 | 客户端幂等键，最长 128 |

响应：

```json
{
  "user_id": 1001,
  "target_user_id": 1002,
  "status": 2,
  "following": false,
  "following_count": 17,
  "follower_count": 41
}
```

处理规则：

| 场景 | 结果 |
| --- | --- |
| 当前已关注 | 将状态更新为 `2已取关` |
| 重复取消关注 | 返回当前关系状态 |
| 目标用户异常 | 返回 `404` 或业务错误 |

取关操作只更新关系状态，历史记录保留用于审计和反作弊分析。

### 3.3 我的关注列表

#### GET `/api/users/me/following`

请求参数：

| 参数 | 位置 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- | --- |
| `cursor` | query | string | 否 | - | 上一页返回的游标 |
| `limit` | query | int | 否 | 20 | 返回数量，最大 100 |

响应：

```json
{
  "items": [
    {
      "user_id": 1002,
      "nickname": "creator",
      "avatar_url": "https://example.com/avatar.png",
      "bio": "daily maker",
      "followed_at": "2026-05-04T12:00:00Z"
    }
  ],
  "next_cursor": "eyJmb2xsb3dlZF9hdCI6IjIwMjYtMDUtMDRUMTI6MDA6MDBaIiwidXNlcl9pZCI6MTAwMn0",
  "has_more": true
}
```

排序规则：

| 排序字段 | 方向 | 说明 |
| --- | --- | --- |
| `updated_at` | DESC | 最近关注靠前 |
| `target_user_id` | DESC | 同一更新时间下稳定排序 |

游标内容：

| 字段 | 说明 |
| --- | --- |
| `followed_at` | 当前页最后一条关注关系的更新时间 |
| `user_id` | 当前页最后一个被关注用户ID |

### 3.4 我的粉丝列表

#### GET `/api/users/me/followers`

请求参数：

| 参数 | 位置 | 类型 | 必填 | 默认值 | 说明 |
| --- | --- | --- | --- | --- | --- |
| `cursor` | query | string | 否 | - | 上一页返回的游标 |
| `limit` | query | int | 否 | 20 | 返回数量，最大 100 |

响应：

```json
{
  "items": [
    {
      "user_id": 1003,
      "nickname": "viewer",
      "avatar_url": "https://example.com/avatar.png",
      "bio": "video lover",
      "followed_at": "2026-05-04T12:10:00Z"
    }
  ],
  "next_cursor": "eyJmb2xsb3dlZF9hdCI6IjIwMjYtMDUtMDRUMTI6MTA6MDBaIiwidXNlcl9pZCI6MTAwM30",
  "has_more": true
}
```

排序规则：

| 排序字段 | 方向 | 说明 |
| --- | --- | --- |
| `updated_at` | DESC | 最近成为粉丝靠前 |
| `user_id` | DESC | 同一更新时间下稳定排序 |

游标内容：

| 字段 | 说明 |
| --- | --- |
| `followed_at` | 当前页最后一条关注关系的更新时间 |
| `user_id` | 当前页最后一个粉丝用户ID |

## 4. 数据表设计

### 4.1 `user_follow`

`user_follow` 保存用户之间的关注关系，同一组 `user_id + target_user_id` 只保留一条记录。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 关系ID |
| `user_id` | BIGINT | NOT NULL | 发起关注用户 |
| `target_user_id` | BIGINT | NOT NULL | 被关注用户 |
| `status` | TINYINT | NOT NULL, DEFAULT 1 | 1关注中/2已取关 |
| `idempotency_key` | VARCHAR(128) | NULLABLE | 最近一次写入幂等键 |
| `created_at` | DATETIME | NOT NULL | 创建时间 |
| `updated_at` | DATETIME | NOT NULL | 更新时间 |

索引建议：

| 索引 | 字段 | 说明 |
| --- | --- | --- |
| `uk_user_target` | `user_id, target_user_id` | 保证同一用户对同一目标用户只有一条关系记录 |
| `idx_user_status_updated` | `user_id, status, updated_at, target_user_id` | 支持关注列表分页 |
| `idx_target_status_updated` | `target_user_id, status, updated_at, user_id` | 支持粉丝列表分页 |

### 4.2 `user_relation_stat`

`user_relation_stat` 保存关注数和粉丝数，用于个人页和列表页快速展示。

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `user_id` | BIGINT | PK | 用户ID |
| `following_count` | BIGINT | NOT NULL, DEFAULT 0 | 关注数 |
| `follower_count` | BIGINT | NOT NULL, DEFAULT 0 | 粉丝数 |
| `created_at` | DATETIME | NOT NULL | 创建时间 |
| `updated_at` | DATETIME | NOT NULL | 更新时间 |

更新规则：

| 操作 | `following_count` | `follower_count` |
| --- | --- | --- |
| 关注成功 | 当前用户 +1 | 目标用户 +1 |
| 取消关注成功 | 当前用户 -1 | 目标用户 -1 |
| 重复关注 | 保持当前值 | 保持当前值 |
| 重复取消关注 | 保持当前值 | 保持当前值 |

计数更新和关系状态更新放在同一个数据库事务内完成。

## 5. 领域规则

| 规则 | 说明 |
| --- | --- |
| 关注目标必须是其他正常用户 | `user_id` 和 `target_user_id` 必须指向两个不同的正常用户 |
| 关注关系使用软状态 | 取关通过 `status=2` 表示，保留历史记录 |
| 写操作支持幂等 | 相同幂等键重复提交返回同一业务结果 |
| 列表只返回有效关系 | 关注列表和粉丝列表查询 `status=1` 的记录 |
| 列表使用游标分页 | 按 `updated_at DESC, user_id DESC` 或 `updated_at DESC, target_user_id DESC` 稳定翻页 |

## 6. 错误码建议

| HTTP 状态码 | 业务错误 | 场景 |
| --- | --- | --- |
| `400` | `INVALID_TARGET_USER_ID` | `target_user_id` 格式异常 |
| `400` | `INVALID_CURSOR` | 游标解析失败 |
| `400` | `INVALID_LIMIT` | `limit` 超出范围 |
| `400` | `FOLLOW_SELF_FORBIDDEN` | 关注目标是当前用户 |
| `401` | `UNAUTHORIZED` | 登录态缺失或 Token 异常 |
| `404` | `TARGET_USER_NOT_FOUND` | 目标用户不存在或状态异常 |
| `500` | `RELATION_SAVE_FAILED` | 保存关注关系失败 |
| `500` | `RELATION_LOAD_FAILED` | 查询关注关系失败 |

## 7. 事件设计

### 7.1 `FOLLOW_CREATED`

关注成功后发布事件：

```json
{
  "event_id": "follow:1001:1002:20260504120000",
  "event_type": "FOLLOW_CREATED",
  "user_id": 1001,
  "target_user_id": 1002,
  "created_at": "2026-05-04T12:00:00Z"
}
```

消费方：

| 消费方 | 用途 |
| --- | --- |
| `message` | 给被关注用户生成关注通知 |
| `recommendation` | 更新用户关系特征 |
| `feed` | 后续生成关注流候选 |

## 8. 测试建议

| 测试场景 | 期望结果 |
| --- | --- |
| 首次关注正常用户 | 返回 `following=true`，双方计数增加 |
| 重复关注同一用户 | 返回当前关注状态，计数保持稳定 |
| 取关已关注用户 | 返回 `following=false`，双方计数减少 |
| 重复取关 | 返回当前取关状态，计数保持稳定 |
| 关注当前用户 | 返回 `400 FOLLOW_SELF_FORBIDDEN` |
| 关注异常目标用户 | 返回 `404 TARGET_USER_NOT_FOUND` |
| 查询关注列表 | 按最近关注时间倒序返回 |
| 查询粉丝列表 | 按最近成为粉丝时间倒序返回 |
| 游标翻页 | 多页结果稳定且无重复 |
