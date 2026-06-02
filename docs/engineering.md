# GCFeed 工程规范

本文定义 GCFeed 的目录职责、代码风格、接口设计、数据模型和测试约定。新增功能时优先遵循本文，再查看对应模块文档。

## 1. 技术栈

| 区域 | 技术 |
| --- | --- |
| API | Go、Gin、GORM |
| 数据库 | MySQL |
| 缓存 | Redis |
| 消息队列 | RabbitMQ |
| 鉴权 | JWT |
| Web | React、Vite |
| 规格驱动 | OpenSpec |

## 2. 后端分层

后端位于 `apps/api`，采用四层结构：

```text
apps/api/
  cmd/feed/main.go
  cmd/worker/main.go
  configs/
  internal/
    domain/{module}/
    application/{module}/
    infra/
    infra/persistence/{module}/
    interfaces/http/{module}/
    interfaces/http/router/router.go
  test/
```

| 层 | 职责 | 依赖方向 |
| --- | --- | --- |
| Domain | 实体、领域错误、业务不变量、仓储接口 | 只依赖标准库 |
| Application | 用例编排、分页游标、幂等、跨实体流程 | 依赖 Domain 接口 |
| Infrastructure | GORM、Redis、RabbitMQ、JWT、配置 | 实现 Domain/Application 所需接口 |
| Interfaces | HTTP Handler、DTO、路由、中间件 | 调用 Application Service |

模块接入顺序：

1. 定义 Domain 实体、错误和仓储接口。
2. 编写 Application Service。
3. 实现 GORM 模型和 Repository。
4. 编写 HTTP DTO 和 Handler。
5. 在 `router.Register` 装配 `Repository -> Service -> Handler -> Route`。
6. 补充 API 流程测试和模块文档。

## 3. 新增后端模块文件组

```text
apps/api/internal/domain/{module}/entity.go
apps/api/internal/domain/{module}/errors.go
apps/api/internal/domain/{module}/repository.go
apps/api/internal/application/{module}/service.go
apps/api/internal/infra/persistence/{module}/model.go
apps/api/internal/infra/persistence/{module}/gorm.go
apps/api/internal/interfaces/http/{module}/dto.go
apps/api/internal/interfaces/http/{module}/handler.go
apps/api/test/{module}_api_test.go
docs/modules/{module}.md
```

当前模块体量继续增长时，按职责拆出 `cursor.go`、`worker.go`、`event.go`、`errors.go` 等文件。

## 4. Go 包和命名

包名使用层级前缀 + 模块名：

```go
package domainvideo
package applicationvideo
package infravideo
package interfaceshttpvideo
```

导入时使用同名别名：

```go
import (
    applicationvideo "GCFeed/internal/application/video"
    domainvideo "GCFeed/internal/domain/video"
)
```

常用类型命名：

| 类型 | 命名 |
| --- | --- |
| 应用服务 | `Service`，构造函数 `New` |
| HTTP 入口 | `Handler`，构造函数 `New` |
| 仓储接口 | `Repository` |
| 仓储实现 | `Repository`，通过包名区分 |
| GORM 模型 | `{Entity}Model` |
| 响应转换 | `{xxx}ResponseFromDomain`、`{xxx}ResponseFromResult` |
| 参数解析 | `parse{Field}`、`parsePositiveInt64`、`parseLimit` |
| 错误写入 | `write{Module}Error` |

常量位置：

| 常量类型 | 位置 |
| --- | --- |
| 领域枚举、最大长度 | `domain/{module}/entity.go` |
| 应用默认值 | `application/{module}/service.go` |
| HTTP query 默认值 | `interfaces/http/{module}/handler.go` |

## 5. Domain 规则

Domain 层负责业务不变量和领域表达。

放在 Domain 的内容：

- 实体结构体。
- 状态常量和业务限制。
- 领域错误。
- 领域构造函数，例如 `NewPublished`、`NewComment`、`NewFollow`。
- 数据恢复函数，例如 `RestoreVideo`、`RestoreUserWithStats`。
- 实体方法，例如 `Authenticate`、`UpdateProfile`、`DeleteBy`、`Active`。
- 仓储接口。

领域构造函数负责清理字符串输入、校验 ID、校验必填字段、校验长度限制、设置默认状态和业务时间。读取路径使用 `Restore*` 保留数据库状态并做展示字段清洗。

## 6. Application 规则

Application 层负责用例编排。

放在 Application 的内容：

- `Service`。
- 用例入参结构，例如 `FeedRequest`。
- 用例返回结构，例如 `LoginResult`、`CreateResult`、`CommentListResult`。
- 跨实体流程，例如发布视频时处理幂等键。
- 游标解析和编码。
- 默认分页大小和最大分页裁剪。
- 基础设施能力的最小接口，例如 `TokenSigner`。

Service 依赖 Domain 的 `Repository` 接口，构造函数注入依赖。Redis、RabbitMQ、JWT 这类能力通过小接口注入，便于测试。

## 7. Infrastructure 规则

Infrastructure 层负责外部资源和技术实现。

主要目录：

```text
internal/infra/config/
internal/infra/database/
internal/infra/cache/
internal/infra/mq/
internal/infra/jwt/
internal/infra/persistence/{module}/
internal/infra/persistence/migration/
```

GORM Repository 规则：

