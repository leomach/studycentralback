// Package flashcard guarda os cards e o estado de repetição espaçada (SM-2).
// O algoritmo em si vive em sm2.go, sem tocar HTTP nem banco.
package flashcard

import "time"

type Kind string

const (
	KindQuestionAnswer Kind = "pergunta_resposta"
	KindSummary        Kind = "resumo"
)

func (k Kind) Valid() bool {
	return k == KindQuestionAnswer || k == KindSummary
}

type Flashcard struct {
	ID     uint `gorm:"primaryKey" json:"id"`
	UserID uint `json:"-"`

	SubjectID uint   `json:"subject_id"`
	Kind      Kind   `json:"kind"`
	Front     string `json:"front"`
	Back      string `json:"back"`

	// SourceQuestionID liga o card à questão que o originou (errei a questão,
	// virou card). Ponteiro porque a maioria dos cards não vem de questão.
	SourceQuestionID *uint `json:"source_question_id,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Review é a linha de estado do SM-2 no banco: uma por flashcard.
type Review struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	UserID       uint      `json:"-"`
	FlashcardID  uint      `gorm:"uniqueIndex" json:"flashcard_id"`
	DueDate      time.Time `json:"due_date"`
	IntervalDays int       `json:"interval_days"`
	EaseFactor   float64   `json:"ease_factor"`
	Reps         int       `json:"reps"`
	Lapses       int       `json:"lapses"`
	LastGrade    int       `json:"last_grade"`
	// LastClientID é a chave de idempotência do outbox offline: como esta
	// tabela guarda só o estado ATUAL do card (uma linha por flashcard, não um
	// log), retentativa se detecta comparando com o último client_id aplicado
	// — não por índice único, que aqui não faria sentido.
	LastClientID string    `json:"-" gorm:"column:last_client_id"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (Review) TableName() string { return "flashcard_reviews" }

// WithReview é o card como a listagem devolve: o front computa localmente se
// o card está vencido/em aprendizado/maduro a partir de due_date/interval_days,
// então o estado do SM-2 precisa viajar junto em vez de ficar em outro
// endpoint. Review é ponteiro porque, em teoria, um card sem review não
// deveria existir (Create grava os dois juntos), mas nada impede o campo de
// faltar sem quebrar a serialização.
type WithReview struct {
	Flashcard
	Review *Review `json:"review,omitempty"`
}

// State converte a linha do banco no tipo puro que o SM-2 entende, e volta.
// Manter os dois separados é o que permite testar sm2.go sem Postgres.
func (r Review) State() State {
	return State{
		IntervalDays: r.IntervalDays,
		EaseFactor:   r.EaseFactor,
		Reps:         r.Reps,
		Lapses:       r.Lapses,
		DueDate:      r.DueDate,
	}
}

func (r *Review) applyState(s State, g Grade) {
	r.IntervalDays = s.IntervalDays
	r.EaseFactor = s.EaseFactor
	r.Reps = s.Reps
	r.Lapses = s.Lapses
	r.DueDate = s.DueDate
	r.LastGrade = int(g)
}

// Due é um card pronto para estudo, já com o estado junto. É o que a fila do
// dia consome.
//
// EaseFactor e IntervalDays viajam junto mesmo sem uso direto na priorização:
// é o que permite ao app calcular offline (sem consultar o servidor) o
// próximo intervalo de cada nota antes de o usuário escolher — o preview "3 d"
// nos botões de avaliação da sessão.
type Due struct {
	FlashcardID  uint      `json:"flashcard_id"`
	SubjectID    uint      `json:"subject_id"`
	Kind         Kind      `json:"kind"`
	Front        string    `json:"front"`
	Back         string    `json:"back"`
	DueDate      time.Time `json:"due_date"`
	IntervalDays int       `json:"interval_days"`
	EaseFactor   float64   `json:"ease_factor"`
	Lapses       int       `json:"lapses"`
	Reps         int       `json:"reps"`
}
