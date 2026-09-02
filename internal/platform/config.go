package platform

import (
	"fmt"
	"os"
	"strings"
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
	// CORSOrigins aceita mais de uma origem (separadas por vírgula) — em dev,
	// o mesmo Next.js costuma ser acessível por mais de uma origem
	// (localhost:3000 e 127.0.0.1:3000 não são a mesma origem para o
	// navegador, mesmo apontando pro mesmo servidor; um só valor fixo aqui já
	// causou "Load failed" no fetch de quem abriu pela origem que não batia).
	CORSOrigins []string
	// JWTSecret assina e valida os access tokens. AdminSecret protege as
	// rotas /api/admin/*. Os dois exigem no mínimo 32 bytes — abaixo disso
	// um segredo é curto o bastante para ser adivinhado por força bruta em
	// tempo viável, então nem deixamos o servidor subir.
	JWTSecret   string
	AdminSecret string
}

const minSecretLen = 32

func LoadConfig() (Config, error) {
	cfg := Config{
		Port:        env("PORT", "8080"),
		DatabaseURL: os.Getenv("DATABASE_URL"),
		Env:         env("APP_ENV", "development"),
		// O PWA em Next.js roda em outra origem, então CORS não é opcional.
		CORSOrigins: splitOrigins(env("CORS_ORIGIN", "http://localhost:3000,http://127.0.0.1:3000")),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		AdminSecret: os.Getenv("ADMIN_SECRET"),
	}

	// Em Go erros são valores retornados, não exceções lançadas. Quem chama
	// decide o que fazer — aqui o main aborta, mas um teste poderia ignorar.
	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("DATABASE_URL não definida")
	}
	if len(cfg.JWTSecret) < minSecretLen {
		return Config{}, fmt.Errorf("JWT_SECRET precisa ter ao menos %d caracteres", minSecretLen)
	}
	if len(cfg.AdminSecret) < minSecretLen {
		return Config{}, fmt.Errorf("ADMIN_SECRET precisa ter ao menos %d caracteres", minSecretLen)
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

func splitOrigins(raw string) []string {
	parts := strings.Split(raw, ",")
	origins := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			origins = append(origins, p)
		}
	}
	return origins
}
