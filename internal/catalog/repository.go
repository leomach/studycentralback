package catalog

import "gorm.io/gorm"

// Repository isola o acesso ao banco. Struct minúscula (privada) exposta por
// um construtor: quem usa só enxerga os métodos, nunca o *gorm.DB dentro.
type Repository struct {
	db *gorm.DB
}

func NewRepository(db *gorm.DB) *Repository { return &Repository{db: db} }

// --- subjects ---

func (r *Repository) ListSubjects(userID uint) ([]Subject, error) {
	var subjects []Subject
	err := r.db.Where("user_id = ?", userID).Order("name").Find(&subjects).Error
	return subjects, err
}

func (r *Repository) FindSubject(userID, id uint) (Subject, error) {
	var subject Subject
	err := r.db.Where("user_id = ? AND id = ?", userID, id).First(&subject).Error
	return subject, err
}

func (r *Repository) CreateSubject(s *Subject) error { return r.db.Create(s).Error }

// UpdateSubject aplica só os campos presentes no map. O Gorm também preenche
// o struct com o resultado, então ele volta atualizado para o handler.
func (r *Repository) UpdateSubject(s *Subject, fields map[string]any) error {
	return r.db.Model(s).Updates(fields).Error
}

func (r *Repository) DeleteSubject(userID, id uint) (int64, error) {
	res := r.db.Where("user_id = ? AND id = ?", userID, id).Delete(&Subject{})
	return res.RowsAffected, res.Error
}

func (r *Repository) SubjectExists(userID, id uint) (bool, error) {
	return r.exists(r.db.Model(&Subject{}).Where("user_id = ? AND id = ?", userID, id))
}

// --- bancas ---

func (r *Repository) ListBancas() ([]Banca, error) {
	var bancas []Banca
	err := r.db.Order("name").Find(&bancas).Error
	return bancas, err
}

func (r *Repository) FindBanca(id uint) (Banca, error) {
	var banca Banca
	err := r.db.First(&banca, id).Error
	return banca, err
}

func (r *Repository) CreateBanca(b *Banca) error { return r.db.Create(b).Error }

func (r *Repository) UpdateBanca(b *Banca, fields map[string]any) error {
	return r.db.Model(b).Updates(fields).Error
}

func (r *Repository) DeleteBanca(id uint) (int64, error) {
	res := r.db.Delete(&Banca{}, id)
	return res.RowsAffected, res.Error
}

func (r *Repository) BancaExists(id uint) (bool, error) {
	return r.exists(r.db.Model(&Banca{}).Where("id = ?", id))
}

// --- exams ---

func (r *Repository) ListExams() ([]Exam, error) {
	var exams []Exam
	err := r.db.Order("year desc, name").Find(&exams).Error
	return exams, err
}

func (r *Repository) FindExam(id uint) (Exam, error) {
	var exam Exam
	err := r.db.First(&exam, id).Error
	return exam, err
}

func (r *Repository) CreateExam(e *Exam) error { return r.db.Create(e).Error }

func (r *Repository) UpdateExam(e *Exam, fields map[string]any) error {
	return r.db.Model(e).Updates(fields).Error
}

func (r *Repository) DeleteExam(id uint) (int64, error) {
	res := r.db.Delete(&Exam{}, id)
	return res.RowsAffected, res.Error
}

func (r *Repository) ExamExists(id uint) (bool, error) {
	return r.exists(r.db.Model(&Exam{}).Where("id = ?", id))
}

func (r *Repository) exists(q *gorm.DB) (bool, error) {
	var n int64
	err := q.Count(&n).Error
	return n > 0, err
}
