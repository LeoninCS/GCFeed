# GCFeed 软件设计与代码规范

本文定义 GCFeed 的项目结构、代码风格、接口设计和测试约定。目标是让开发者和 AI 生成代码时都能保持同一套工程风格，新增模块时优先沿用本文规范。

## 1. 项目定位

GCFeed 是一个短视频 Feed 系统，当前采用 Go API 单体 + React Web 客户端。

后端采用分层架构：

- `Domain`：领域实体、领域常量、领域错误、仓储接口。
- `Application`：用例编排、跨实体流程、分页游标编码、幂等重放逻辑。
- `Infrastructure`：数据库、JWT、配置、GORM 模型与仓储实现。
- `Interfaces`：HTTP Handler、DTO、路由、中间件和上传入口。

前端采用 React + Vite，单页应用内通过轻量路由函数切换页面，并通过 `apiRequest` 访问后端 REST API。

## 2. 目录职责

### 2.1 后端目录

```text
apps/api/
  cmd/feed/main.go                         # 进程入口，只负责启动装配
  configs/config.yaml                      # 本地配置
  internal/
    domain/{module}/                       # 领域模型、错误、仓储接口
    application/{module}/                  # 应用服务和用例结果对象
    infra/{config,database,httpgin,jwt}/   # 基础设施能力
    infra/persistence/{module}/            # GORM 模型和仓储实现
    interfaces/http/{module}/              # HTTP Handler 和 DTO
    interfaces/http/router/router.go       # 路由和依赖装配
    interfaces/http/middleware/            # HTTP 中间件
  test/                                    # API 流程测试
```

新增业务模块时按以下文件组创建：

```text
internal/domain/{module}/entity.go
internal/domain/{module}/errors.go
internal/domain/{module}/repository.go
internal/application/{module}/service.go
internal/infra/persistence/{module}/model.go
internal/infra/persistence/{module}/gorm.go
internal/interfaces/http/{module}/dto.go
internal/interfaces/http/{module}/handler.go
```

模块接入顺序：

1. 先定义领域对象、领域错误和仓储接口。
2. 再写应用服务，应用服务依赖领域仓储接口。
3. 再写 GORM 模型和仓储实现。
4. 再写 HTTP DTO 与 Handler。
5. 最后在 `router.Register` 中装配 `Repository -> Service -> Handler -> Route`。

### 2.2 文档目录

```text
docs/product.md                 # 产品范围、模块、接口优先级
docs/architecture.md            # 系统架构图
docs/uiux.md                    # UI/UX 设计规范
docs/modules/*.md               # 单模块设计文档
docs/sdd.md                     # 工程设计和代码规范
```

接口、领域名、模块边界调整时，同步更新对应模块文档和本文。

### 2.3 前端目录

```text
apps/web/
  src/App.jsx       # 页面、状态、API 调用和组件
  src/main.jsx      # React 挂载入口
  src/styles.css    # 全局样式、布局、组件样式
```

当前前端保持轻量结构。页面数量继续增长时，将 `App.jsx` 拆分为 `src/pages/`、`src/components/`、`src/lib/api.js`，拆分后的命名仍沿用当前函数组件和 CSS class 风格。

## 3. Go 包与命名规范

### 3.1 包名

后端包名使用层级前缀 + 模块名，沿用现有风格：

```go
package domainvideo
package applicationvideo
package infravideo
package interfaceshttpvideo
```

导入时使用同名别名，保持调用处能看出分层来源：

```go
import (
    applicationvideo "GCFeed/internal/application/video"
    domainvideo "GCFeed/internal/domain/video"
    interfaceshttpmiddleware "GCFeed/internal/interfaces/http/middleware"
)
```

新增模块示例：

```go
domainmessage "GCFeed/internal/domain/message"
applicationmessage "GCFeed/internal/application/message"
inframessage "GCFeed/internal/infra/persistence/message"
interfaceshttpmessage "GCFeed/internal/interfaces/http/message"
```

### 3.2 命名