- 每个模块独立 `model.go` 和 `gorm.go`。
- `model.go` 只定义数据库模型和 `TableName`。
- `gorm.go` 实现 Domain Repository 接口。
- 写操作尽量保持事务边界清晰。
- 列表查询使用稳定排序字段和游标。
- 返回 Domain 实体，避免把 GORM 模型泄漏到 Application。

## 8. Interfaces 规则

Interfaces 层负责 HTTP 入口。

Handler 职责：

- 解析 path、query、body 和 header。
- 从鉴权上下文读取用户 ID。
- 调用 Application Service。
- 将结果转换为响应 DTO。
- 将业务错误映射成 HTTP 状态码。

Handler 避免承载业务规则。业务判断放在 Domain 或 Application。

## 9. HTTP API 规范

路径使用资源名，方法表达动作：

| 方法 | 用途 |
| --- | --- |
| `GET` | 查询资源 |
| `POST` | 创建资源或提交复杂查询 |
| `PUT` | 设置确定状态 |
| `PATCH` | 部分更新 |
| `DELETE` | 删除或取消 |

路径约定：

```text
POST   /api/users
POST   /api/sessions
DELETE /api/sessions/current
GET    /api/users/me
PATCH  /api/users/me
POST   /api/videos
GET    /api/videos/{videoId}
GET    /api/feed-items
POST   /api/feed-queries
PUT    /api/videos/{videoId}/like
DELETE /api/videos/{videoId}/like
```

状态码约定：

| 状态码 | 场景 |
| --- | --- |
| `200` | 查询、更新、删除成功 |
| `201` | 创建成功 |
| `400` | 参数格式或业务输入错误 |
| `401` | 登录态缺失或 Token 异常 |
| `403` | 已登录但权限不足 |
| `404` | 资源缺失 |
| `409` | 幂等冲突或唯一性冲突 |
| `500` | 服务内部错误 |

写接口支持 `Idempotency-Key` 时，客户端可传最长 128 字符的幂等键。重复请求返回同一业务结果。

## 10. 分页和游标

列表接口优先使用游标分页。游标内容使用排序字段，编码为 URL-safe 字符串。

常见排序：

| 列表 | 排序 |
| --- | --- |
| Timeline Feed | `published_at DESC, id DESC` |
| 评论列表 | `created_at DESC, id DESC` |
| 关注列表 | `updated_at DESC, target_user_id DESC` |
| 粉丝列表 | `updated_at DESC, user_id DESC` |

返回结构：

```json
{
  "items": [],
  "next_cursor": "",
  "has_more": false
}
```

## 11. 数据库规范

表名使用小写蛇形命名。领域主表使用单数名，例如 `user`、`video`。关系和行为表使用业务事实名，例如 `user_follow`、`interaction_action`。

通用字段：

| 字段 | 说明 |
| --- | --- |
| `id` | BIGINT 主键 |
| `created_at` | 创建时间 |
| `updated_at` | 更新时间 |
| `status` | 软状态字段 |
| `idempotency_key` | 写操作幂等键 |

高频计数独立成统计表，例如 `video_stat`、`user_relation_stat`。计数更新与事实写入放在同一事务中完成。缓存计数允许短暂偏差，持久化表保存最终事实。

## 12. 错误处理

Domain 定义明确业务错误：

```go
var ErrInvalidVideoID = errors.New("invalid video id")
```

Application 可以包装跨资源错误，但保留可判断性：

```go
return nil, fmt.Errorf("%w: %d", domainvideo.ErrVideoNotFound, videoID)
```

HTTP 层使用 `errors.Is` 映射状态码。响应保持简洁：

```json
{"error":"invalid request"}
```

## 13. 前端规范

前端位于 `apps/web`，当前保持轻量结构：

```text
apps/web/src/App.jsx
apps/web/src/main.jsx
apps/web/src/styles.css
```

规则：

- 组件使用函数组件。
- API 调用集中使用 `apiRequest`。
- 服务端错误显示为用户可理解文案。
- 页面状态保持清楚：loading、error、empty、success。
- CSS class 使用语义命名。
- 页面继续增长时拆为 `src/pages/`、`src/components/`、`src/lib/api.js`。

## 14. 测试规范

后端测试位于 `apps/api/test`。新增接口至少覆盖：

- 成功路径。
- 参数错误。
- 鉴权错误。
- 幂等重复请求。
- 游标分页稳定性。
- 关键状态变化和计数变化。

常用命令：

```bash
cd apps/api
go test ./...
```

Web 构建命令：

```bash
cd apps/web
npm run build
```

OpenSpec 校验命令：

```bash
openspec validate --all --strict
```

## 15. 文档同步

改动以下内容时同步文档：

| 改动 | 需要同步 |
| --- | --- |
| 新增接口 | `docs/product.md`、`docs/modules/{module}.md` |
| 新增模块 | `docs/modules/README.md`、模块文档、OpenSpec |
| 改目录或分层 | 本文、`docs/quickread.md` |
| 改核心链路 | `docs/architecture.md` |
| 改前端页面 | `docs/uiux.md` |
| 改性能策略 | `docs/optimization.md` |

新增后端能力检查清单：

- Domain 实体、错误和仓储接口完整。
- Application Service 覆盖核心用例。
- Infrastructure Repository 实现接口。
- Handler 完成参数解析和错误映射。
- Router 完成依赖装配。
- API 测试覆盖成功和失败路径。
- 模块文档和产品状态更新。
