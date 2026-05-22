package logger

import (
	"net"
	"net/http"
	"net/http/httputil"
	"os"
	"runtime/debug"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Zap 供 GORM 等必须使用 *zap.Logger 的组件使用，业务代码请用 L()。
func Zap() *zap.Logger {
	return L().(*ZapLogger).l
}

func GinLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.Request.URL.Path
		query := c.Request.URL.RawQuery
		c.Next()

		cost := time.Since(start)
		L().Info(path,
			Int("status", c.Writer.Status()),
			String("method", c.Request.Method),
			String("path", path),
			String("query", query),
			String("ip", c.ClientIP()),
			String("user-agent", c.Request.UserAgent()),
			String("errors", c.Errors.ByType(gin.ErrorTypePrivate).String()),
			Duration("cost", cost),
		)
	}
}

func GinRecovery(stack bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if err := recover(); err != nil {
				// Check for a broken connection, as it is not really a
				// condition that warrants a panic stack trace.
				var brokenPipe bool
				if ne, ok := err.(*net.OpError); ok {
					if se, ok := ne.Err.(*os.SyscallError); ok {
						if strings.Contains(strings.ToLower(se.Error()), "broken pipe") || strings.Contains(strings.ToLower(se.Error()), "connection reset by peer") {
							brokenPipe = true
						}
					}
				}

				httpRequest, _ := httputil.DumpRequest(c.Request, false)
				if brokenPipe {
					L().Error(c.Request.URL.Path,
						Any("error", err),
						String("request", string(httpRequest)),
					)
					// If the connection is dead, we can't write a status to it.
					c.Error(err.(error)) // nolint: errcheck
					c.Abort()
					return
				}

				if stack {
					L().Error("[Recovery from panic]",
						Any("error", err),
						String("request", string(httpRequest)),
						String("stack", string(debug.Stack())),
					)
				} else {
					L().Error("[Recovery from panic]",
						Any("error", err),
						String("request", string(httpRequest)),
					)
				}
				c.AbortWithStatus(http.StatusInternalServerError)
			}
		}()
		c.Next()
	}
}