- 领域实体使用业务名词：`User`、`Video`、`FeedItem`、`Comment`。
- 应用服务统一命名为 `Service`，构造函数统一命名为 `New`。
- HTTP 入口统一命名为 `Handler`，构造函数统一命名为 `New`。
- 仓储接口统一命名为 `Repository`，仓储实现也命名为 `Repository`，通过包名区分。
- GORM 模型使用 `{Entity}Model`，例如 `VideoModel`、`CommentModel`。
- 响应转换函数使用 `{xxx}ResponseFromDomain`、`{xxx}ResponseFromResult`。
- 参数解析函数使用 `parse{Field}`、`parsePositiveInt64`、`parseLimit`。
- 错误写入函数使用 `write{Module}Error`。
- 错误判断函数使用 `isBadRequestError`，放在对应 Handler 文件底部。

### 3.3 常量

领域枚举和限制放在 `domain/{module}/entity.go`：

```go
const (
    StatusPublished = 2
    MaxTitleLength = 128
)
```

应用层默认值放在 `application/{module}/service.go`：

```go
const defaultCommentLimit = 20
```

HTTP 层默认 query 参数放在 Handler 文件顶部：

```go
const defaultListLimit = 20
```

## 4. 后端分层规则

### 4.1 Domain 层

Domain 层负责业务不变量和领域表达。

放在 Domain 的内容：

- 实体结构体。
- 领域状态常量。
- 最大长度、最大页大小等业务限制。
- 领域错误。
- 领域构造函数，例如 `NewPublished`、`NewComment`、`NewFollow`。
- 数据恢复函数，例如 `RestoreVideo`、`RestoreUserWithStats`。
- 实体方法，例如 `Authenticate`、`UpdateProfile`、`DeleteBy`、`Active`。
- 仓储接口。

领域构造函数负责：

- 清理字符串输入：`strings.TrimSpace`。
- 校验 ID 为正数。
- 校验必填字段。
- 校验长度限制。
- 设置默认状态。
- 创建时间类业务值。

示例：

```go
func NewComment(videoID int64, userID int64, content string, idempotencyKey string) (*Comment, error) {
    if videoID <= 0 {
        return nil, ErrInvalidVideoID
    }
    if userID <= 0 {
        return nil, ErrInvalidUserID
    }

    content = strings.TrimSpace(content)
    idempotencyKey = strings.TrimSpace(idempotencyKey)
    if content == "" {
        return nil, ErrEmptyCommentContent
    }

    return &Comment{
        VideoID:        videoID,
        UserID:         userID,
        Content:        content,
        Status:         CommentStatusNormal,
        IdempotencyKey: idempotencyKey,
    }, nil
}
```

`Restore*` 函数用于数据库读取路径，读取路径保留数据库状态并做展示字段清洗。

### 4.2 Application 层

Application 层负责用例编排。

放在 Application 的内容：

- `Service`。
- 用例入参结构，例如 `FeedRequest`。
- 用例返回结构，例如 `LoginResult`、`CreateResult`、`CommentListResult`。
- 跨实体流程，例如发布视频时处理幂等键。
- 游标解析和编码。
- 默认分页大小和最大分页裁剪。
- 基础设施能力的最小接口，例如账号服务中的 `TokenSigner`。

Application 层依赖 Domain 的 `Repository` 接口，构造函数注入依赖：

```go
type Service struct {
    repo domainvideo.Repository
}

func New(repo domainvideo.Repository) *Service {
    return &Service{repo: repo}
}
```

应用层错误使用面向用例的错误名：

```go
var ErrLoadVideoFailed = errors.New("failed to load video")
var ErrSaveVideoFailed = errors.New("failed to save video")
var ErrUpdateVideoFailed = errors.New("failed to update video")
```

应用层处理仓储错误时，把存储细节收敛成领域错误或用例错误：

```go
video, err := s.repo.FindByID(ctx, videoID)
if err != nil {
    if errors.Is(err, domainvideo.ErrVideoNotFound) {
        return nil, domainvideo.ErrVideoNotFound
    }
    return nil, ErrLoadVideoFailed
}
```

### 4.3 Infrastructure 层

