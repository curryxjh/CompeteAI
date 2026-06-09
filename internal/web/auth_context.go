package web

import (
	ijwt "CompeteAI/internal/web/jwt"

	"github.com/gin-gonic/gin"
)

func uidFromContext(c *gin.Context) (int64, bool) {
	v, ok := c.Get("claims")
	if !ok {
		return 0, false
	}
	claims, ok := v.(*ijwt.UserClaims)
	if !ok || claims.Uid == 0 {
		return 0, false
	}
	return claims.Uid, true
}
