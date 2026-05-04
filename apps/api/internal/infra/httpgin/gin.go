package infrahttpgin

import (
	infraconfig "GCFeed/internal/infra/config"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Init 创建 Gin 引擎，并使用默认日志和恢复中间件。
func Init() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	g := gin.Default()
	return g
}

// Run 根据配置端口启动 HTTP 服务。
func Run(cfg *infraconfig.Config, g *gin.Engine) error {
	port := cfg.Port
	addr := ":" + strconv.Itoa(port)
	return g.Run(addr)
}
