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
		cache.NewUserCache,
		// Repository
		repository.NewUserRepository,
		// Service
		service.NewUserService,
		// Eino Chat
		ioc.InitChatService,
		// Handler
		ijwt.NewRedisJwtHandler,
		web.NewUserHandler,
		web.NewChatHandler,

		// middlewares
		ioc.InitMiddlewares,

		ioc.InitWebServer,
	)
	return new(gin.Engine)
}
