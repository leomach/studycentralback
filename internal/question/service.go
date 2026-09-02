package question

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/leomach/studycentralback/internal/catalog"
	"github.com/leomach/studycentralback/internal/platform"
)

// Service valida contra o catalog antes de gravar, para que um subject_id
// inexistente vire 400 com mensagem legível em vez de estourar a foreign key
// e vazar erro de banco para o app.
type Service struct {
	repo    *Repository
	catalog *catalog.Service
}

func NewService(repo *Repository, catalogSvc *catalog.Service) *Service {
	return &Service{repo: repo, catalog: catalogSvc}
}

// List devolve a página pedida mais o total que bate com o filtro (sem
// limit/offset), para a UI saber quanto falta e oferecer "carregar mais".
func (s *Service) List(f ListFilter) ([]Question, int64, error) {
	items, err := s.repo.List(f)
	if err != nil {
		return nil, 0, err
	}
	total, err := s.repo.Count(f)
	if err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

type NewQuestion struct {
	SubjectID     uint
	BancaID       *uint
	ExamID        *uint
	Format        Format
	Statement     string
	Alternatives  Alternatives
	CorrectAnswer string
}

func (s *Service) Create(in NewQuestion) (Question, error) {
	if strings.TrimSpace(in.Statement) == "" {
		return Question{}, platform.Invalid("enunciado é obrigatório")
	}
	if strings.TrimSpace(in.CorrectAnswer) == "" {
		return Question{}, platform.Invalid("resposta correta é obrigatória")
	}
	if in.Format == "" {
		in.Format = FormatMultipleChoice
	}
	if !in.Format.Valid() {
		return Question{}, platform.Invalid("format deve ser multipla_escolha ou certo_errado")
	}
	if err := s.validateRefs(&in.SubjectID, in.BancaID, in.ExamID); err != nil {
		return Question{}, err
	}

	q := Question{
		SubjectID:     in.SubjectID,
		BancaID:       in.BancaID,
		ExamID:        in.ExamID,
		Format:        in.Format,
		Statement:     in.Statement,
		Alternatives:  in.Alternatives,
		CorrectAnswer: in.CorrectAnswer,
	}
	if err := s.repo.Create(&q); err != nil {
		return Question{}, err
	}
	return q, nil
}

// Answer registra a tentativa. A correção é feita no servidor, comparando com
// a resposta guardada — o cliente nunca diz se acertou.
//
// clientID é a chave de idempotência do outbox offline do PWA: se a mesma
// tentativa chegar duas vezes (sincronização interrompida e reenviada), a
// segunda chamada devolve a tentativa já gravada em vez de duplicá-la.
func (s *Service) Answer(userID, questionID uint, clientID, answer string, confidence Confidence) (Attempt, error) {
	if strings.TrimSpace(clientID) == "" {
		return Attempt{}, platform.Invalid("client_id é obrigatório")
	}
	if !confidence.Valid() {
		return Attempt{}, platform.Invalid("confidence deve ser certeza, duvida ou chute")
	}

	if existing, err := s.repo.FindAttemptByClientID(userID, clientID); err == nil {
		return existing, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return Attempt{}, err
	}

	q, err := s.repo.FindByID(questionID)
	if err != nil {
		return Attempt{}, err
	}

	attempt := Attempt{
		UserID:     userID,
		QuestionID: q.ID,
		Answer:     answer,
		IsCorrect:  strings.EqualFold(strings.TrimSpace(answer), strings.TrimSpace(q.CorrectAnswer)),
		Confidence: confidence,
		ClientID:   clientID,
	}
	if err := s.repo.CreateAttempt(&attempt); err != nil {
		// Corrida rara entre o SELECT acima e este INSERT (ex.: duas abas
		// sincronizando ao mesmo tempo): o índice único pegou primeiro:
		// devolve a linha que já está lá em vez de propagar o erro.
		if platform.IsUniqueViolation(err) {
			return s.repo.FindAttemptByClientID(userID, clientID)
		}
		return Attempt{}, err
	}
	return attempt, nil
}

func (s *Service) SubjectStats(userID uint) ([]SubjectStat, error) {
	return s.repo.SubjectStats(userID)
}

func (s *Service) Candidates(userID uint, limit int) ([]QueueCandidate, error) {
	return s.repo.Candidates(userID, limit)
}

func (s *Service) ConfidenceStats(userID uint) ([]ConfidenceStat, error) {
	return s.repo.ConfidenceStats(userID)
}

func (s *Service) ExamStats(userID uint) ([]ExamStat, error) {
	return s.repo.ExamStats(userID)
}

func (s *Service) AttemptVolume(userID uint, now time.Time) (Volume, error) {
	return s.repo.AttemptVolume(userID, now)
}

func (s *Service) FindByID(id uint) (Question, error) {
	return s.repo.FindByID(id)
}

// QuestionPatch traz só o que muda; nil significa "não enviado".
// BancaID e ExamID com 0 desvinculam a questão da banca/concurso.
type QuestionPatch struct {
	SubjectID     *uint         `json:"subject_id"`
	BancaID       *uint         `json:"banca_id"`
	ExamID        *uint         `json:"exam_id"`
	Format        *Format       `json:"format"`
	Statement     *string       `json:"statement"`
	Alternatives  *Alternatives `json:"alternatives"`
	CorrectAnswer *string       `json:"correct_answer"`
}

func (s *Service) Update(id uint, patch QuestionPatch) (Question, error) {
	q, err := s.repo.FindByID(id)
	if err != nil {
		return Question{}, err
	}

	fields := map[string]any{}
	if patch.Statement != nil {
		statement := strings.TrimSpace(*patch.Statement)
		if statement == "" {
			return Question{}, platform.Invalid("enunciado é obrigatório")
		}
		fields["statement"] = statement
	}
	if patch.CorrectAnswer != nil {
		answer := strings.TrimSpace(*patch.CorrectAnswer)
		if answer == "" {
			return Question{}, platform.Invalid("resposta correta é obrigatória")
		}
		fields["correct_answer"] = answer
	}
	if patch.Format != nil {
		if !patch.Format.Valid() {
			return Question{}, platform.Invalid("format deve ser multipla_escolha ou certo_errado")
		}
		fields["format"] = *patch.Format
	}
	if patch.Alternatives != nil {
		fields["alternatives"] = *patch.Alternatives
	}
	if err := s.validateRefs(patch.SubjectID, patch.BancaID, patch.ExamID); err != nil {
		return Question{}, err
	}
	if patch.SubjectID != nil {
		fields["subject_id"] = *patch.SubjectID
	}
	if patch.BancaID != nil {
		fields["banca_id"] = nullableID(*patch.BancaID)
	}
	if patch.ExamID != nil {
		fields["exam_id"] = nullableID(*patch.ExamID)
	}
	if len(fields) == 0 {
		return q, nil
	}

	if err := s.repo.Update(&q, fields); err != nil {
		return Question{}, err
	}
	return q, nil
}

func (s *Service) Delete(id uint) error {
	rows, err := s.repo.Delete(id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return platform.NotFound("questão não encontrada")
	}
	return nil
}

// validateRefs confere que os ids apontados existem. subjectID obrigatório
// quando informado; banca e exam são opcionais e 0 significa "sem vínculo".
func (s *Service) validateRefs(subjectID, bancaID, examID *uint) error {
	if subjectID != nil {
		if *subjectID == 0 {
			return platform.Invalid("subject_id é obrigatório")
		}
		if err := s.catalog.RequireSubject(*subjectID); err != nil {
			return err
		}
	}
	if bancaID != nil && *bancaID != 0 {
		if err := s.catalog.RequireBanca(*bancaID); err != nil {
			return err
		}
	}
	if examID != nil && *examID != 0 {
		if err := s.catalog.RequireExam(*examID); err != nil {
			return err
		}
	}
	return nil
}

// nullableID converte 0 em NULL: é como o PATCH desvincula um campo opcional
// sem precisar de ponteiro para ponteiro no JSON.
func nullableID(id uint) any {
	if id == 0 {
		return nil
	}
	return id
}
