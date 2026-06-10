package app

import (
	"CompeteAI/settings"
	"fmt"
	"os"
	"path/filepath"
)

// LoadConfigFile 使用指定路径加载配置；为空则使用默认路径。
func LoadConfigFile(configFile string) error {
	if configFile == "" {
		configFile = filepath.Join("config", "dev.yaml")
	}
	if err := settings.Init(configFile); err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	return settings.InitLogger()
}

// LoadConfig 从命令行参数或默认路径加载配置（Worker / Recovery 用）。
func LoadConfig() error {
	configFile := filepath.Join("config", "dev.yaml")
	if len(os.Args) >= 2 && os.Args[1] != "" && os.Args[1][0] != '-' {
		configFile = os.Args[1]
	}
	return LoadConfigFile(configFile)
}

func WorkerRole() string {
	if r := os.Getenv("WORKER_ROLE"); r != "" {
		return r
	}
	return "all"
}

func WorkerID() string {
	if id := os.Getenv("WORKER_ID"); id != "" {
		return id
	}
	host, _ := os.Hostname()
	if host == "" {
		return "worker-local"
	}
	return host
}
