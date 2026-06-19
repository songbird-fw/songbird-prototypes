package auth

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func RequireJWT(m *JWTManager) gin.HandlerFunc {
	return func(c *gin.Context) {
		h := c.GetHeader("Authorization")
		if !strings.HasPrefix(h, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "missing bearer token"})
			return
		}
		raw := strings.TrimPrefix(h, "Bearer ")
		tok, err := m.ParseAccessToken(raw)
		if err != nil || !tok.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "invalid token"})
			return
		}
		claims := tok.Claims.(map[string]interface{})
		c.Set("userID", claims["sub"])
		c.Set("role", claims["role"])
		c.Next()
	}
}
