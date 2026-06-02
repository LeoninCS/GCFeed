# 账户模块设计

## 1. 模块职责

账户模块负责注册、登录、登出、当前用户资料、公开资料和个人资料更新，为其他模块提供统一身份能力。

## 2. 接口设计

| 方法 | 接口路径 | 作用 | 鉴权 | 幂等键 |
| --- | --- | --- | --- | --- |
| POST | `/api/users` | 用户注册 | 无 | 支持 |
| POST | `/api/sessions` | 密码登录并获取 Token | 无 | 无 |
| DELETE | `/api/sessions/current` | 用户登出 | Bearer JWT | 无 |
| GET | `/api/users/me` | 获取当前用户资料 | Bearer JWT | 无 |
| PATCH | `/api/users/me` | 更新头像、昵称、简介 | Bearer JWT | 支持 |
| GET | `/api/users/{userId}` | 获取公开用户资料 | 可匿名 | 无 |

## 3. 数据表设计

### 3.1 `user`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 用户 ID |
| `account` | VARCHAR(64) | UNIQUE, NOT NULL | 登录账号 |
| `password_hash` | VARCHAR(255) | NOT NULL | 密码哈希 |
| `nickname` | VARCHAR(64) | NOT NULL | 昵称 |
| `avatar_url` | VARCHAR(512) | NULLABLE | 头像 |
| `bio` | VARCHAR(255) | NULLABLE | 简介 |
| `status` | TINYINT | NOT NULL, DEFAULT 1 | 1 正常 / 2 冻结 / 3 注销 |
| `created_at` | DATETIME | NOT NULL | 创建时间 |
| `updated_at` | DATETIME | NOT NULL | 更新时间 |

索引建议：

| 索引 | 字段 | 说明 |
| --- | --- | --- |
| `uk_account` | `account` | 保证账号唯一 |

## 4. 业务规则

| 规则 | 说明 |
| --- | --- |
| 账号唯一 | 注册时同一 `account` 只能创建一个用户 |
| 密码只保存哈希 | 接口和数据库都不保存明文密码 |
| 登录只允许正常账号 | 冻结和注销用户不能登录 |
| 当前用户资料走鉴权 | `/api/users/me` 只返回当前登录用户 |
| 公开资料隐藏敏感字段 | 公开接口不返回 `account` 和 `password_hash` |
| 资料字段做长度限制 | 昵称、头像和简介由 Domain 层校验 |

## 5. 测试建议

| 场景 | 期望 |
| --- | --- |
| 注册新用户 | 返回用户资料，密码哈希写入数据库 |
| 重复账号注册 | 返回冲突或业务错误 |
| 正确密码登录 | 返回 access token |
| 错误密码登录 | 返回 401 或业务错误 |
| 未登录访问当前用户 | 返回 401 |
| 更新个人资料 | 返回更新后的昵称、头像和简介 |

## 6. 前端接入点

| 页面 | 接入能力 |
| --- | --- |
| 登录/注册页 | 注册、登录、错误提示 |
| 顶部用户区 | 展示当前用户资料和登出 |
| 个人主页 | 展示和编辑个人资料 |
| 作者主页 | 展示公开用户资料 |
