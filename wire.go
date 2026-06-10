//go:build wireinject

package main

import (
	"CompeteAI/internal/repository"
	"CompeteAI/internal/repository/cache"
	"CompeteAI/internal/repository/dao"
	"CompeteAI/internal/service"
	"CompeteAI/internal/web"
	ijwt "CompeteAI/internal/web/jwt"
	"CompeteAI/ioc"

	"github.com/gin-gonic/gin"
	"github.com/google/wire"
)

func InitWebServer() *gin.Engine {
	wire.Build(
		// DB
		ioc.InitDB,
		// Cache
		ioc.InitRedis,
		//Logger
		
		// DAO
		dao.NewUserDao,
		dao.NewTaskDao,
		dao.NewReportDao,
		dao.NewTraceDao,
		dao.NewDeadLetterDao,
		cache.NewUserCache,
		// Repository
		repository.NewUserRepository,
		repository.NewTaskRepository,
		repository.NewReportRepository,
		repository.NewTraceRepository,
		// Service
		service.NewUserService,
		service.NewAgentRegistry,
		service.NewWorkflowEngine,
		service.NewTaskService,
		service.NewOpsService,
		service.NewReportService,
		service.NewTraceService,
		// Eino Chat + Firecrawl MCP
		ioc.InitFirecrawlTools,
		ioc.InitChatService,
		ioc.InitKafkaBus,
		ioc.InitKafkaRouter,
		ioc.InitEventHub,
		// Handler
		ijwt.NewRedisJwtHandler,
		web.NewUserHandler,
		web.NewChatHandler,
		web.NewMCPHandler,
		web.NewTaskHandler,
		web.NewOpsHandler,
		web.NewMemoryHandler,
		web.NewReportHandler,
		web.NewTraceHandler,
		web.NewAgentHandler,

		ioc.InitOutboxPublisher,
		ioc.InitMemoryService,
		ioc.InitMiddlewares,

		ioc.InitWebServer,
	)
	return new(gin.Engine)
}
