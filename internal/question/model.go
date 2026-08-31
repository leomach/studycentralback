// Package question guarda as questões de prova e as tentativas de resposta.
// Depende apenas de catalog (por ID, sem importar o pacote).
package question

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// Format distingue o estilo da questão. String simples em vez de enum: Go não
// tem enums de verdade, o idiomático é um tipo nomeado sobre string com
// constantes — legível no banco e no JSON, sem tabela de lookup.
type Format string

const (
	FormatMultipleChoice Format = "multipla_escolha"
	FormatRightWrong     Format = "certo_errado"
)

func (f Format) Valid() bool {
	return f == FormatMultipleChoice || f == FormatRightWrong
}

// Confidence é o campo crítico do produto: como o estudo se sentiu ao
// responder. Distingue acerto sólido de acerto por sorte.
type Confidence string

const (
	ConfidenceCertain Confidence = "certeza"
	ConfidenceDoubt   Confidence = "duvida"
	ConfidenceGuess   Confidence = "chute"
)

func (c Confidence) Valid() bool {
	switch c {
	case ConfidenceCertain, ConfidenceDoubt, ConfidenceGuess:
		return true
	}
	return false
}

type Alternative struct {
	Key  string `json:"key"` // "a", "b", "c"...
	Text string `json:"text"`
}

// Alternatives é salvo como jsonb. Implementar Valuer/Scanner é como se ensina
// ao driver a converter um tipo Go em coluna e de volta — o equivalente manual
// ao JSONField do Django.
type Alternatives []Alternative

func (a Alternatives) Value() (driver.Value, error) {
	if a == nil {
		return "[]", nil
	}
	b, err := json.Marshal(a)
	return string(b), err
}

func (a *Alternatives) Scan(src any) error {
	switch v := src.(type) {
	case nil:
		*a = nil
		return nil
	case []byte:
		return json.Unmarshal(v, a)
	case string:
		return json.Unmarshal([]byte(v), a)
	default:
		return fmt.Errorf("não sei ler %T como Alternatives", src)
	}
}

type Question struct {
	ID            uint         `gorm:"primaryKey" json:"id"`
	UserID        uint         `json:"-"`
	SubjectID     uint         `json:"subject_id"`
	BancaID       *uint        `json:"banca_id,omitempty"`
	ExamID        *uint        `json:"exam_id,omitempty"`
	Format        Format       `json:"format"`
	Statement     string       `json:"statement"`
	Alternatives  Alternatives `gorm:"type:jsonb" json:"alternatives"`
	CorrectAnswer string       `json:"correct_answer"`
	CreatedAt     time.Time    `json:"created_at"`
	UpdatedAt     time.Time    `json:"updated_at"`
}

type Attempt struct {
	ID         uint       `gorm:"primaryKey" json:"id"`
	UserID     uint       `json:"-"`
	QuestionID uint       `json:"question_id"`
	Answer     string     `json:"answer"`
	IsCorrect  bool       `json:"is_correct"`
	Confidence Confidence `json:"confidence"`
	CreatedAt  time.Time  `json:"created_at"`
}
