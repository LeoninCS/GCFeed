# 视频 Feed 系统架构图（MVP）

本文按 `mermaid-diagrams` skill 重构：每张图只表达一个概念，节点保持克制，连接线带语义标签，图前给出用途说明。当前实现以 Go API 单体承载账户、视频、Feed 与上传能力，后续能力以演进图呈现。

## 1. 系统上下文

这张图展示 GCFeed 与客户端、存储和演进型基础设施的边界。

```mermaid
---
config:
  theme: base
  layout: dagre
  themeVariables:
    background: transparent
    fontFamily: Inter, PingFang SC, Microsoft YaHei, Arial
    primaryTextColor: "#0F172A"
    lineColor: "#94A3B8"
    clusterBkg: "#F8FAFC"
    clusterBorder: "#CBD5E1"
---
flowchart LR
  %% GCFeed MVP system context
  classDef client fill:#EFF6FF,stroke:#60A5FA,color:#0F172A,stroke-width:1px;
  classDef system fill:#DCFCE7,stroke:#22C55E,color:#0F172A,stroke-width:1px;
  classDef store fill:#F1F5F9,stroke:#64748B,color:#0F172A,stroke-width:1px;
  classDef future fill:#FAE8FF,stroke:#C084FC,color:#0F172A,stroke-width:1px,stroke-dasharray:5 5;

  Web["Web App<br/>React + Vite"]
  Client["移动端 / API 调用方"]
  API["GCFeed API<br/>Go + Gin"]
  MySQL[("MySQL<br/>业务数据")]
  Uploads[("uploads<br/>视频 / 封面 / 头像")]
  Redis[("Redis<br/>缓存与计数")]
  MQ[("MQ<br/>异步事件")]
  ObjectStorage[("对象存储<br/>媒体文件")]

  Web -->|"调用管理与浏览接口"| API
  Client -->|"调用公开 API"| API
  API -->|"读写 account / video / video_stat"| MySQL
  API -->|"保存和读取本地文件"| Uploads
  API -.->|"缓存 Feed 与计数"| Redis
  API -.->|"投递互动和审核事件"| MQ
  API -.->|"迁移媒体文件"| ObjectStorage

  class Web,Client client;
  class API system;
  class MySQL,Uploads store;
  class Redis,MQ,ObjectStorage future;
  linkStyle default stroke:#94A3B8,stroke-width:1.4px
```

## 2. API 内部分层

这张图展示 Go API 单体内部的主要代码层次和依赖方向。

```mermaid
---
config:
  theme: base
  layout: dagre
  themeVariables:
    background: transparent
    fontFamily: Inter, PingFang SC, Microsoft YaHei, Arial
    primaryTextColor: "#0F172A"
    lineColor: "#94A3B8"
    clusterBkg: "#F8FAFC"
    clusterBorder: "#CBD5E1"
---
flowchart LR
  %% Internal layering in apps/api
  classDef entry fill:#F8FAFC,stroke:#94A3B8,color:#0F172A,stroke-width:1px;
  classDef http fill:#DBEAFE,stroke:#3B82F6,color:#0F172A,stroke-width:1px;
  classDef service fill:#DCFCE7,stroke:#22C55E,color:#0F172A,stroke-width:1px;
  classDef domain fill:#CCFBF1,stroke:#14B8A6,color:#0F172A,stroke-width:1px;
  classDef infra fill:#FFEDD5,stroke:#F97316,color:#0F172A,stroke-width:1px;
  classDef store fill:#F1F5F9,stroke:#64748B,color:#0F172A,stroke-width:1px;

  Entry["cmd/feed/main.go<br/>启动装配"]
  Router["Router<br/>路由注册"]
  Auth["JWT Middleware<br/>鉴权上下文"]
  Handlers["HTTP Handlers<br/>account / video / feed / upload"]
  Services["Application Services<br/>account / video / feed"]
  Domains["Domain Models<br/>account / video / feed"]
  Config["Config Loader<br/>configs/config.yaml"]
  JWT["JWT Manager<br/>签发访问令牌"]
  Repo["GORM Repository<br/>仓储实现"]
  SQL["database/sql<br/>MySQL 连接"]
  MySQL[("MySQL")]
  Uploads[("uploads 目录")]

  Entry -->|"加载配置"| Config
  Entry -->|"创建数据库连接"| SQL
  Entry -->|"注册 HTTP 路由"| Router
  Router -->|"校验受保护接口"| Auth
  Auth -->|"解析和签名 Token"| JWT
  Router -->|"分发业务请求"| Handlers
  Handlers -->|"调用用例服务"| Services
  Services -->|"执行领域规则"| Domains
  Services -->|"读写仓储"| Repo
  Repo -->|"复用连接池"| SQL
  SQL -->|"持久化数据"| MySQL
  Handlers -->|"保存上传文件"| Uploads
  Router -->|"暴露静态文件"| Uploads

  class Entry entry;
  class Router,Auth,Handlers http;
  class Services service;
  class Domains domain;
  class Config,JWT,Repo,SQL infra;
  class MySQL,Uploads store;
  linkStyle default stroke:#94A3B8,stroke-width:1.4px
```

