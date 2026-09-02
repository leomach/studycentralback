package platform

import "github.com/gin-gonic/gin"

// NewRouter devolve um engine já com os middlewares comuns. Ele não conhece
// nenhum domínio: quem monta as rotas é o cmd/api/main.go, para que platform
// nunca dependa de catalog/question/flashcard/dashboard.
func NewRouter(cfg Config) *gin.Engine {
	if !cfg.IsDevelopment() {
		gin.SetMode(gin.ReleaseMode)
	}

	r := gin.New()
	r.Use(gin.Recovery(), RequestLogger(), CORS(cfg.CORSOrigins))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	return r
}
