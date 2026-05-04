package interfaceshttprouter

import (
	applicationaccount "GCFeed/internal/application/account"
	applicationfeed "GCFeed/internal/application/feed"
	applicationinteraction "GCFeed/internal/application/interaction"
	applicationvideo "GCFeed/internal/application/video"
	infraconfig "GCFeed/internal/infra/config"
	infrajwt "GCFeed/internal/infra/jwt"
	infraaccount "GCFeed/internal/infra/persistence/account"
	infrafeed "GCFeed/internal/infra/persistence/feed"
	infrainteraction "GCFeed/internal/infra/persistence/interaction"
	infravideo "GCFeed/internal/infra/persistence/video"
	interfaceshttpaccount "GCFeed/internal/interfaces/http/account"
	interfaceshttpfeed "GCFeed/internal/interfaces/http/feed"
	interfaceshttpinteraction "GCFeed/internal/interfaces/http/interaction"
	interfaceshttpmiddleware "GCFeed/internal/interfaces/http/middleware"
	interfaceshttpupload "GCFeed/internal/interfaces/http/upload"
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
		&infravideo.VideoStatModel{},
		&infrainteraction.ActionModel{},
		&infrainteraction.CommentModel{},
	); err != nil {
		return err
	}
	if err := infravideo.EnsureStats(gormDB); err != nil {
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
	interactionRepo := infrainteraction.New(gormDB)
	interactionService := applicationinteraction.NewService(interactionRepo)
	interactionHandler := interfaceshttpinteraction.NewHandler(interactionService)
	uploadHandler := interfaceshttpupload.NewHandler("./uploads")

	g.GET("/health", HealthCheck)
	g.Static("/uploads", "./uploads")

	authMiddleware := interfaceshttpmiddleware.NewJWTAuth(jwtManager)
	api := g.Group("/api")

	sessions := api.Group("/sessions")
	sessions.POST("", accountHandler.Login)
	sessions.DELETE("/current", authMiddleware, accountHandler.Logout)

	users := api.Group("/users")
	users.POST("", accountHandler.Register)
	users.GET("/me", authMiddleware, accountHandler.Me)
	users.PATCH("/me", authMiddleware, accountHandler.UpdateMe)
	users.GET("/me/videos", authMiddleware, videoHandler.ListMine)
	users.GET("/:userId/videos", videoHandler.ListByAuthor)

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

	api.GET("/feed-items", feedHandler.Timeline)
	api.DELETE("/comments/:commentId", authMiddleware, interactionHandler.DeleteComment)

	return nil
}

func HealthCheck(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "All is well",
	})
}