Infrastructure 层负责技术实现。

放在 Infrastructure 的内容：

- GORM 模型。
- 数据库表名和索引标签。
- GORM 仓储实现。
- 事务。
- 行锁。
- 唯一键冲突识别。
- 数据库模型到领域对象的转换。
- JWT、配置、数据库连接和 Gin 初始化。

GORM 模型规范：

```go
type VideoModel struct {
    ID        int64     `gorm:"column:id;primaryKey;autoIncrement"`
    AuthorID  int64     `gorm:"column:author_id;not null;index"`
    CreatedAt time.Time `gorm:"column:created_at;autoCreateTime"`
    UpdatedAt time.Time `gorm:"column:updated_at;autoUpdateTime"`
}

func (VideoModel) TableName() string {
    return "video"
}
```

仓储实现规范：

- 所有数据库调用使用 `r.db.WithContext(ctx)`。
- 写入多个表时使用 `Transaction`。
- 计数更新前锁定统计行：`clause.Locking{Strength: "UPDATE"}`。
- 查询列表时使用稳定排序字段，通常为业务时间倒序 + ID 倒序。
- 数据库查询结果通过 `restore*` 函数转换为领域对象。
- 仓储文件底部声明接口实现校验：

```go
var _ domaininteraction.Repository = (*Repository)(nil)
```

唯一键冲突处理沿用 MySQL 1062 和 GORM 标准错误兼容写法：

```go
func isDuplicateKeyError(err error) bool {
    if errors.Is(err, gorm.ErrDuplicatedKey) {
        return true
    }
    var mysqlErr *mysql.MySQLError
    return errors.As(err, &mysqlErr) && mysqlErr.Number == 1062
}
```

### 4.4 Interfaces 层

Interfaces 层负责 HTTP 输入输出。

Handler 职责：

- 读取路径参数、query 参数、请求体和请求头。
- 从 JWT 上下文读取当前用户。
- 调用应用服务。
- 把应用层结果转换成 HTTP 响应 DTO。
- 把领域错误和应用错误映射成 HTTP 状态码。

Handler 代码顺序：

1. `type Handler struct`。
2. `New` 构造函数。
3. 对外 Handler 方法。
4. 内部复用方法。
5. 参数解析函数。
6. 响应转换函数。
7. 错误映射函数。

DTO 文件职责：

- 请求体结构。
- 响应结构。
- JSON 字段名。

DTO 命名规范：

```go
type CreateVideoRequest struct {
    Title       string `json:"title"`
    Description string `json:"description"`
}

type videoResponse struct {
    ID        int64  `json:"id"`
    AuthorID  int64  `json:"author_id"`
}
```

对外请求体需要跨包复用时使用导出类型，例如 `CreateVideoRequest`。模块内部响应结构使用小写类型，例如 `videoResponse`。

## 5. HTTP API 设计规范

### 5.1 路径与方法

API 使用资源名复数形式，路径表达资源，HTTP 方法表达动作。

| 场景 | 方法 | 路径示例 | 语义 |
| --- | --- | --- | --- |
| 创建资源 | `POST` | `/api/users` | 注册用户 |
| 创建会话 | `POST` | `/api/sessions` | 登录 |
| 删除当前会话 | `DELETE` | `/api/sessions/current` | 退出登录 |
| 读取单个资源 | `GET` | `/api/videos/{videoId}` | 视频详情 |
| 更新当前用户 | `PATCH` | `/api/users/me` | 部分更新 |
| 设置关系状态 | `PUT` | `/api/videos/{videoId}/like` | 点赞生效 |
| 取消关系状态 | `DELETE` | `/api/videos/{videoId}/like` | 取消点赞 |
| 子资源列表 | `GET` | `/api/videos/{videoId}/comments` | 评论列表 |
| 复杂查询 | `POST` | `/api/feed-queries` | Feed 查询体扩展 |

路径参数使用小驼峰：`:videoId`、`:userId`、`:targetUserId`。

URL 使用 kebab-case 资源名：

```text
/api/feed-items
/api/feed-queries
/api/video-view-events
```

