// Package catalog guarda o conteúdo de referência: eixos temáticos, bancas e
// concursos. É um domínio de conteúdo — nunca importa question, flashcard ou
// dashboard.
package catalog

import "time"

// Subject é um eixo temático. ParentID permite subeixos (ex: "Tributário" ->
// "Impostos federais"). É um ponteiro porque a raiz não tem pai — em Go o
// ponteiro nil é como se representa "coluna nula", já que uint zero-value é 0.
type Subject struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	UserID    uint      `json:"-"`
	ParentID  *uint     `json:"parent_id,omitempty"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Banca struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type Exam struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	BancaID   uint      `json:"banca_id"`
	Name      string    `json:"name"`
	Year      int       `json:"year"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (Banca) TableName() string { return "bancas" }
