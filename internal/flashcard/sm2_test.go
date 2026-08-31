package flashcard

import (
	"testing"
	"time"
)

var refDate = time.Date(2026, 8, 31, 9, 0, 0, 0, time.UTC)

func TestScheduleCardNovo(t *testing.T) {
	// Um card novo (interval 0) precisa sair do zero em qualquer nota.
	casos := []struct {
		grade        Grade
		wantInterval int
		wantEase     float64
	}{
		{GradeAgain, 1, 2.3},
		{GradeHard, 1, 2.5}, // 1 * 1.2 = 1.2 -> 1
		{GradeGood, 3, 2.5}, // 1 * 2.5 = 2.5 -> 3
		{GradeEasy, 3, 2.6}, // 1 * 2.5 * 1.3 = 3.25 -> 3
	}

	for _, c := range casos {
		got := Schedule(NewState(refDate), c.grade, refDate)
		if got.IntervalDays != c.wantInterval {
			t.Errorf("grade %d: interval = %d, quer %d", c.grade, got.IntervalDays, c.wantInterval)
		}
		if !quase(got.EaseFactor, c.wantEase) {
			t.Errorf("grade %d: ease = %.2f, quer %.2f", c.grade, got.EaseFactor, c.wantEase)
		}
	}
}

func TestScheduleErroReiniciaEContaLapse(t *testing.T) {
	s := State{IntervalDays: 30, EaseFactor: 2.5, Reps: 5, Lapses: 1, DueDate: refDate}

	got := Schedule(s, GradeAgain, refDate)

	if got.Reps != 0 {
		t.Errorf("reps = %d, quer 0", got.Reps)
	}
	if got.Lapses != 2 {
		t.Errorf("lapses = %d, quer 2", got.Lapses)
	}
	if got.IntervalDays != 1 {
		t.Errorf("interval = %d, quer 1", got.IntervalDays)
	}
	if !quase(got.EaseFactor, 2.3) {
		t.Errorf("ease = %.2f, quer 2.30", got.EaseFactor)
	}
}

func TestScheduleEaseNaoDesceAbaixoDoPiso(t *testing.T) {
	s := State{IntervalDays: 5, EaseFactor: 1.4, Reps: 3, DueDate: refDate}

	got := Schedule(s, GradeAgain, refDate)

	if got.EaseFactor != MinEase {
		t.Errorf("ease = %.2f, quer o piso %.2f", got.EaseFactor, MinEase)
	}
}

func TestScheduleCrescimentoDosIntervalos(t *testing.T) {
	s := State{IntervalDays: 10, EaseFactor: 2.5, Reps: 3, DueDate: refDate}

	if got := Schedule(s, GradeHard, refDate); got.IntervalDays != 12 { // 10 * 1.2
		t.Errorf("hard: interval = %d, quer 12", got.IntervalDays)
	}
	if got := Schedule(s, GradeGood, refDate); got.IntervalDays != 25 { // 10 * 2.5
		t.Errorf("good: interval = %d, quer 25", got.IntervalDays)
	}
	if got := Schedule(s, GradeEasy, refDate); got.IntervalDays != 33 { // 10 * 2.5 * 1.3
		t.Errorf("easy: interval = %d, quer 33", got.IntervalDays)
	}
}

func TestScheduleAgendaDueDateAPartirDeHoje(t *testing.T) {
	// Card atrasado: o próximo vencimento conta de hoje, não da data antiga.
	atrasado := refDate.AddDate(0, 0, -7)
	s := State{IntervalDays: 4, EaseFactor: 2.5, Reps: 2, DueDate: atrasado}

	got := Schedule(s, GradeGood, refDate)

	want := day(refDate).AddDate(0, 0, got.IntervalDays)
	if !got.DueDate.Equal(want) {
		t.Errorf("due = %v, quer %v", got.DueDate, want)
	}
}

func TestScheduleGradeInvalidaNaoMudaNada(t *testing.T) {
	s := State{IntervalDays: 7, EaseFactor: 2.5, Reps: 2, DueDate: refDate}

	if got := Schedule(s, Grade(9), refDate); got != s {
		t.Errorf("estado mudou com grade inválida: %+v", got)
	}
}

func TestOverdue(t *testing.T) {
	casos := []struct {
		due  time.Time
		want int
	}{
		{refDate.AddDate(0, 0, -3), 3},
		{refDate, 0},
		{refDate.AddDate(0, 0, 5), 0}, // futuro não é atraso negativo
	}

	for _, c := range casos {
		s := State{DueDate: c.due}
		if got := s.Overdue(refDate); got != c.want {
			t.Errorf("due %v: overdue = %d, quer %d", c.due, got, c.want)
		}
	}
}

func quase(a, b float64) bool {
	d := a - b
	return d < 0.0001 && d > -0.0001
}
