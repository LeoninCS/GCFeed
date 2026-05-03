package interfaceshttprouter

import (
	"database/sql"
	applicationaccount "feedsystem_video_hard/internal/application/account"
	applicationvideo "feedsystem_video_hard/internal/application/video"
	infraconfig "feedsystem_video_hard/internal/infra/config"
	infrajwt "feedsystem_video_hard/internal/infra/jwt"
	infraaccount "feedsystem_video_hard/internal/infra/persistence/account"
	infravideo "feedsystem_video_hard/internal/infra/persistence/video"
	interfaceshttpaccount "feedsystem_video_hard/internal/interfaces/http/account"
	interfaceshttpmiddleware "feedsystem_video_hard/internal/interfaces/http/middleware"
	interfaceshttpvideo "feedsystem_video_hard/internal/interfaces/http/video"

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

	if err := gormDB.AutoMigrate(&infraaccount.UserModel{}, &infravideo.VideoModel{}); err != nil {
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

	return nil
}

func HealthCheck(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "All is well",
	})
}
