package interfaceshttprouter

import (
	applicationaccount "GCFeed/internal/application/account"
	applicationfeed "GCFeed/internal/application/feed"
	applicationvideo "GCFeed/internal/application/video"
	infraconfig "GCFeed/internal/infra/config"
	infrajwt "GCFeed/internal/infra/jwt"
	infraaccount "GCFeed/internal/infra/persistence/account"
	infrafeed "GCFeed/internal/infra/persistence/feed"
	infravideo "GCFeed/internal/infra/persistence/video"
	interfaceshttpaccount "GCFeed/internal/interfaces/http/account"
	interfaceshttpfeed "GCFeed/internal/interfaces/http/feed"
	interfaceshttpmiddleware "GCFeed/internal/interfaces/http/middleware"
	interfaceshttpvideo "GCFeed/internal/interfaces/http/video"
	"database/sql"

	"github.com/gin-gonic/gin"
	gormmysql "gorm.io/driver/mysql"
	"gorm.io/gorm"
)

func Register(g *gin.Engine, cfg *infraconfig.Config, db *sql.DB) error {
	gormDB, err := gorm.Open(gormmysql.New(gormmysql.Config{
		Conn: db,
	}), &gorm.Config{})
	if err != nil {
		return err
	}

	if err := gormDB.AutoMigrate(
		&infraaccount.UserModel{},
		&infravideo.VideoModel{},
		&infrafeed.FeedCursorModel{},
		&infrafeed.FeedViewEventModel{},
	); err != nil {
		return err
	}

	jwtManager, err := infrajwt.NewManager(cfg.JWT.Secret, cfg.JWT.AccessTTL)
	if err != nil {
		return err
	}

	accountRepo := infraaccount.New(gormDB)
	accountService := applicationaccount.NewService(accountRepo, jwtManager)
	accountHandler := interfaceshttpaccount.NewHandler(accountService)
	videoRepo := infravideo.New(gormDB)
	videoService := applicationvideo.NewService(videoRepo)
	videoHandler := interfaceshttpvideo.NewHandler(videoService)
	feedRepo := infrafeed.New(gormDB)
	feedService := applicationfeed.NewService(feedRepo)
	feedHandler := interfaceshttpfeed.NewHandler(feedService)

	g.GET("/health", HealthCheck)

	authMiddleware := interfaceshttpmiddleware.NewJWTAuth(jwtManager)
	api := g.Group("/api")

	auth := api.Group("/auth")
	auth.POST("/register", accountHandler.Register)
	auth.POST("/login/password", accountHandler.Login)
	auth.POST("/logout", authMiddleware, accountHandler.Logout)

	users := api.Group("/users", authMiddleware)
	users.GET("/me", accountHandler.Me)
	users.PATCH("/me", accountHandler.UpdateMe)

	videos := api.Group("/videos")
	videos.POST("", authMiddleware, videoHandler.Create)
	videos.GET("/mine", authMiddleware, videoHandler.ListMine)
	videos.GET("/:videoId", videoHandler.Get)
	videos.DELETE("/:videoId", authMiddleware, videoHandler.Delete)

	api.GET("/users/:userId/videos", videoHandler.ListByAuthor)

	feed := api.Group("/feed")
	feed.GET("/time", feedHandler.Time)
	feed.GET("/refresh", feedHandler.Refresh)
	feed.POST("/view-events", feedHandler.ReportViewEvent)

	return nil
}

func HealthCheck(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "All is well",
	})
}
