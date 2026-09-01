package platform

import (
	"crypto/subtle"
	"log"
	"net/http"
	"strings"
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

// RequireAuth exige um access token válido em "Authorization: Bearer <token>".
// Sem ele, a request nem chega ao handler — 401 direto. Grava user_id e plan
// no contexto do Gin, lidos depois por UserID e RequirePremium.
func RequireAuth(cfg Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		header := c.GetHeader("Authorization")
		token, ok := strings.CutPrefix(header, "Bearer ")
		if !ok || token == "" {
			Fail(c, Unauthorized("token de acesso ausente"))
			return
		}

		claims, err := ParseAccessToken(token, cfg.JWTSecret)
		if err != nil {
			Fail(c, Unauthorized("token de acesso inválido ou expirado"))
			return
		}

		c.Set("user_id", claims.UserID)
		c.Set("plan", claims.Plan)
		c.Next()
	}
}

// RequirePremium bloqueia contas do plano free. Roda sempre depois de
// RequireAuth — sem plan no contexto, nega por padrão (falha fechada, não
// aberta: um erro de ordem de middleware bloqueia acesso em vez de liberar).
func RequirePremium() gin.HandlerFunc {
	return func(c *gin.Context) {
		plan, _ := c.Get("plan")
		if plan != "premium" {
			Fail(c, Forbidden("este recurso exige plano premium"))
			return
		}
		c.Next()
	}
}

// RequireAdminSecret protege as rotas administrativas (hoje, só promover uma
// conta a premium — não existe cobrança real ainda). ConstantTimeCompare
// evita que o tempo de resposta vaze quantos caracteres do segredo bateram.
func RequireAdminSecret(cfg Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		given := c.GetHeader("X-Admin-Secret")
		if subtle.ConstantTimeCompare([]byte(given), []byte(cfg.AdminSecret)) != 1 {
			Fail(c, Unauthorized("secret de admin inválido"))
			return
		}
		c.Next()
	}
}

// UserID lê o usuário autenticado gravado por RequireAuth. Só é chamado por
// rotas que passam por esse middleware — não existe mais um fallback
// silencioso para um usuário fixo.
func UserID(c *gin.Context) uint {
	if v, ok := c.Get("user_id"); ok {
		if id, ok := v.(uint); ok {
			return id
		}
	}
	return 0
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
