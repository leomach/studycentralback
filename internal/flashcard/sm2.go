package flashcard

import (
	"math"
	"time"
)

// Grade é a autoavaliação após ver a resposta do card.
type Grade int

const (
	GradeAgain Grade = 1 // errei
	GradeHard  Grade = 2 // difícil
	GradeGood  Grade = 3 // bom
	GradeEasy  Grade = 4 // fácil
)

func (g Grade) Valid() bool { return g >= GradeAgain && g <= GradeEasy }

const (
	DefaultEase = 2.5 // ease inicial do SM-2 clássico
	MinEase     = 1.3 // piso: abaixo disso o card entraria em loop infinito
)

// State é o estado de agendamento de um card. Espelha flashcard_reviews, mas
// é um tipo próprio, sem tags de banco: assim o SM-2 fica testável sem Gorm.
type State struct {
	IntervalDays int
	EaseFactor   float64
	Reps         int
	Lapses       int
	DueDate      time.Time
}

// NewState é o estado de um card nunca revisado — já vencido, para entrar na
// fila de hoje.
func NewState(now time.Time) State {
	return State{
		IntervalDays: 0,
		EaseFactor:   DefaultEase,
		Reps:         0,
		Lapses:       0,
		DueDate:      day(now),
	}
}

// Schedule aplica o SM-2 e devolve o PRÓXIMO estado do card.
//
// Nota Go: a função é pura — recebe State por valor, não muda nada fora dela e
// devolve um State novo. Não há self.save() escondido; quem chama decide
// persistir. É isso que permite testar o algoritmo inteiro sem banco.
func Schedule(s State, g Grade, now time.Time) State {
	if !g.Valid() {
		return s
	}

	if s.EaseFactor == 0 {
		s.EaseFactor = DefaultEase
	}

	// Base mínima de 1 dia: um card novo tem intervalo 0 e multiplicar zero
	// deixaria o card preso para sempre no mesmo dia.
	base := float64(max(s.IntervalDays, 1))

	switch g {
	case GradeAgain:
		s.Reps = 0
		s.Lapses++
		s.IntervalDays = 1
		s.EaseFactor -= 0.2
	case GradeHard:
		s.Reps++
		s.IntervalDays = roundDays(base * 1.2)
	case GradeGood:
		s.Reps++
		s.IntervalDays = roundDays(base * s.EaseFactor)
	case GradeEasy:
		s.Reps++
		s.IntervalDays = roundDays(base * s.EaseFactor * 1.3)
		s.EaseFactor += 0.1
	}

	if s.EaseFactor < MinEase {
		s.EaseFactor = MinEase
	}
	s.DueDate = day(now).AddDate(0, 0, s.IntervalDays)

	return s
}

// Overdue diz quantos dias o card está atrasado (0 se ainda não venceu). É o
// primeiro critério de prioridade da fila do dia.
func (s State) Overdue(now time.Time) int {
	diff := int(day(now).Sub(day(s.DueDate)).Hours() / 24)
	return max(diff, 0)
}

func (s State) IsDue(now time.Time) bool { return !day(s.DueDate).After(day(now)) }

func roundDays(v float64) int { return max(int(math.Round(v)), 1) }

// day zera a hora: o agendamento é por dia de calendário, não por instante.
func day(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}
