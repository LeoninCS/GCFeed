package interfaceshttprouter

import (
	"database/sql"
	applicationaccount "feedsystem_video_hard/internal/application/account"
	infraaccount "feedsystem_video_hard/internal/infra/persistence/account"
	infraconfig "feedsystem_video_hard/internal/infra/config"
	infrajwt "feedsystem_video_hard/internal/infra/jwt"
	interfaceshttpaccount "feedsystem_video_hard/internal/interfaces/http/account"

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

	if err := gormDB.AutoMigrate(&infraaccount.UserModel{}); err != nil {
		return err
	}

	jwtManager, err := infrajwt.NewManager(cfg.JWT.Secret, cfg.JWT.AccessTTL)
	if err != nil {
		return err
	}

	accountRepo := infraaccount.New(gormDB)
	accountService := applicationaccount.NewService(accountRepo, jwtManager)
	accountHandler := interfaceshttpaccount.NewHandler(accountService)

	g.GET("/health", HealthCheck)
	auth := g.Group("/auth")
	auth.POST("/register", accountHandler.Register)
	auth.POST("/login", accountHandler.Login)

	return nil
}

func HealthCheck(c *gin.Context) {
	c.JSON(200, gin.H{
		"message": "All is well",
	})
}
