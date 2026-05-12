package main

import (
	"log"

	infraconfig "GCFeed/internal/infra/config"
	infradatabase "GCFeed/internal/infra/database"
	infrahttpgin "GCFeed/internal/infra/httpgin"
	interfaceshttprouter "GCFeed/internal/interfaces/http/router"
)

const configPath = "./configs/config.yaml"

func main() {
	// 启动顺序保持简单：配置 -> 数据库 -> Gin -> 路由 -> 启动服务。
	cfg, err := infraconfig.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}
	log.Printf(
		"config loaded: port=%d database=%s:%d/%s jwt_access_ttl=%s",
		cfg.Port,
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.Name,
		cfg.JWT.AccessTTL,
	)

	// 数据库连接使用 database/sql 连接池，后续会被 GORM 复用。
	db, err := infradatabase.New(cfg.Database)
	if err != nil {
		log.Fatalf("init database failed: %v", err)
	}
	log.Println("database connection initialized")

	// Gin 引擎只负责 HTTP 入口，业务依赖在 router.Register 中装配。
	g := infrahttpgin.Init()
	log.Println("gin engine initialized")

	// router.Register 会完成仓储、Service、Handler 和中间件的组装。
	if err := interfaceshttprouter.Register(g, cfg, db); err != nil {
		log.Fatalf("init router failed: %v", err)
	}
	log.Println("router registered")

	// Run 会阻塞当前进程，直到服务器停止或启动失败。
	log.Println("server is running")
	if err := infrahttpgin.Run(cfg, g); err != nil {
		log.Fatalf("run server failed: %v", err)
	}

}