JSON 字段使用 snake_case：

```json
{
  "access_token": "...",
  "token_type": "Bearer",
  "expires_in_seconds": 900
}
```

### 5.2 状态码

| 状态码 | 使用场景 |
| --- | --- |
| `200 OK` | 读取成功、状态切换成功、幂等重放返回已有资源 |
| `201 Created` | 新资源创建成功 |
| `204 No Content` | 删除成功且响应体为空 |
| `400 Bad Request` | 参数格式、必填字段、长度、游标等请求错误 |
| `401 Unauthorized` | 登录态缺失或 access token 失效 |
| `403 Forbidden` | 已登录用户缺少资源操作权限 |
| `404 Not Found` | 资源不存在或公开视图不可见 |
| `409 Conflict` | 唯一资源冲突，例如账号重复 |
| `500 Internal Server Error` | 服务端内部错误 |

错误响应统一使用：

```json
{"error":"invalid request"}
```

JWT 中间件现有部分响应使用 `message` 字段，新增业务 Handler 使用 `error` 字段。

### 5.3 鉴权

受保护接口使用 `authMiddleware`。

公共接口需要个性化能力时使用 `optionalAuthMiddleware`，例如 Feed。可选鉴权解析成功时写入 `auth_user_id` 和 `auth_role`，解析失败时按匿名请求继续处理。

Handler 中读取登录用户：

```go
func userIDFromContext(c *gin.Context) (int64, bool) {
    value, exists := c.Get(interfaceshttpmiddleware.ContextUserIDKey)
    if !exists {
        return 0, false
    }
    userID, ok := value.(int64)
    return userID, ok && userID > 0
}
```

### 5.4 幂等设计

客户端可能重试的写接口支持 `Idempotency-Key` 请求头：

- `POST /api/videos`
- `PUT /api/videos/{videoId}/like`
- `DELETE /api/videos/{videoId}/like`
- `PUT /api/videos/{videoId}/favorite`
- `DELETE /api/videos/{videoId}/favorite`
- `POST /api/videos/{videoId}/comments`
- `PUT /api/users/me/following/{targetUserId}`
- `DELETE /api/users/me/following/{targetUserId}`

幂等键规范：

- Header 名固定为 `Idempotency-Key`。
- 最大长度放在领域层：`MaxIdempotencyKeyLength = 128`。
- 应用层先 trim，再校验长度。
- 创建类接口命中幂等键时返回原资源。
- 状态类接口重复请求时返回当前状态和当前计数。
- 空幂等键存储为 `NULL`，允许普通请求多次执行。

### 5.5 分页设计

Feed、评论、关注列表使用游标分页。

游标分页规范：

- 响应字段固定为 `items`、`next_cursor`、`has_more`。
- 请求参数使用 `cursor` 和 `limit`。
- Handler 校验显式传入的 `limit` 必须为正数。
- 默认页大小在应用层处理。
- 最大页大小在领域层定义，当前最大为 `100`。
- 查询多取一条：`limit + 1`，用于判断 `has_more`。
- 游标内容使用 JSON + `base64.RawURLEncoding` 编码。
- 游标时间使用 `time.RFC3339Nano` 和 UTC 格式。
- 排序字段和游标条件保持一致。

时间线分页示例：

```go
Order("v.published_at DESC").
Order("v.id DESC")
```

对应游标条件：

```go
Where(
    "(v.published_at < ? OR (v.published_at = ? AND v.id < ?))",
    cursor.PublishedAt,
    cursor.PublishedAt,
    cursor.VideoID,
)
```

普通作品列表当前使用 offset 分页，响应回显 `limit` 和 `offset`。

### 5.6 Feed scene 设计

Feed 使用 scene 策略注册表扩展：

```go
type Strategy interface {
    Scene() domainfeed.Scene
    List(ctx context.Context, req FeedRequest) (*FeedResult, error)
}
```

新增 Feed 场景步骤：

1. 在 `domain/feed/entity.go` 增加 scene 常量。
2. 实现新的 `Strategy`。
3. 在 `application/feed.New` 或装配层注册策略。
4. 为 scene 查询添加 API 流程测试。

