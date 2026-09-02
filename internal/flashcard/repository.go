package flashcard

import (
	"time"

	"gorm.io/gorm"
)

type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

func (r *Repository) List(userID uint, subjectID uint, limit, offset int) ([]Flashcard, error) {
	q := r.db.Where("user_id = ?", userID)
	if subjectID != 0 {
		q = q.Where("subject_id = ?", subjectID)
	}
	if limit > 0 {
		q = q.Limit(limit)
	}
	if offset > 0 {
		q = q.Offset(offset)
	}

	var cards []Flashcard
	err := q.Order("id desc").Find(&cards).Error
	return cards, err
}

// Count devolve quantos cards do usuário batem com o filtro, sem
// limit/offset — mesmo propósito do Count em question/repository.go.
func (r *Repository) Count(userID uint, subjectID uint) (int64, error) {
	q := r.db.Model(&Flashcard{}).Where("user_id = ?", userID)
	if subjectID != 0 {
		q = q.Where("subject_id = ?", subjectID)
	}
	var n int64
	err := q.Count(&n).Error
	return n, err
}

// ReviewsByFlashcardID busca as reviews dos cards informados de uma vez (uma
// query, não uma por card) para a listagem embutir o estado de cada um.
func (r *Repository) ReviewsByFlashcardID(userID uint, flashcardIDs []uint) (map[uint]Review, error) {
	byID := make(map[uint]Review, len(flashcardIDs))
	if len(flashcardIDs) == 0 {
		return byID, nil
	}

	var reviews []Review
	err := r.db.Where("user_id = ? AND flashcard_id IN ?", userID, flashcardIDs).Find(&reviews).Error
	if err != nil {
		return nil, err
	}
	for _, rv := range reviews {
		byID[rv.FlashcardID] = rv
	}
	return byID, nil
}

// Create grava o card e o estado inicial do SM-2 na mesma transação: um card
// sem linha de review nunca apareceria na fila.
func (r *Repository) Create(card *Flashcard, now time.Time) error {
	return r.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(card).Error; err != nil {
			return err
		}

		s := NewState(now)
		review := Review{
			UserID:       card.UserID,
			FlashcardID:  card.ID,
			DueDate:      s.DueDate,
			IntervalDays: s.IntervalDays,
			EaseFactor:   s.EaseFactor,
		}
		return tx.Create(&review).Error
	})
}

func (r *Repository) FindByID(userID, id uint) (Flashcard, error) {
	var card Flashcard
	err := r.db.Where("user_id = ? AND id = ?", userID, id).First(&card).Error
	return card, err
}

func (r *Repository) Update(card *Flashcard, fields map[string]any) error {
	return r.db.Model(card).Updates(fields).Error
}

// Delete remove o card; a linha de flashcard_reviews vai junto pela foreign
// key ON DELETE CASCADE, sem precisar de código aqui.
func (r *Repository) Delete(userID, id uint) (int64, error) {
	res := r.db.Where("user_id = ? AND id = ?", userID, id).Delete(&Flashcard{})
	return res.RowsAffected, res.Error
}

func (r *Repository) FindReview(userID, flashcardID uint) (Review, error) {
	var review Review
	err := r.db.Where("user_id = ? AND flashcard_id = ?", userID, flashcardID).First(&review).Error
	return review, err
}

func (r *Repository) SaveReview(review *Review) error { return r.db.Save(review).Error }

// DueCards devolve os cards vencidos até `now`, mais atrasados primeiro.
func (r *Repository) DueCards(userID uint, now time.Time, limit int) ([]Due, error) {
	q := r.db.
		Table("flashcards").
		Select("flashcards.id AS flashcard_id, flashcards.subject_id, flashcards.kind, "+
			"flashcards.front, flashcards.back, flashcard_reviews.due_date, "+
			"flashcard_reviews.interval_days, flashcard_reviews.ease_factor, "+
			"flashcard_reviews.lapses, flashcard_reviews.reps").
		Joins("JOIN flashcard_reviews ON flashcard_reviews.flashcard_id = flashcards.id").
		Where("flashcards.user_id = ? AND flashcard_reviews.due_date <= ?", userID, now).
		Order("flashcard_reviews.due_date")
	if limit > 0 {
		q = q.Limit(limit)
	}

	var due []Due
	err := q.Scan(&due).Error
	return due, err
}

// Counts resume o estado do baralho. "Maduro" é o card cujo intervalo já
// passou de MatureIntervalDays: saiu do aprendizado e virou memória de longo
// prazo. É o número que mostra progresso real, não só esforço do dia.
type Counts struct {
	Due    int64 `json:"due"`
	Mature int64 `json:"mature"`
	Total  int64 `json:"total"`
}

// MatureIntervalDays segue a convenção do Anki: 21 dias.
const MatureIntervalDays = 21

func (r *Repository) CountDue(userID uint, now time.Time) (int64, error) {
	var n int64
	err := r.db.Model(&Review{}).
		Where("user_id = ? AND due_date <= ?", userID, now).
		Count(&n).Error
	return n, err
}

func (r *Repository) CountCards(userID uint, now time.Time) (Counts, error) {
	var c Counts

	count := func(scope *gorm.DB) (int64, error) {
		var n int64
		err := scope.Count(&n).Error
		return n, err
	}

	base := func() *gorm.DB { return r.db.Model(&Review{}).Where("user_id = ?", userID) }

	var err error
	if c.Due, err = count(base().Where("due_date <= ?", now)); err != nil {
		return Counts{}, err
	}
	if c.Mature, err = count(base().Where("interval_days >= ?", MatureIntervalDays)); err != nil {
		return Counts{}, err
	}
	if c.Total, err = count(base()); err != nil {
		return Counts{}, err
	}
	return c, nil
}