## 3. 核心请求链路

这张图展示从注册、登录、上传、发布到刷 Feed 的 MVP 主链路。

```mermaid
---
config:
  theme: base
  themeVariables:
    background: transparent
    fontFamily: Inter, PingFang SC, Microsoft YaHei, Arial
    primaryTextColor: "#0F172A"
    actorBkg: "#EFF6FF"
    actorBorder: "#60A5FA"
    actorTextColor: "#0F172A"
    activationBkgColor: "#DCFCE7"
    activationBorderColor: "#22C55E"
    sequenceNumberColor: "#475569"
    lineColor: "#94A3B8"
---
sequenceDiagram
  autonumber
  participant C as Client
  participant R as Gin Router
  participant H as Handler
  participant S as Service
  participant Repo as GORM Repository
  participant DB as MySQL
  participant FS as uploads

  C->>R: POST /api/auth/register
  R->>H: Account.Register
  H->>S: 创建账户
  S->>Repo: Save account
  Repo->>DB: INSERT account
  DB-->>Repo: 返回 user id
  Repo-->>S: 返回用户实体
  S-->>H: 返回 profile
  H-->>C: 201 Created

  C->>R: POST /api/auth/login/password
  R->>H: Account.Login
  H->>S: 校验账号密码
  S->>Repo: FindByAccount
  Repo->>DB: SELECT account
  DB-->>Repo: 返回用户记录
  S-->>H: 返回 Bearer JWT
  H-->>C: 返回 access_token

  C->>R: POST /api/uploads
  R->>H: Upload.Create
  H->>FS: 写入 video / cover / avatar
  FS-->>H: 返回文件路径
  H-->>C: 返回 /uploads/{kind}/{filename}

  C->>R: POST /api/videos
  R->>H: Video.Create
  H->>S: CreatePublished
  S->>Repo: Save video and video_stat
  Repo->>DB: INSERT video, video_stat
  DB-->>Repo: 返回 video id
  Repo-->>S: 返回视频实体
  S-->>H: 返回视频详情
  H-->>C: 201 Created

  C->>R: GET /api/feed/timeline?cursor=...&limit=...
  R->>H: Feed.Timeline
  H->>S: GetTimelineFeed
  S->>Repo: ListTimelineFeed
  Repo->>DB: SELECT video LEFT JOIN account LEFT JOIN video_stat
  DB-->>Repo: 返回 Feed 行
  Repo-->>S: 返回 FeedItem 列表
  S-->>H: 返回 items + next_cursor + has_more
  H-->>C: 返回 Timeline Feed
```

## 4. 数据模型

