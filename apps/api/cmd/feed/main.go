package main

import (
	"log"

	"feedsystem_video_hard/internal/infra/config"
	"feedsystem_video_hard/internal/infra/httpgin"
	"feedsystem_video_hard/internal/platform/router"
)

const configPath = "./configs/config.yaml"

func main() {
	// 加载配置
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("load config failed: %v", err)
	}

	// 初始化 Gin 引擎
	g := httpgin.Init()

	// 注册路由
	if err := router.Register(g); err != nil {
		log.Fatalf("init router failed: %v", err)
	}

	// 运行服务器
	if err := httpgin.Run(cfg, g); err != nil {
		log.Fatalf("run server failed: %v", err)
	}
}
