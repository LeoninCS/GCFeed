# 模块文档索引（MVP）

- 账户模块：`docs/modules/account.md`
- 关系模块：`docs/modules/relation.md`
- 视频模块：`docs/modules/video.md`
- 推荐模块：`docs/modules/recommendation.md`
- Feed 模块：`docs/modules/feed.md`
- 互动模块：`docs/modules/interaction.md`
- 消息模块：`docs/modules/message.md`
- 审核模块：`docs/modules/review.md`
- 后台运营模块：`docs/modules/admin.md`
- 播放优化模块：`docs/modules/playback.md`
- 系统治理模块：`docs/modules/governance.md`
- 监控告警模块：`docs/modules/monitoring.md`

最小实现原则：
- 每个模块仅保留必要闭环能力，优先可上线而不是一次做全。
- 接口数量控制在 2-5 个，覆盖核心读写。
- 数据表优先 1-2 张主表（监控类可 3 张），字段只保留当前版本必需项。
