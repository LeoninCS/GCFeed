# 账户模块设计（MVP）

## 1. 模块职责
负责注册登录和个人资料维护，提供统一身份能力。

## 2. 接口设计（最小实现）

| 方法 | 接口路径 | 作用 | 鉴权 | 幂等键 |
| --- | --- | --- | --- | --- |
| POST | `/api/auth/register` | 用户注册 | 无 | 支持 |
| POST | `/api/auth/login/password` | 密码登录并获取 Token | 无 | - |
| POST | `/api/auth/logout` | 用户登出 | Bearer JWT | - |
| GET | `/api/users/me` | 获取当前用户资料 | Bearer JWT | - |
| PATCH | `/api/users/me` | 更新头像、昵称、简介 | Bearer JWT | 支持 |

## 3. 数据表设计（最小实现）

### 3.1 `user`

| 字段 | 类型 | 约束 | 说明 |
| --- | --- | --- | --- |
| `id` | BIGINT | PK | 用户ID |
| `account` | VARCHAR(64) | UNIQUE, NOT NULL | 登录账号（邮箱或手机号） |
| `password_hash` | VARCHAR(255) | NOT NULL | 密码哈希 |
| `nickname` | VARCHAR(64) | NOT NULL | 昵称 |
| `avatar_url` | VARCHAR(512) | NULLABLE | 头像 |
| `bio` | VARCHAR(255) | NULLABLE | 简介 |
| `status` | TINYINT | NOT NULL, DEFAULT 1 | 1正常/2冻结/3注销 |
| `created_at` | DATETIME | NOT NULL | 创建时间 |
| `updated_at` | DATETIME | NOT NULL | 更新时间 |

索引建议：`uk_account(account)`。
