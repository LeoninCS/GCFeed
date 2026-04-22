package interfaceshttpmiddleware

import (
	"net/http"
	"strings"
	infrajwt "feedsystem_video_hard/internal/infra/jwt"

	"github.com/gin-gonic/gin"
)

const ContextUserIDKey = "auth_user_id"

func NewJWTAuth(jwtManager *infrajwt.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "authorization header is required",
			})
			return
		}

		parts := strings.SplitN(header, " ", 2)
		if len(parts) != 2 {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "authorization format must be Bearer <token>",
			})
			return
		}
		if !strings.EqualFold(strings.TrimSpace(parts[0]), "Bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "authorization scheme must be Bearer",
			})
			return
		}

		token := strings.TrimSpace(parts[1])
		claims, err := jwtManager.ParseAndValidateToken(token, infrajwt.TokenTypeAccess)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "invalid access token",
			})
			return
		}

		c.Set(ContextUserIDKey, claims.UserID)
		c.Next()
	}
}
