package catalog

import (
	"strings"

	"github.com/leomach/studycentralback/internal/platform"
)

type Service struct {
	repo *Repository
}

func NewService(repo *Repository) *Service { return &Service{repo: repo} }

// --- subjects ---

func (s *Service) ListSubjects(userID uint) ([]Subject, error) {
	return s.repo.ListSubjects(userID)
}

func (s *Service) CreateSubject(userID uint, name string, parentID *uint) (Subject, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Subject{}, platform.Invalid("nome é obrigatório")
	}
	if parentID != nil {
		if err := s.requireSubject(userID, *parentID); err != nil {
			return Subject{}, err
		}
	}

	subject := Subject{UserID: userID, Name: name, ParentID: parentID}
	if err := s.repo.CreateSubject(&subject); err != nil {
		return Subject{}, err
	}
	return subject, nil
}

// SubjectPatch traz só o que muda. Ponteiro nil significa "campo não enviado";
// é assim que se faz um PATCH parcial em Go, já que não há um `partial=True`
// como no serializer do DRF.
type SubjectPatch struct {
	Name *string `json:"name"`
	// ParentID com 0 desvincula do pai (vira raiz); ausente não mexe.
	ParentID *uint `json:"parent_id"`
}

func (s *Service) UpdateSubject(userID, id uint, patch SubjectPatch) (Subject, error) {
	subject, err := s.repo.FindSubject(userID, id)
	if err != nil {
		return Subject{}, err
	}

	fields := map[string]any{}
	if patch.Name != nil {
		name := strings.TrimSpace(*patch.Name)
		if name == "" {
			return Subject{}, platform.Invalid("nome é obrigatório")
		}
		fields["name"] = name
	}
	if patch.ParentID != nil {
		switch {
		case *patch.ParentID == 0:
			fields["parent_id"] = nil
		case *patch.ParentID == id:
			return Subject{}, platform.Invalid("um eixo não pode ser pai de si mesmo")
		default:
			if err := s.requireSubject(userID, *patch.ParentID); err != nil {
				return Subject{}, err
			}
			fields["parent_id"] = *patch.ParentID
		}
	}
	if len(fields) == 0 {
		return subject, nil
	}

	if err := s.repo.UpdateSubject(&subject, fields); err != nil {
		return Subject{}, err
	}
	return subject, nil
}

func (s *Service) DeleteSubject(userID, id uint) error {
	rows, err := s.repo.DeleteSubject(userID, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return platform.NotFound("eixo temático não encontrado")
	}
	return nil
}

// --- bancas ---

func (s *Service) ListBancas() ([]Banca, error) { return s.repo.ListBancas() }

func (s *Service) CreateBanca(name string) (Banca, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Banca{}, platform.Invalid("nome é obrigatório")
	}

	banca := Banca{Name: name}
	if err := s.repo.CreateBanca(&banca); err != nil {
		return Banca{}, err
	}
	return banca, nil
}

func (s *Service) UpdateBanca(id uint, name *string) (Banca, error) {
	banca, err := s.repo.FindBanca(id)
	if err != nil {
		return Banca{}, err
	}
	if name == nil {
		return banca, nil
	}

	trimmed := strings.TrimSpace(*name)
	if trimmed == "" {
		return Banca{}, platform.Invalid("nome é obrigatório")
	}
	if err := s.repo.UpdateBanca(&banca, map[string]any{"name": trimmed}); err != nil {
		return Banca{}, err
	}
	return banca, nil
}

func (s *Service) DeleteBanca(id uint) error {
	rows, err := s.repo.DeleteBanca(id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return platform.NotFound("banca não encontrada")
	}
	return nil
}

// --- exams ---

func (s *Service) ListExams() ([]Exam, error) { return s.repo.ListExams() }

func (s *Service) CreateExam(name string, bancaID uint, year int) (Exam, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return Exam{}, platform.Invalid("nome é obrigatório")
	}
	if err := s.RequireBanca(bancaID); err != nil {
		return Exam{}, err
	}

	exam := Exam{Name: name, BancaID: bancaID, Year: year}
	if err := s.repo.CreateExam(&exam); err != nil {
		return Exam{}, err
	}
	return exam, nil
}

type ExamPatch struct {
	Name    *string `json:"name"`
	BancaID *uint   `json:"banca_id"`
	Year    *int    `json:"year"`
}

func (s *Service) UpdateExam(id uint, patch ExamPatch) (Exam, error) {
	exam, err := s.repo.FindExam(id)
	if err != nil {
		return Exam{}, err
	}

	fields := map[string]any{}
	if patch.Name != nil {
		name := strings.TrimSpace(*patch.Name)
		if name == "" {
			return Exam{}, platform.Invalid("nome é obrigatório")
		}
		fields["name"] = name
	}
	if patch.BancaID != nil {
		if err := s.RequireBanca(*patch.BancaID); err != nil {
			return Exam{}, err
		}
		fields["banca_id"] = *patch.BancaID
	}
	if patch.Year != nil {
		fields["year"] = *patch.Year
	}
	if len(fields) == 0 {
		return exam, nil
	}

	if err := s.repo.UpdateExam(&exam, fields); err != nil {
		return Exam{}, err
	}
	return exam, nil
}

func (s *Service) DeleteExam(id uint) error {
	rows, err := s.repo.DeleteExam(id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return platform.NotFound("concurso não encontrado")
	}
	return nil
}

// --- validações usadas pelos outros domínios ---
//
// question e flashcard chamam estes métodos antes de gravar, para que um id
// inexistente vire 400 com mensagem clara em vez de erro de foreign key.

func (s *Service) RequireSubject(userID, id uint) error { return s.requireSubject(userID, id) }

func (s *Service) RequireBanca(id uint) error {
	ok, err := s.repo.BancaExists(id)
	if err != nil {
		return err
	}
	if !ok {
		return platform.Invalid("banca_id não existe")
	}
	return nil
}

func (s *Service) RequireExam(id uint) error {
	ok, err := s.repo.ExamExists(id)
	if err != nil {
		return err
	}
	if !ok {
		return platform.Invalid("exam_id não existe")
	}
	return nil
}

func (s *Service) requireSubject(userID, id uint) error {
	ok, err := s.repo.SubjectExists(userID, id)
	if err != nil {
		return err
	}
	if !ok {
		return platform.Invalid("subject_id não existe")
	}
	return nil
}
