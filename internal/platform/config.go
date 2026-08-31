package platform

import (
	"fmt"
	"os"
)

// Config carrega tudo que a aplicação precisa do ambiente.
//
// Nota Go vs. Django: não existe um `settings.py` global importável de
// qualquer lugar. A configuração é um valor comum, criado uma vez no main e
// passado explicitamente para quem precisa. Isso evita estado global e torna
// os testes triviais (basta construir outro Config).
type Config struct {
	Port        string
	DatabaseURL string
	Env         string
	CORSOrigin  string
}

func LoadConfig() (Config, error) {
	cfg := Config{
		Port:        env("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Env:         env("APP_ENV", "development"),
		// O PWA em Next.js roda em outra origem, então CORS não é opcional.
		CORSOrigin: env("CORS_ORIGIN", "http://localhost:3000"),
	}

	// Em Go erros são valores retornados, não exceções lançadas. Quem chama
	// decide o que fazer — aqui o main aborta, mas um teste poderia ignorar.
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL não definida")
	}

	return cfg, nil
}

func (c Config) IsDevelopment() bool { return c.Env == "development" }

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
