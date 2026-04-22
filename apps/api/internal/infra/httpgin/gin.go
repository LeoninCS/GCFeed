package infrahttpgin

import (
	"strconv"
	infraconfig "feedsystem_video_hard/internal/infra/config"

	"github.com/gin-gonic/gin"
)

func Init() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	g := gin.Default()
	return g
}

func Run(cfg *infraconfig.Config, g *gin.Engine) error {
	port := cfg.Port
	addr := ":" + strconv.Itoa(port)
	return g.Run(addr)
}
