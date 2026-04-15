# 视频 Feed 系统架构图（MVP）

## 1. 总体分层架构

```mermaid
flowchart TB
  %% ===== 样式 =====
  classDef client fill:#E8F4FD,stroke:#1D4ED8,color:#0F172A,stroke-width:1px;
  classDef gateway fill:#DBEAFE,stroke:#2563EB,color:#0F172A,stroke-width:1px;
  classDef app fill:#ECFDF3,stroke:#16A34A,color:#0F172A,stroke-width:1px;
  classDef support fill:#FEF3C7,stroke:#D97706,color:#0F172A,stroke-width:1px;
  classDef data fill:#F1F5F9,stroke:#475569,color:#0F172A,stroke-width:1px;
  classDef infra fill:#FCE7F3,stroke:#BE185D,color:#0F172A,stroke-width:1px;

  %% ===== 客户端层 =====
  subgraph L1[客户端层]
    App[移动端 App]
    Web[管理后台 Web]
  end

  %% ===== 接入层 =====
  subgraph L2[接入层]
    Gateway[API Gateway<br/>/api/* 对外<br/>/internal/* 仅内网]
    Auth[Auth/JWT 中间件]
  end

  %% ===== 核心业务层 =====
  subgraph L3[核心业务服务]
    Account[账户服务]
    Relation[关系服务]
    Video[视频服务]
    Feed[Feed 服务]
    Reco[推荐服务]
    Interaction[互动服务]
    Message[消息服务]
    Review[审核服务]
    Admin[后台运营服务]
  end

  %% ===== 支撑能力层 =====
  subgraph L4[支撑服务]
    Playback[播放优化服务]
    Governance[系统治理服务<br/>限流/降级/重试]
    Monitoring[监控告警服务]
  end

  %% ===== 数据与基础设施层 =====
  subgraph L5[数据与基础设施]
    MySQL[(MySQL<br/>业务主数据)]
    Redis[(Redis<br/>缓存/计数/游标)]
    MQ[(MQ/Kafka<br/>异步事件)]
    Obj[(对象存储<br/>视频/封面)]
    Prom[(Prometheus/Grafana)]
    K8s[(Docker + Kubernetes)]
  end

  %% ===== 入口链路 =====
  App --> Gateway
  Web --> Gateway
  Gateway --> Auth

  %% ===== 网关到业务 =====
  Auth --> Account
  Auth --> Relation
  Auth --> Video
  Auth --> Feed
  Auth --> Interaction
  Auth --> Message
  Auth --> Review
  Auth --> Admin
  Auth --> Playback
  Auth --> Governance
  Auth --> Monitoring

  %% ===== 核心调用关系 =====
  Feed --> Reco
  Feed --> Video
  Reco --> Video

  Interaction --> Message
  Interaction --> Video

  Video --> Review
  Review --> Video
  Admin --> Review
  Admin --> Video
  Admin --> Reco
  Admin --> Governance
  Admin --> Monitoring

  Gateway --> Governance
  Feed --> Governance
  Interaction --> Governance
  Gateway --> Monitoring
  Feed --> Monitoring
  Review --> Monitoring

  %% ===== 业务到存储 =====
  Account --> MySQL
  Relation --> MySQL
  Video --> MySQL
  Feed --> MySQL
  Interaction --> MySQL
  Message --> MySQL
  Review --> MySQL
  Admin --> MySQL

  Feed --> Redis
  Reco --> Redis
  Interaction --> Redis

  Interaction --> MQ
  Message --> MQ

  Video --> Obj

  Monitoring --> Prom

  %% ===== 部署承载 =====
  Gateway --- K8s
  Account --- K8s
  Relation --- K8s
  Video --- K8s
  Feed --- K8s
  Reco --- K8s
  Interaction --- K8s
  Message --- K8s
  Review --- K8s
  Admin --- K8s
  Playback --- K8s
  Governance --- K8s
  Monitoring --- K8s

  %% ===== 应用样式 =====
  class App,Web client;
  class Gateway,Auth gateway;
  class Account,Relation,Video,Feed,Reco,Interaction,Message,Review,Admin app;
  class Playback,Governance,Monitoring support;
  class MySQL,Redis,MQ,Obj data;
  class Prom,K8s infra;
```

## 2. 核心链路时序（刷 Feed + 互动 + 审核）

```mermaid
sequenceDiagram
  autonumber
  participant C as App
  participant G as API Gateway
  participant F as Feed 服务
  participant R as 推荐服务
  participant V as 视频服务
  participant I as 互动服务
  participant RV as 审核服务
  participant M as 消息服务
  participant Q as MQ/Kafka
  participant DB as MySQL/Redis

  C->>G: GET /api/feed?cursor=...&limit=...
  G->>F: 转发请求 + 用户身份
  F->>R: /internal/reco/candidates
  R-->>F: 候选视频ID列表
  F->>V: 批量获取视频详情
  V-->>F: 视频卡片数据
  F->>R: /internal/reco/exposure/commit
  F->>DB: 记录游标
  F-->>G: Feed 列表 + next_cursor
  G-->>C: 返回可刷视频流

  C->>G: POST /api/interactions/likes/toggle
  G->>I: 点赞/取消点赞
  I->>DB: 写互动记录 + 更新计数
  I->>Q: 投递互动事件
  Q-->>M: 消费事件
  M->>DB: 写入站内消息
  I-->>G: 操作成功
  G-->>C: 点赞状态更新

  C->>G: POST /api/videos
  G->>V: 发布视频
  V->>RV: /internal/review/tasks 创建审核任务
  RV->>DB: 写审核任务
  RV-->>V: 待审核状态
  V-->>G: 发布受理成功
  G-->>C: 视频进入审核中
```

## 3. 说明

- 对外统一走 `/api/*`，服务间调用走 `/internal/*`，用于明确安全边界。
- 互动、审核、消息通过事件解耦，保证主链路可用性和可扩展性。
- MVP 阶段优先保证主闭环：注册登录 -> 发视频 -> 刷 Feed -> 互动 -> 审核下架。
