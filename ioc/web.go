package ioc

import (
	"CompeteAI/internal/metrics"
	"CompeteAI/internal/memory"
	"CompeteAI/internal/outbox"
	"CompeteAI/internal/pkg/ginx/middleware/ratelimit"
	"CompeteAI/internal/web"
	ijwt "CompeteAI/internal/web/jwt"
	"CompeteAI/internal/web/middleware"
	"context"
	"strings"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

func InitWebServer(
	mdls []gin.HandlerFunc,
	userHdl *web.UserHandler,
	chatHdl *web.ChatHandler,
	chatConvHdl *web.ChatConversationHandler,
	mcpHdl *web.MCPHandler,
	taskHdl *web.TaskHandler,
	opsHdl *web.OpsHandler,
	memoryHdl *web.MemoryHandler,
	reportHdl *web.ReportHandler,
	traceHdl *web.TraceHandler,
	agentHdl *web.AgentHandler,
	outboxPub *outbox.Publisher,
) *gin.Engine {
	go outboxPub.Run(context.Background())
	server := gin.Default()
	server.Use(mdls...)
	server.GET("/metrics", gin.WrapH(metrics.HTTPHandler()))
	server.GET("/metrics/prometheus", gin.WrapH(metrics.PrometheusHandler()))
	server.GET("/metrics/memory", func(c *gin.Context) {
		c.JSON(200, memory.MetricsSnapshot())
	})
	userHdl.RegisterRoutes(server)
	chatHdl.RegisterRoutes(server)
	if chatConvHdl != nil {
		chatConvHdl.RegisterRoutes(server)
	}
	if mcpHdl != nil {
		mcpHdl.RegisterRoutes(server)
	}
	taskHdl.RegisterRoutes(server)
	opsHdl.RegisterRoutes(server)
	memoryHdl.RegisterRoutes(server)
	reportHdl.RegisterRoutes(server)
	traceHdl.RegisterRoutes(server)
	agentHdl.RegisterRoutes(server)
	return server
}

func InitMiddlewares(redisClient redis.Cmdable, jwtHdl ijwt.Handler) []gin.HandlerFunc {
	return []gin.HandlerFunc{
		corsHdl(),
		ratelimit.NewBuilder(NewRateLimiter(redisClient, time.Second, 100)).Build(),
		middleware.NewLoginJWTMiddlewareBuilder(jwtHdl).
			IgnorePaths("/users/login").
			IgnorePaths("/users/signup").
			IgnorePaths("/users/refresh_token").
			IgnorePaths("/api/chat/completions").
			IgnorePaths("/api/chat/stream").
			IgnorePaths("/api/mcp/tools").
			IgnorePaths("/api/mcp/call").
			IgnorePaths("/api/tasks").
			IgnorePaths("/api/ops").
			IgnorePaths("/api/memory").
			IgnorePaths("/api/reports").
			IgnorePaths("/api/traces").
			IgnorePaths("/api/agents").Build(),
	}
}

func corsHdl() gin.HandlerFunc {
	return cors.New(cors.Config{
		//AllowOrigins: []string{"http://localhost:3000"},
		//AllowMethods: []string{},
		AllowHeaders: []string{"authorization", "Content-Type"},
		//
		ExposeHeaders: []string{"x-jwt-token", "x-refresh-token"},
		// 是否允许携带用户认证信息，如cookie
		AllowCredentials: true,
		AllowOriginFunc: func(origin string) bool {
			if strings.HasPrefix(origin, "http://localhost") || strings.HasPrefix(origin, "https://") {
				return true
			}
			return strings.Contains(origin, "http://youcompany.com")
		},
		MaxAge: 12 * time.Hour,
	})
}