scene 参数统一由 `domainfeed.NormalizeScene` 清洗。

### 5.7 上传接口

上传使用 multipart form：

- 文件字段名：`file`
- 分类字段名：`kind`
- 支持分类：`video`、`cover`、`avatar`、`file`
- 大小限制：`1024 << 20`
- 返回字段：`url`、`kind`、`filename`、`size`

文件保存在 `./uploads/{kind}`，通过 `/uploads/...` 静态路径访问。

## 6. 数据库与持久化规范

### 6.1 表名

表名使用单数或业务组合名，当前已有：

- `account`
- `video`
- `video_stat`
- `interaction_action`
- `interaction_comment`
- `user_follow`
- `user_relation_stat`

所有 GORM 模型定义 `TableName()`，保持表名显式。

### 6.2 计数表

互动计数放在独立统计表：

- 视频计数：`video_stat`
- 关系计数：`user_relation_stat`

计数更新规范：

- 状态记录和计数记录放在同一事务。
- 先锁定业务资源，再锁定行为记录或统计记录。
- 重复 PUT/DELETE 保持计数稳定。
- 计数通过 `clampCount` 保底为 0。

### 6.3 软删除

需要审计或幂等的业务使用状态字段：

- 视频删除：`StatusDeleted`
- 评论删除：`CommentStatusDeleted`
- 关注取消：`FollowStatusCanceled`
- 点赞/收藏取消：`ActionStatusCanceled`

公开查询过滤可见状态：

```go
Where("v.status = ?", domainvideo.StatusPublished)
Where("c.status = ?", domaininteraction.CommentStatusNormal)
```

### 6.4 查询模型

联表查询使用局部查询模型承接结果：

```go
type videoWithStatModel struct {
    ID            int64
    AuthorID      int64
    LikeCount     int
    PublishedAt   *time.Time
}
```

复杂 `SELECT` 字段列表封装为函数：

```go
func videoWithStatSelect() string {
    return "v.id, v.author_id, ..."
}
```

## 7. 错误处理规范

### 7.1 领域错误

领域错误放在 `domain/{module}/errors.go`：

```go
var ErrInvalidVideoID = errors.New("video id must be positive")
var ErrVideoNotFound = errors.New("video not found")
```

错误消息使用英文小写句子，适合直接返回客户端的参数错误保持清晰。

### 7.2 应用错误

应用错误表达用例失败：

```go
var ErrLoadFeedFailed = errors.New("failed to load feed")
```

HTTP 层对应用错误通常返回 `500`，对领域错误按业务映射。

### 7.3 HTTP 错误映射

每个 Handler 维护本模块的错误映射：

```go
func writeVideoError(c *gin.Context, err error) {
    if isBadRequestError(err) {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    if errors.Is(err, domainvideo.ErrVideoNotFound) {
        c.JSON(http.StatusNotFound, gin.H{"error": "video not found"})
        return
    }
    c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
}
```

使用 `errors.Is` 和 `errors.As` 判断错误，保持错误包装后仍可识别。

## 8. 注释与代码风格

Go 代码使用 `gofmt`。

注释风格：

- 导出类型、导出函数、关键私有函数写简短中文注释。
- 注释解释业务意图、边界和并发原因。
- 普通字段和显而易见赋值保持代码自解释。
- 事务、锁、幂等、游标、权限判断处写注释。

示例：

```go
// limit+1 是常见分页技巧：多取一条即可判断后面还有没有数据。
hasMore := len(items) > limit
```

函数结构：

- 参数校验放在开头。
- 字符串清洗紧跟参数校验。
- 早返回处理错误。
- 主流程保持从上到下线性阅读。
- 响应转换放在独立函数。

字符串处理统一使用 `strings.TrimSpace`。行为枚举需要容错时提供 `Normalize*` 函数。

时间格式：

- 游标时间使用 `time.RFC3339Nano`。
- 游标编码前转 UTC。
- 数据库时间交给 GORM `autoCreateTime` 和 `autoUpdateTime`。

