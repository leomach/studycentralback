package question

import (
	"time"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

type ListFilter struct {
	SubjectID uint
	BancaID   uint
	ExamID    uint
	Format    Format
	Limit     int
	Offset    int
}

// applyFilters isola o WHERE compartilhado por List e Count: os dois
// precisam enxergar exatamente o mesmo recorte, senão o total paginado
// mostrado na UI não bate com o que List devolve.
func (r *Repository) applyFilters(q *gorm.DB, f ListFilter) *gorm.DB {
	if f.SubjectID != 0 {
		q = q.Where("subject_id = ?", f.SubjectID)
	}
	if f.BancaID != 0 {
		q = q.Where("banca_id = ?", f.BancaID)
	}
	if f.ExamID != 0 {
		q = q.Where("exam_id = ?", f.ExamID)
	}
	if f.Format != "" {
		q = q.Where("format = ?", f.Format)
	}
	return q
}

func (r *Repository) List(f ListFilter) ([]Question, error) {
	q := r.applyFilters(r.db.Model(&Question{}), f)
	if f.Limit > 0 {
		q = q.Limit(f.Limit)
	}
	if f.Offset > 0 {
		q = q.Offset(f.Offset)
	}

	var questions []Question
	err := q.Order("id desc").Find(&questions).Error
	return questions, err
}

// Count devolve quantas questões batem com o filtro, sem limit/offset — é o
// que permite a UI saber quanto falta carregar ("carregar mais").
func (r *Repository) Count(f ListFilter) (int64, error) {
	var n int64
	err := r.applyFilters(r.db.Model(&Question{}), f).Count(&n).Error
	return n, err
}

func (r *Repository) FindByID(id uint) (Question, error) {
	var question Question
	err := r.db.First(&question, id).Error
	return question, err
}

func (r *Repository) Create(q *Question) error { return r.db.Create(q).Error }

func (r *Repository) Update(q *Question, fields map[string]any) error {
	return r.db.Model(q).Updates(fields).Error
}

func (r *Repository) Delete(id uint) (int64, error) {
	res := r.db.Delete(&Question{}, id)
	return res.RowsAffected, res.Error
}

func (r *Repository) CreateAttempt(a *Attempt) error { return r.db.Create(a).Error }

func (r *Repository) FindAttemptByClientID(userID uint, clientID string) (Attempt, error) {
	var a Attempt
	err := r.db.Where("user_id = ? AND client_id = ?", userID, clientID).First(&a).Error
	return a, err
}

// SubjectStat alimenta o dashboard e a fila do dia: quantas tentativas e
// quantos acertos por eixo temático.
type SubjectStat struct {
	SubjectID uint `json:"subject_id"`
	Attempts  int  `json:"attempts"`
	Correct   int  `json:"correct"`
}

func (r *Repository) SubjectStats(userID uint) ([]SubjectStat, error) {
	var stats []SubjectStat
	err := r.db.
		Table("attempts").
		Select("questions.subject_id AS subject_id, COUNT(*) AS attempts, "+
			"COUNT(*) FILTER (WHERE attempts.is_correct) AS correct").
		Joins("JOIN questions ON questions.id = attempts.question_id").
		Where("attempts.user_id = ?", userID).
		Group("questions.subject_id").
		Scan(&stats).Error
	return stats, err
}

// QueueCandidate é a questão como a fila do dia a enxerga: o que a
// priorização precisa (eixo, tentativas) mais o conteúdo que o app vai
// mostrar. Levar o enunciado junto é o que permite baixar a fila inteira num
// request só e estudar sem rede depois.
type QueueCandidate struct {
	ID            uint         `json:"id"`
	SubjectID     uint         `json:"subject_id"`
	BancaID       *uint        `json:"banca_id,omitempty"`
	ExamID        *uint        `json:"exam_id,omitempty"`
	Attempts      int          `json:"attempts"`
	Format        Format       `json:"format"`
	Statement     string       `json:"statement"`
	Alternatives  Alternatives `json:"alternatives"`
	CorrectAnswer string       `json:"correct_answer"`
}

// Candidates traz TODAS as questões compartilhadas (não só as de um usuário),
// com a contagem de tentativas DAQUELE usuário em cada uma — "nunca
// respondidas [por mim]" primeiro. O corte fino de prioridade é do
// BuildQueue, não daqui.
func (r *Repository) Candidates(userID uint, limit int) ([]QueueCandidate, error) {
	q := r.db.
		Table("questions").
		Select("questions.id, questions.subject_id, questions.banca_id, questions.exam_id, "+
			"questions.format, questions.statement, questions.alternatives, "+
			"questions.correct_answer, COUNT(attempts.id) AS attempts").
		Joins("LEFT JOIN attempts ON attempts.question_id = questions.id AND attempts.user_id = ?", userID).
		Group("questions.id, questions.subject_id, questions.banca_id, questions.exam_id, " +
			"questions.format, questions.statement, questions.alternatives, " +
			"questions.correct_answer").
		Order("attempts, questions.id")
	if limit > 0 {
		q = q.Limit(limit)
	}

	var cands []QueueCandidate
	err := q.Scan(&cands).Error
	return cands, err
}

// ConfidenceStat cruza certeza declarada com acerto real — o dado mais
// revelador do sistema: acerto no chute e erro na certeza.
type ConfidenceStat struct {
	Confidence Confidence `json:"confidence"`
	Attempts   int        `json:"attempts"`
	Correct    int        `json:"correct"`
}

func (r *Repository) ConfidenceStats(userID uint) ([]ConfidenceStat, error) {
	var stats []ConfidenceStat
	err := r.db.
		Table("attempts").
		Select("confidence, COUNT(*) AS attempts, COUNT(*) FILTER (WHERE is_correct) AS correct").
		Where("user_id = ?", userID).
		Group("confidence").
		Scan(&stats).Error
	return stats, err
}

// ExamStat é o desempenho por concurso — útil para saber se o estilo de uma
// prova específica está dominado.
type ExamStat struct {
	ExamID   uint `json:"exam_id"`
	Attempts int  `json:"attempts"`
	Correct  int  `json:"correct"`
}

func (r *Repository) ExamStats(userID uint) ([]ExamStat, error) {
	var stats []ExamStat
	err := r.db.
		Table("attempts").
		Select("questions.exam_id, COUNT(*) AS attempts, "+
			"COUNT(*) FILTER (WHERE attempts.is_correct) AS correct").
		Joins("JOIN questions ON questions.id = attempts.question_id").
		Where("attempts.user_id = ? AND questions.exam_id IS NOT NULL", userID).
		Group("questions.exam_id").
		Scan(&stats).Error
	return stats, err
}

// Volume conta as tentativas em janelas recentes: mede constância, que num
// estudo de micro-sessões importa mais do que volume absoluto.
type Volume struct {
	Last7Days  int64 `json:"last_7_days"`
	Last30Days int64 `json:"last_30_days"`
}

func (r *Repository) AttemptVolume(userID uint, now time.Time) (Volume, error) {
	var v Volume

	count := func(days int) (int64, error) {
		var n int64
		err := r.db.Model(&Attempt{}).
			Where("user_id = ? AND created_at >= ?", userID, now.AddDate(0, 0, -days)).
			Count(&n).Error
		return n, err
	}

	var err error
	if v.Last7Days, err = count(7); err != nil {
		return Volume{}, err
	}
	if v.Last30Days, err = count(30); err != nil {
		return Volume{}, err
	}
	return v, nil
}
