package platform

import (
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// RequestLogger substitui o gin.Logger() padrão por um formato mais enxuto.
func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		log.Printf("%s %s %d %s", c.Request.Method, c.Request.URL.Path,
			c.Writer.Status(), time.Since(start).Round(time.Millisecond))
	}
}

// CurrentUser é o placeholder do dono dos dados. O MVP é monousuário: as
// tabelas já carregam user_id, mas não há isolamento ativo nem autenticação.
// Quando houver login de verdade, só este middleware precisa mudar.
const singleUserID uint = 1

func CurrentUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("user_id", singleUserID)
		c.Next()
	}
}

func UserID(c *gin.Context) uint {
	if v, ok := c.Get("user_id"); ok {
		if id, ok := v.(uint); ok {
			return id
		}
	}
	return singleUserID
}

// CORS libera a origem do PWA. Escrito à mão em vez de usar gin-contrib/cors:
// são poucos headers e uma dependência menos para manter.
func CORS(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", origin)
		c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		c.Header("Access-Control-Max-Age", "86400")
		c.Header("Vary", "Origin")

		// O preflight do navegador não deve seguir para os handlers.
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