## 9. 前端代码规范

### 9.1 React 组件

当前前端使用函数组件和 Hooks：

- 组件名使用 PascalCase。
- 状态变量使用业务名：`items`、`index`、`feedState`、`commentsOpen`。
- 异步状态使用明确字符串：`idle`、`loading`、`ready`、`error`。
- 事件处理函数使用 `handle` 前缀。
- API 调用函数使用动词短语：`loadFeed`、`setLike`、`loadFollowingMap`。

复杂逻辑拆成纯函数：

```jsx
const current = items[index];
const trackStyle = getFeedTrackStyle(swipe);
```

浏览器存储 key 统一定义在文件顶部：

```jsx
const TOKEN_KEY = "gcfeed.accessToken";
const USER_KEY = "gcfeed.user";
```

### 9.2 API 调用

前端请求遵守后端接口契约：

- 登录 token 放在 `Authorization: Bearer {token}`。
- JSON 请求设置 `Content-Type: application/json`。
- 写接口需要重试保护时附加 `Idempotency-Key`。
- 后端错误优先读取 `error`，兼容读取 `message`。
- 401 时清理本地登录态并跳转登录页。

### 9.3 UI 与 CSS

样式集中在 `styles.css`，使用 class 命名表达组件结构：

```css
.top-nav {}
.feed-stage {}
.profile-form {}
```

CSS 规范：

- 全局 design token 放在 `:root`。
- 颜色、尺寸、阴影优先使用 CSS 变量。
- 布局使用 grid/flex。
- 交互状态使用 `:hover`、`:active`、`:disabled`。
- 移动端适配使用 media query。
- 图标使用 Material Symbols，类名沿用 `material-symbols-outlined`。

页面体验保持短视频产品风格：深色界面、沉浸式 Feed、固定顶栏、左侧导航、视频主体优先。

## 10. 测试规范

后端测试放在 `apps/api/test`，以 API 流程测试为主。

测试结构：

- 每个模块一个或多个 `{module}_api_test.go`。
- 使用 `gin.New()` 只装配当前模块需要的路由。
- 使用内存仓储实现领域 `Repository` 接口。
- 测试路由路径与正式路由保持一致。
- 使用 `httptest` 发起 HTTP 请求。
- 使用响应结构体解码 JSON。
- 使用 `requireStatus`、`decodeJSON` 等共享测试工具。

新增接口至少覆盖：

1. 成功主流程。
2. 参数错误。
3. 鉴权错误。
4. 资源不存在。
5. 权限错误。
6. 幂等重放。
7. 分页第一页和下一页。

按接口特性选择对应测试项。Feed、评论、关注列表必须覆盖游标分页。点赞、收藏、关注、评论创建必须覆盖幂等。

运行后端测试：

```bash
cd apps/api
go test ./...
```

运行前端构建：

```bash
cd apps/web
npm run build
```

本地同时启动后端和前端：

```bash
./scripts/start.sh
```

## 11. 路由装配规范

`router.Register` 是后端唯一装配入口。

装配顺序：

1. 创建 GORM 连接。
2. AutoMigrate 当前模型。
3. 执行兼容性修复函数，例如 `infravideo.EnsureStats`。
4. 创建基础设施组件，例如 JWT Manager。
5. 按模块创建 Repository。
6. 按模块创建 Service。
7. 按模块创建 Handler。
8. 创建鉴权中间件。
9. 注册 RESTful 路由。

模块路由分组示例：

```go
videos := api.Group("/videos")
videos.POST("", authMiddleware, videoHandler.Create)
videos.GET("/:videoId", videoHandler.Get)
videos.DELETE("/:videoId", authMiddleware, videoHandler.Delete)
videos.PUT("/:videoId/like", authMiddleware, interactionHandler.Like)
```

路由注释说明资源语义和父子资源关系。

## 12. 新增模块检查清单

新增模块时按这份清单完成：