这张图展示当前 GORM 自动迁移的三张核心表及 Feed 查询依赖。

```mermaid
---
config:
  theme: base
  themeVariables:
    background: transparent
    fontFamily: Inter, PingFang SC, Microsoft YaHei, Arial
    primaryTextColor: "#0F172A"
    lineColor: "#94A3B8"
---
erDiagram
  ACCOUNT ||--o{ VIDEO : "发布视频"
  VIDEO ||--|| VIDEO_STAT : "拥有计数"

  ACCOUNT {
    bigint id PK
    string account UK
    string password
    string nickname
    string avatar_url
    string role
    int status
  }

  VIDEO {
    bigint id PK
    bigint author_id FK
    string title
    string media_url
    string cover_url
    int status
    datetime published_at
    string idempotency_key
  }

  VIDEO_STAT {
    bigint video_id PK
    int like_count
    int comment_count
    int favorite_count
  }
```

## 5. 演进能力地图

这张图展示已落地闭环和后续模块的连接方式，虚线代表规划中的能力边界。

```mermaid
---
config:
  theme: base
  layout: dagre
  themeVariables:
    background: transparent
    fontFamily: Inter, PingFang SC, Microsoft YaHei, Arial
    primaryTextColor: "#0F172A"
    lineColor: "#94A3B8"
    clusterBkg: "#F8FAFC"
    clusterBorder: "#CBD5E1"
---
flowchart TB
  %% Roadmap map from current MVP to planned modules
  classDef current fill:#DCFCE7,stroke:#22C55E,color:#0F172A,stroke-width:1px;
  classDef growth fill:#FAE8FF,stroke:#C084FC,color:#0F172A,stroke-width:1px,stroke-dasharray:5 5;
  classDef platform fill:#FFEDD5,stroke:#F97316,color:#0F172A,stroke-width:1px,stroke-dasharray:5 5;
  classDef data fill:#F1F5F9,stroke:#64748B,color:#0F172A,stroke-width:1px,stroke-dasharray:5 5;

  Account["账户"]
  Video["视频"]
  Feed["Feed"]
  Upload["上传"]
  Recommendation["推荐"]
  Interaction["互动"]
  Review["审核"]
  Message["消息"]
  Admin["后台运营"]
  Governance["系统治理"]
  Observability["监控告警"]
  AsyncStore[("Redis / MQ / 对象存储")]

  Account -->|"扩展关注关系"| Interaction
  Video -->|"进入内容审核"| Review
  Video -->|"承载点赞评论收藏"| Interaction
  Upload -->|"迁移媒体文件"| AsyncStore
  Feed -->|"接入召回排序"| Recommendation
  Recommendation -->|"返回候选内容"| Feed
  Interaction -->|"投递互动事件"| AsyncStore
  AsyncStore -->|"生成站内通知"| Message
  Admin -->|"处理审核任务"| Review
  Governance -->|"保护核心接口"| Feed
  Observability -->|"采集服务指标"| Governance

  class Account,Video,Feed,Upload current;
  class Recommendation,Interaction,Review,Message,Admin growth;
  class Governance,Observability platform;
  class AsyncStore data;
  linkStyle default stroke:#94A3B8,stroke-width:1.4px
```

## 6. 说明

- 当前代码以 Go API 单体承载账户、视频、Feed 与上传能力，内部按接口层、应用层、领域层、基础设施层组织。
- 对外接口统一挂载在 `/api/*`，静态文件通过 `/uploads/*` 访问，健康检查使用 `/health`。
- 数据持久化使用 MySQL，GORM 自动迁移 `account`、`video`、`video_stat` 三张表。
- Feed 当前采用 Timeline 策略，按 `published_at DESC, id DESC` 排序，并通过 Base64 游标分页。
- 推荐、互动、审核、消息、治理和监控模块作为演进边界保留，后续可从单体内模块逐步扩展为异步事件和独立服务。
