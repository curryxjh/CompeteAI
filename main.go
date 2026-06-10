package main

import (
	"CompeteAI/internal/app"
	"log"
	"os"
	"strings"
)

// subcommands 是已知的子命令名，不应被当作配置文件路径。
var subcommands = map[string]bool{
	"server": true,
	"api":    true,
	"worker": true,
	"all":    true,
	"run":    true,
	"start":  true,
}

// isConfigFilePath 判断参数是否为配置文件路径（含路径分隔符或 yaml/json/toml 后缀）。
func isConfigFilePath(s string) bool {
	if subcommands[strings.ToLower(s)] {
		return false
	}
	return strings.Contains(s, "/") ||
		strings.HasSuffix(s, ".yaml") ||
		strings.HasSuffix(s, ".yml") ||
		strings.HasSuffix(s, ".json") ||
		strings.HasSuffix(s, ".toml")
}

// 支持以下启动方式：
//
//	go run . server              ← 仅 API（默认 config/dev.yaml）
//	go run . all                 ← API + Worker 同进程（本地开发推荐）
//	go run . all config/prod.yaml
//	go run . config/prod.yaml    ← 仅 API，指定配置文件
//	CONFIG_FILE=config/prod.yaml go run .
func main() {
	configFile := app.ConfigFromEnv()

	// 第一个参数若是配置文件路径则提取出来
	args := os.Args[1:]
	if len(args) > 0 && isConfigFilePath(args[0]) {
		configFile = args[0]
		args = args[1:]
	}

	// 第一个非路径参数作为子命令
	subcmd := "server"
	if len(args) > 0 {
		subcmd = strings.ToLower(args[0])
		// 子命令后可跟配置文件（如 go run . all config/prod.yaml）
		if len(args) > 1 && isConfigFilePath(args[1]) {
			configFile = args[1]
		}
	}

	var err error
	switch subcmd {
	case "all":
		log.Printf("[all-in-one] starting API + Worker with config: %s", configFile)
		err = app.RunAll(configFile)
	default: // "server", "api", 其他
		err = app.RunAPI(configFile)
	}

	if err != nil {
		log.Fatal(err)
	}
}