- 明确模块职责，更新 `docs/modules/{module}.md`。
- 创建 Domain 实体、错误和 Repository 接口。
- 领域构造函数完成清洗和校验。
- Application Service 只依赖领域 Repository 接口。
- Infrastructure 仓储实现使用 GORM、事务和 restore 函数。
- HTTP DTO 使用 snake_case JSON 字段。
- Handler 完成参数解析、鉴权读取、服务调用、响应转换和错误映射。
- 在 `router.Register` 中装配依赖和路由。
- 为 API 成功流、错误流、幂等和分页补测试。
- 运行 `go test ./...`。
- 涉及 Web 时运行 `npm run build`。

## 13. AI 生成代码提示词规范

让 AI 修改本项目时，提示词应包含以下约束：

```text
请按照 docs/sdd.md 扩展 GCFeed。
后端保持 Domain/Application/Infrastructure/Interfaces 分层。
业务规则放在 domain 和 application，HTTP Handler 只做参数解析、鉴权上下文读取、服务调用、响应转换和错误映射。
新增接口使用 RESTful 资源路径，JSON 字段使用 snake_case。
写接口按需要支持 Idempotency-Key。
列表接口优先使用 cursor + limit，响应 items/next_cursor/has_more。
GORM 仓储使用 WithContext(ctx)，多表写入使用事务，计数更新使用行锁。
新增模块补齐 API 流程测试，测试路由与正式路由保持一致。
完成后运行 go test ./...；涉及 Web 时运行 npm run build。
```

代码生成完成后的人工检查重点：

- 分层依赖方向。
- 错误映射状态码。
- 幂等和计数稳定性。
- 游标排序条件和 SQL 排序字段一致性。
- JSON 字段命名。
- 路由路径和 README 接口表一致性。
- 测试覆盖成功流和主要失败流。

## 14. 当前接口约定速查

### 14.1 账户

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| `POST` | `/api/users` | 公共 | 注册 |
| `POST` | `/api/sessions` | 公共 | 登录 |
| `DELETE` | `/api/sessions/current` | 必需 | 退出登录 |
| `GET` | `/api/users/me` | 必需 | 当前用户资料 |
| `PATCH` | `/api/users/me` | 必需 | 更新当前用户资料 |
| `GET` | `/api/users/{userId}` | 公共 | 公开用户资料 |

### 14.2 视频与上传

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| `POST` | `/api/uploads` | 必需 | 上传文件 |
| `POST` | `/api/videos` | 必需 | 发布视频 |
| `GET` | `/api/videos/{videoId}` | 公共 | 视频详情 |
| `DELETE` | `/api/videos/{videoId}` | 必需 | 删除视频 |
| `GET` | `/api/users/me/videos` | 必需 | 我的作品 |
| `GET` | `/api/users/{userId}/videos` | 公共 | 用户公开视频 |

### 14.3 Feed

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| `GET` | `/api/feed-items` | 可选 | Feed 游标分页 |
| `POST` | `/api/feed-queries` | 可选 | 复杂 Feed 查询 |

### 14.4 互动

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| `PUT` | `/api/videos/{videoId}/like` | 必需 | 点赞 |
| `DELETE` | `/api/videos/{videoId}/like` | 必需 | 取消点赞 |
| `PUT` | `/api/videos/{videoId}/favorite` | 必需 | 收藏 |
| `DELETE` | `/api/videos/{videoId}/favorite` | 必需 | 取消收藏 |
| `POST` | `/api/videos/{videoId}/comments` | 必需 | 创建评论 |
| `GET` | `/api/videos/{videoId}/comments` | 公共 | 评论列表 |
| `DELETE` | `/api/comments/{commentId}` | 必需 | 删除评论 |

### 14.5 关系

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| `PUT` | `/api/users/me/following/{targetUserId}` | 必需 | 关注 |
| `DELETE` | `/api/users/me/following/{targetUserId}` | 必需 | 取关 |
| `GET` | `/api/users/me/following` | 必需 | 关注列表 |
| `GET` | `/api/users/me/followers` | 必需 | 粉丝列表 |

新增接口先放入对应模块文档和 README 接口表，再按本文规范落地实现和测试。
