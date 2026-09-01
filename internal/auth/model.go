// Package auth guarda contas de usuário e as credenciais de sessão (refresh
// tokens). É o único domínio, junto de platform, que não depende de nenhum
// outro — fica no mesmo nível de catalog na cadeia de dependência.
package auth

import "time"

type Plan string

const (
	PlanFree    Plan = "free"
	PlanPremium Plan = "premium"
)

func (p Plan) Valid() bool { return p == PlanFree || p == PlanPremium }

type User struct {
	ID    uint   `gorm:"primaryKey" json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	// PasswordHash nunca aparece em JSON (json:"-"): mesmo sendo um hash e
	// não a senha em si, não há razão para esse campo sair da API.
	PasswordHash string    `json:"-"`
	Plan         Plan      `json:"plan"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// RefreshToken é a credencial de sessão de longa duração. Só o hash do token
// fica gravado (TokenHash) — o valor em texto puro existe só no momento em
// que é emitido para o cliente, nunca depois disso no servidor.
type RefreshToken struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	UserID    uint       `json:"-"`
	TokenHash string     `gorm:"uniqueIndex" json:"-"`
	ExpiresAt time.Time  `json:"-"`
	RevokedAt *time.Time `json:"-"`
	CreatedAt time.Time  `json:"-"`
}

func (RefreshToken) TableName() string { return "refresh_tokens" }
