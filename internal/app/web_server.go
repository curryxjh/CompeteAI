package app

import (
	"CompeteAI/internal/repository"
	"CompeteAI/internal/repository/cache"
	"CompeteAI/internal/repository/dao"
	"CompeteAI/internal/service"
	"CompeteAI/internal/web"
	"CompeteAI/internal/web/jwt"
	"CompeteAI/ioc"
	"CompeteAI/settings"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"github.com/gin-gonic/gin"
)

// InitWebServer 组装 API 进程 Gin 引擎（与 wire 注入等价）。
func InitWebServer() *gin.Engine {
	cmdable := ioc.InitRedis()
	handler := jwt.NewRedisJwtHandler(cmdable)
	mdls := ioc.InitMiddlewares(cmdable, handler)
	db := ioc.InitDB()
	userDao := dao.NewUserDao(db)
	userCache := cache.NewUserCache(cmdable)
	userRepository := repository.NewUserRepository(userDao, userCache)
	userService := service.NewUserService(userRepository)
	userHandler := web.NewUserHandler(userService, cmdable)
	toolRegistry := ioc.InitFirecrawlTools()
	chatService := ioc.InitChatService(toolRegistry)
	chatHandler := web.NewChatHandler(chatService)
	chatDao := dao.NewChatDao(db)
	chatRepo := repository.NewChatRepository(chatDao)
	chatConvService := service.NewChatConversationService(chatRepo)
	chatConvHandler := web.NewChatConversationHandler(chatConvService)
	mcpHandler := web.NewMCPHandler(toolRegistry)
	taskDao := dao.NewTaskDao(db)
	taskRepository := repository.NewTaskRepository(taskDao)
	reportDao := dao.NewReportDao(db)
	reportRepository := repository.NewReportRepository(reportDao)
	traceDao := dao.NewTraceDao(db)
	traceRepository := repository.NewTraceRepository(traceDao)
	registry := service.NewAgentRegistry(chatService, toolRegistry)
	taskHub := ioc.InitEventHub(db)
	bus := ioc.InitKafkaBus(cmdable)
	router := ioc.InitKafkaRouter(bus)
	outboxPublisher := ioc.InitOutboxPublisher(db, bus)
	memSvc := ioc.InitMemoryService(db)
	engine := service.NewWorkflowEngine(registry, taskRepository, reportRepository, traceRepository, taskHub, bus, router, cmdable, memSvc)
	taskService := service.NewTaskService(db, taskRepository, reportRepository, traceRepository, engine, outboxPublisher)
	taskHandler := web.NewTaskHandler(taskService, taskHub)
	deadLetterDao := dao.NewDeadLetterDao(db)
	opsService := service.NewOpsService(deadLetterDao, outboxPublisher)
	opsHandler := web.NewOpsHandler(opsService)
	memoryHandler := web.NewMemoryHandler(memSvc)
	reportService := service.NewReportService(reportRepository)
	reportHandler := web.NewReportHandler(reportService)
	traceService := service.NewTraceService(traceRepository)
	traceHandler := web.NewTraceHandler(traceService)
	agentHandler := web.NewAgentHandler(registry)
	return ioc.InitWebServer(mdls, userHandler, chatHandler, chatConvHandler, mcpHandler, taskHandler, opsHandler, memoryHandler, reportHandler, traceHandler, agentHandler, outboxPublisher)
}

// RunAPI 启动 API 进程。
func RunAPI(configFile string) error {
	if configFile == "" {
		configFile = filepath.Join("config", "dev.yaml")
	}
	if err := settings.Init(configFile); err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := settings.InitLogger(); err != nil {
		return fmt.Errorf("init logger: %w", err)
	}
	server := InitWebServer()
	server.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong", "mode": "api"})
	})
	log.Printf("[api] listening :%d — worker: go run ./cmd/worker", settings.Conf.Port)
	return server.Run(fmt.Sprintf(":%d", settings.Conf.Port))
}

// RunAPIFromArgs 解析命令行 config 路径后启动 API。
func RunAPIFromArgs(args []string) error {
	configFile := filepath.Join("config", "dev.yaml")
	if len(args) >= 2 && args[1] != "" {
		configFile = args[1]
	}
	return RunAPI(configFile)
}

// DefaultConfigPath 默认配置文件路径。
func DefaultConfigPath() string {
	return filepath.Join("config", "dev.yaml")
}

// ConfigFromEnv 允许 CONFIG_FILE 环境变量覆盖。
func ConfigFromEnv() string {
	if v := os.Getenv("CONFIG_FILE"); v != "" {
		return v
	}
	return DefaultConfigPath()
}
