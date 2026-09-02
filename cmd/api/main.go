package main

import (
	"log"
	"time"

	"github.com/leomach/studycentralback/internal/auth"
	"github.com/leomach/studycentralback/internal/catalog"
	"github.com/leomach/studycentralback/internal/dashboard"
	"github.com/leomach/studycentralback/internal/flashcard"
	"github.com/leomach/studycentralback/internal/platform"
	"github.com/leomach/studycentralback/internal/question"
)

// Limite de tentativas de login/cadastro por IP — proteção contra força
// bruta. 5 tentativas por 15 minutos é generoso o bastante para um usuário
// real errar a senha algumas vezes, apertado o bastante para tornar um
// ataque de dicionário impraticável.
const (
	authRateLimitMax    = 5
	authRateLimitWindow = 15 * time.Minute
)

// main é o único lugar que conhece todos os domínios. Ele monta as
// dependências à mão (repository -> service -> handler) e as passa adiante.
//
// Nota Go vs. Django: não há INSTALLED_APPS nem autodiscovery. A montagem é
// explícita e visível num arquivo só — mais verboso, mas dá para ler o
// programa inteiro de cima a baixo sem magia.
func main() {
	cfg, err := platform.LoadConfig()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	db, err := platform.OpenDB(cfg)
	if err != nil {
		log.Fatalf("banco: %v", err)
	}

	// A ordem da montagem segue a direção das dependências:
	// auth e catalog não dependem de mais nada; catalog <- question <- flashcard <- dashboard.
	authSvc := auth.NewService(auth.NewRepository(db), cfg.JWTSecret)
	catalogSvc := catalog.NewService(catalog.NewRepository(db))
	questionSvc := question.NewService(question.NewRepository(db), catalogSvc)
	flashcardSvc := flashcard.NewService(flashcard.NewRepository(db), catalogSvc, questionSvc)
	dashboardSvc := dashboard.NewService(catalogSvc, questionSvc, flashcardSvc)

	authHandler := auth.NewHandler(authSvc)

	router := platform.NewRouter(cfg)

	// Cadastro e login: alvo de força bruta de senha, rate limit por IP.
	rateLimited := router.Group("/api")
	rateLimited.Use(platform.RateLimit(authRateLimitMax, authRateLimitWindow))
	authHandler.RegisterRateLimitedRoutes(rateLimited)

	// Refresh e logout: a credencial é o próprio refresh token no corpo, sem
	// rate limit por IP (não são alvo de força bruta de senha).
	public := router.Group("/api")
	authHandler.RegisterPublicRoutes(public)

	// Identidade: só exige token válido, não premium. "Quem sou eu" precisa
	// responder mesmo para uma conta free (inclusive um admin free logo após
	// o bootstrap) — senão ninguém consegue nem descobrir o próprio
	// is_admin/plan sem já ter acesso ao que essa descoberta desbloqueia.
	identity := router.Group("/api")
	identity.Use(platform.RequireAuth(cfg))
	authHandler.RegisterProtectedRoutes(identity)

	// Rotas de conteúdo/estudo: exigem token válido E plano premium. Free
	// autentica com sucesso mas recebe 403 em qualquer coisa aqui dentro — é
	// assim "por enquanto", até existir cobrança de verdade.
	protected := router.Group("/api")
	protected.Use(platform.RequireAuth(cfg), platform.RequirePremium())
	catalog.NewHandler(catalogSvc).RegisterRoutes(protected)
	question.NewHandler(questionSvc).RegisterRoutes(protected)
	flashcard.NewHandler(flashcardSvc).RegisterRoutes(protected)
	dashboard.NewHandler(dashboardSvc).RegisterRoutes(protected)

	// Rota de bootstrap: promove uma conta a premium ou concede o primeiro
	// papel de admin. Protegida por um segredo de ambiente, não por login de
	// usuário nenhum — uso esperado é raro, tipicamente uma vez (o primeiro
	// admin), ou como caminho de emergência se o painel abaixo ficar
	// inacessível por algum motivo.
	admin := router.Group("/api/admin")
	admin.Use(platform.RequireAdminSecret(cfg))
	authHandler.RegisterAdminRoutes(admin)

	// Painel administrativo de verdade: qualquer conta com is_admin=true usa
	// isto pelo próprio login, sem precisar do segredo acima. Não passa por
	// RequirePremium de propósito (ver RequireAdminRole).
	adminPanel := router.Group("/api")
	adminPanel.Use(platform.RequireAuth(cfg), platform.RequireAdminRole())
	authHandler.RegisterAdminPanelRoutes(adminPanel)

	log.Printf("central de estudos ouvindo em :%s (%s)", cfg.Port, cfg.Env)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("servidor: %v", err)
	}
}
