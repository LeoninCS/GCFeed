package interfaceshttpmiddleware

import (
	infrajwt "GCFeed/internal/infra/jwt"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

const ContextUserIDKey = "auth_user_id"
const ContextRoleKey = "auth_role"

// NewJWTAuth 返回 Gin 鉴权中间件，负责解析 Bearer token 并写入用户上下文。
func NewJWTAuth(jwtManager *infrajwt.Manager) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := strings.TrimSpace(c.GetHeader("Authorization"))
		if header == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"message": "authorization header is required",
			})
			return
		}

		// Authorization 标准格式为：Bearer <access_token>。
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

		// 后续 Handler 从 gin.Context 中读取用户 ID 和角色，避免重复解析 JWT。
		c.Set(ContextUserIDKey, claims.UserID)
		c.Set(ContextRoleKey, claims.Role)
		c.Next()
	}
}
