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
	// 加载配置
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

	// 初始化数据库连接
	db, err := infradatabase.New(cfg.Database)
	if err != nil {
		log.Fatalf("init database failed: %v", err)
	}
	log.Println("database connection initialized")

	// 初始化 Gin 引擎
	g := infrahttpgin.Init()
	log.Println("gin engine initialized")

	// 注册路由
	if err := interfaceshttprouter.Register(g, cfg, db); err != nil {
		log.Fatalf("init router failed: %v", err)
	}
	log.Println("router registered")

	// 运行服务器
	log.Println("server is running")
	if err := infrahttpgin.Run(cfg, g); err != nil {
		log.Fatalf("run server failed: %v", err)
	}
}
