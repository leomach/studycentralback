package main

import (
	"log"

	"github.com/leomach/studycentralback/internal/catalog"
	"github.com/leomach/studycentralback/internal/dashboard"
	"github.com/leomach/studycentralback/internal/flashcard"
	"github.com/leomach/studycentralback/internal/platform"
	"github.com/leomach/studycentralback/internal/question"
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
	// catalog <- question <- flashcard <- dashboard.
	catalogSvc := catalog.NewService(catalog.NewRepository(db))
	questionSvc := question.NewService(question.NewRepository(db), catalogSvc)
	flashcardSvc := flashcard.NewService(flashcard.NewRepository(db), catalogSvc, questionSvc)
	dashboardSvc := dashboard.NewService(catalogSvc, questionSvc, flashcardSvc)

	router := platform.NewRouter(cfg)
	api := router.Group("/api")
	catalog.NewHandler(catalogSvc).RegisterRoutes(api)
	question.NewHandler(questionSvc).RegisterRoutes(api)
	flashcard.NewHandler(flashcardSvc).RegisterRoutes(api)
	dashboard.NewHandler(dashboardSvc).RegisterRoutes(api)

	log.Printf("central de estudos ouvindo em :%s (%s)", cfg.Port, cfg.Env)
	if err := router.Run(":" + cfg.Port); err != nil {
		log.Fatalf("servidor: %v", err)
	}
}
