package main

import (
	"log"

	infraconfig "feedsystem_video_hard/internal/infra/config"
	infradatabase "feedsystem_video_hard/internal/infra/database"
	infrahttpgin "feedsystem_video_hard/internal/infra/httpgin"
	interfaceshttprouter "feedsystem_video_hard/internal/interfaces/http/router"
)

const configPath = "./configs/config.yaml"

func main() {
	// 加载配置
	cfg, err := infraconfig.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}
	log.Printf("config loaded: %+v", cfg)

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
	if err := infrahttpgin.Run(cfg, g); err != nil {
		log.Fatalf("run server failed: %v", err)
	}
	log.Println("server is running")

}
