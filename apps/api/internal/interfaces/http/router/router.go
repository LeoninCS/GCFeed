package interfaceshttprouter

import (
	applicationaccount "GCFeed/internal/application/account"
	applicationexposure "GCFeed/internal/application/exposure"
	applicationfeed "GCFeed/internal/application/feed"
	applicationinteraction "GCFeed/internal/application/interaction"
	applicationrecommendation "GCFeed/internal/application/recommendation"
	applicationrelation "GCFeed/internal/application/relation"
	applicationvideo "GCFeed/internal/application/video"
	domainfeed "GCFeed/internal/domain/feed"
	infracache "GCFeed/internal/infra/cache"
	infraconfig "GCFeed/internal/infra/config"
	infrajwt "GCFeed/internal/infra/jwt"
	inframq "GCFeed/internal/infra/mq"
	infraaccount "GCFeed/internal/infra/persistence/account"
	infraexposure "GCFeed/internal/infra/persistence/exposure"
	infrafeed "GCFeed/internal/infra/persistence/feed"
	infrainteraction "GCFeed/internal/infra/persistence/interaction"
	migration "GCFeed/internal/infra/persistence/migration"
	infrarecommendation "GCFeed/internal/infra/persistence/recommendation"
	infrarelation "GCFeed/internal/infra/persistence/relation"
	infravideo "GCFeed/internal/infra/persistence/video"
	interfaceshttpaccount "GCFeed/internal/interfaces/http/account"
	interfaceshttpexposure "GCFeed/internal/interfaces/http/exposure"
	interfaceshttpfeed "GCFeed/internal/interfaces/http/feed"
	interfaceshttpinteraction "GCFeed/internal/interfaces/http/interaction"
	interfaceshttpmiddleware "GCFeed/internal/interfaces/http/middleware"
	interfaceshttprecommendation "GCFeed/internal/interfaces/http/recommendation"
	interfaceshttprelation "GCFeed/internal/interfaces/http/relation"
	interfaceshttpupload "GCFeed/internal/interfaces/http/upload"
	interfaceshttpvideo "GCFeed/internal/interfaces/http/video"
	"context"
	"database/sql"
	"log"

	"github.com/gin-gonic/gin"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Register 负责后端依赖装配：数据库模型、仓储、Service、Handler、中间件和路由。
func Register(g *gin.Engine, cfg *infraconfig.Config, db *sql.DB) error {
	// database/sql 连接池交给 GORM 复用，避免维护两套数据库连接。
	gormDB, err := gorm.Open(gormmysql.New(gormmysql.Config{
		Conn: db,
	}), &gorm.Config{})
	if err != nil {
		return err
	}

	// AutoMigrate 根据模型创建或补齐表结构，适合教学项目快速启动。
	if err := migration.AutoMigrate(gormDB); err != nil {
		return err
	}

	// JWT Manager 同时被账号服务用于签发 token，也被鉴权中间件用于校验 token。
	jwtManager, err := infrajwt.NewManager(cfg.JWT.Secret, cfg.JWT.AccessTTL)
	if err != nil {
		return err
	}

	// 下面按领域模块组装依赖：Repository -> Service -> Handler。
	accountRepo := infraaccount.New(gormDB)
	accountService := applicationaccount.New(accountRepo, jwtManager)
	accountHandler := interfaceshttpaccount.New(accountService)
	videoRepo := infravideo.New(gormDB)
	feedRepo := infrafeed.New(gormDB)
	recommendationRepo := infrarecommendation.New(gormDB)
	recommendationService := applicationrecommendation.New(recommendationRepo)
	recommendationHandler := interfaceshttprecommendation.New(recommendationService)
	feedOptions := []applicationfeed.Option{applicationfeed.WithRecommender(recommendationService)}
	videoOptions := []applicationvideo.Option{}
	interactionOptions := []applicationinteraction.Option{}
	exposureOptions := []applicationexposure.Option{}
	var feedCache *infracache.FeedCache
	var rabbitMQ *inframq.RabbitMQ
	if cfg.Redis.Addr != "" {
		redisClient := infracache.NewRedisClient(cfg.Redis)
		feedCache = infracache.NewFeedCache(redisClient)
		feedOptions = append(feedOptions, applicationfeed.WithFeedCache(feedCache))
		interactionOptions = append(interactionOptions, applicationinteraction.WithHotScoreRecorder(feedCache))
	}
	feedService := applicationfeed.New(feedRepo, feedOptions...)
	feedHandler := interfaceshttpfeed.New(feedService)
	interactionRepo := infrainteraction.New(gormDB)
	if cfg.RabbitMQ.URL != "" {
		rabbitMQ, err = inframq.NewRabbitMQ(cfg.RabbitMQ)
		if err != nil {
			log.Printf("rabbitmq disabled: %v", err)
		} else {
			videoOptions = append(videoOptions, applicationvideo.WithPublishedEventPublisher(rabbitMQ))
			exposureOptions = append(exposureOptions, applicationexposure.WithViewEventPublisher(rabbitMQ))
			if feedCache != nil {
				interactionOptions = append(interactionOptions, applicationinteraction.WithAsyncActionPipeline(feedCache, rabbitMQ))
			}
		}
	}
	videoService := applicationvideo.New(videoRepo, videoOptions...)
	videoHandler := interfaceshttpvideo.New(videoService)
	interactionService := applicationinteraction.New(interactionRepo, interactionOptions...)
	interactionHandler := interfaceshttpinteraction.New(interactionService)
	exposureRepo := infraexposure.New(gormDB)
	exposureService := applicationexposure.New(exposureRepo, exposureOptions...)
	exposureHandler := interfaceshttpexposure.New(exposureService)
	relationRepo := infrarelation.New(gormDB)
	relationOptions := []applicationrelation.Option{}
	if feedCache != nil {
		relationOptions = append(relationOptions, applicationrelation.WithFollowFeedBackfiller(NewFollowFeedBackfiller(feedRepo, feedCache)))
	}
	relationService := applicationrelation.New(relationRepo, relationOptions...)
	relationHandler := interfaceshttprelation.New(relationService)
	uploadHandler := interfaceshttpupload.New("./uploads")

	g.GET("/health", HealthCheck)
	// 静态文件路由让上传后的文件可以通过 /uploads/... 访问。
	g.Static("/uploads", "./uploads")

	authMiddleware := interfaceshttpmiddleware.NewJWTAuth(jwtManager)
	optionalAuthMiddleware := interfaceshttpmiddleware.NewOptionalJWTAuth(jwtManager)
	api := g.Group("/api")

	// RESTful 路由约定：路径表达资源，HTTP 方法表达动作。
	// 会话资源用于登录态：创建会话表示登录，删除当前会话表示登出。
	sessions := api.Group("/sessions")
	sessions.POST("", accountHandler.Login)
	sessions.DELETE("/current", authMiddleware, accountHandler.Logout)

	// 用户资源承载注册、当前用户资料和用户作品列表。
	users := api.Group("/users")
	users.POST("", accountHandler.Register)
	users.GET("/me", authMiddleware, accountHandler.Me)
	users.PATCH("/me", authMiddleware, accountHandler.UpdateMe)
	users.GET("/me/videos", authMiddleware, videoHandler.ListMine)
	users.PUT("/me/following/:targetUserId", authMiddleware, relationHandler.Follow)
	users.DELETE("/me/following/:targetUserId", authMiddleware, relationHandler.Unfollow)
	users.GET("/me/following", authMiddleware, relationHandler.ListFollowing)
	users.GET("/me/followers", authMiddleware, relationHandler.ListFollowers)
	users.GET("/:userId", accountHandler.Get)
	users.GET("/:userId/videos", videoHandler.ListByAuthor)

	// 视频是互动资源的父资源，点赞、收藏和评论都挂在具体视频下。
	videos := api.Group("/videos")
	videos.POST("", authMiddleware, videoHandler.Create)
	videos.GET("/:videoId", videoHandler.Get)
	videos.DELETE("/:videoId", authMiddleware, videoHandler.Delete)
	videos.PUT("/:videoId/like", authMiddleware, interactionHandler.Like)
	videos.DELETE("/:videoId/like", authMiddleware, interactionHandler.Unlike)
	videos.PUT("/:videoId/favorite", authMiddleware, interactionHandler.Favorite)
	videos.DELETE("/:videoId/favorite", authMiddleware, interactionHandler.Unfavorite)
	videos.POST("/:videoId/comments", authMiddleware, interactionHandler.CreateComment)
	videos.GET("/:videoId/comments", interactionHandler.ListComments)

	uploads := api.Group("/uploads", authMiddleware)
	uploads.POST("", uploadHandler.Create)

	// Feed 暴露为条目集合，客户端通过游标和 limit 控制分页。
	api.GET("/feed-items", optionalAuthMiddleware, feedHandler.ListFeedItems)
	api.POST("/feed-queries", optionalAuthMiddleware, feedHandler.Query)
	api.POST("/video-view-events", authMiddleware, exposureHandler.CreateViewEvent)
	// 删除评论只需要评论自身 ID，所以放在顶层 comments 资源下。
	api.DELETE("/comments/:commentId", authMiddleware, interactionHandler.DeleteComment)

	internal := g.Group("/internal")
	internal.POST("/recommendation-candidates", recommendationHandler.ListCandidates)
	internal.POST("/exposure-decisions", recommendationHandler.DecideExposures)
	internal.POST("/exposures", recommendationHandler.SaveExposures)

	return nil
}

type FollowFeedBackfiller struct {
	feedRepo interface {
		CountFollowers(ctx context.Context, authorID int64) (int, error)
		ListAuthorRecentVideos(ctx context.Context, authorID int64, limit int) ([]*domainfeed.FeedPageItem, error)
	}
	feedCache interface {
		AddInboxItems(ctx context.Context, authorID int64, userIDs []int64, item *domainfeed.FeedPageItem, maxLen int64) error
	}
}

func NewFollowFeedBackfiller(feedRepo interface {
	CountFollowers(ctx context.Context, authorID int64) (int, error)
	ListAuthorRecentVideos(ctx context.Context, authorID int64, limit int) ([]*domainfeed.FeedPageItem, error)
}, feedCache interface {
	AddInboxItems(ctx context.Context, authorID int64, userIDs []int64, item *domainfeed.FeedPageItem, maxLen int64) error
}) *FollowFeedBackfiller {
	return &FollowFeedBackfiller{feedRepo: feedRepo, feedCache: feedCache}
}

func (b *FollowFeedBackfiller) CountFollowers(ctx context.Context, authorID int64) (int, error) {
	return b.feedRepo.CountFollowers(ctx, authorID)
}

func (b *FollowFeedBackfiller) ListAuthorRecentVideos(ctx context.Context, authorID int64, limit int) ([]*domainfeed.FeedPageItem, error) {
	return b.feedRepo.ListAuthorRecentVideos(ctx, authorID, limit)
}

func (b *FollowFeedBackfiller) AddInboxItems(ctx context.Context, authorID int64, userIDs []int64, item *domainfeed.FeedPageItem, maxLen int64) error {
	return b.feedCache.AddInboxItems(ctx, authorID, userIDs, item, maxLen)
}

// HealthCheck 提供基础健康检查接口，方便本地调试和容器探活。
func HealthCheck(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "All is well",
	})
}
