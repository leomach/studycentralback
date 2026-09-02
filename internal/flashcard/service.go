package flashcard

import (
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"

	"github.com/leomach/studycentralback/internal/catalog"
	"github.com/leomach/studycentralback/internal/platform"
	"github.com/leomach/studycentralback/internal/question"
)

// Service depende de um relógio injetável. Em Go não se costuma "mockar"
// time.Now globalmente: passa-se a função, e o teste passa outra.
//
// catalog e question entram para validar subject_id e source_question_id
// antes de gravar — id inexistente vira 400 com mensagem clara.
type Service struct {
	repo      *Repository
	catalog   *catalog.Service
	questions *question.Service
	now       func() time.Time
}

func NewService(repo *Repository, catalogSvc *catalog.Service, questionSvc *question.Service) *Service {
	return &Service{repo: repo, catalog: catalogSvc, questions: questionSvc, now: time.Now}
}

// List devolve os cards com a review (estado do SM-2) embutida: é o que
// permite ao front derivar o estado (vencido/aprendizado/maduro) de cada card
// sem uma chamada por card. Também devolve o total que bate com o filtro
// (sem limit/offset), para a UI saber quanto falta carregar.
func (s *Service) List(userID, subjectID uint, limit, offset int) ([]WithReview, int64, error) {
	cards, err := s.repo.List(userID, subjectID, limit, offset)
	if err != nil {
		return nil, 0, err
	}

	ids := make([]uint, len(cards))
	for i, c := range cards {
		ids[i] = c.ID
	}
	reviews, err := s.repo.ReviewsByFlashcardID(userID, ids)
	if err != nil {
		return nil, 0, err
	}

	out := make([]WithReview, len(cards))
	for i, c := range cards {
		out[i] = WithReview{Flashcard: c}
		if rv, ok := reviews[c.ID]; ok {
			out[i].Review = &rv
		}
	}

	total, err := s.repo.Count(userID, subjectID)
	if err != nil {
		return nil, 0, err
	}
	return out, total, nil
}

type NewFlashcard struct {
	SubjectID        uint
	Kind             Kind
	Front            string
	Back             string
	SourceQuestionID *uint
}

func (s *Service) Create(userID uint, in NewFlashcard) (Flashcard, error) {
	if strings.TrimSpace(in.Front) == "" {
		return Flashcard{}, platform.Invalid("frente do card é obrigatória")
	}
	if strings.TrimSpace(in.Back) == "" {
		return Flashcard{}, platform.Invalid("verso do card é obrigatório")
	}
	if in.Kind == "" {
		in.Kind = KindQuestionAnswer
	}
	if !in.Kind.Valid() {
		return Flashcard{}, platform.Invalid("kind deve ser pergunta_resposta ou resumo")
	}
	if err := s.validateRefs(&in.SubjectID, in.SourceQuestionID); err != nil {
		return Flashcard{}, err
	}

	card := Flashcard{
		UserID:           userID,
		SubjectID:        in.SubjectID,
		Kind:             in.Kind,
		Front:            in.Front,
		Back:             in.Back,
		SourceQuestionID: in.SourceQuestionID,
	}
	if err := s.repo.Create(&card, s.now()); err != nil {
		return Flashcard{}, err
	}
	return card, nil
}

func (s *Service) DueCards(userID uint, limit int) ([]Due, error) {
	return s.repo.DueCards(userID, s.now(), limit)
}

func (s *Service) CountDue(userID uint) (int64, error) {
	return s.repo.CountDue(userID, s.now())
}

func (s *Service) CountCards(userID uint) (Counts, error) {
	return s.repo.CountCards(userID, s.now())
}

// Grade registra a autoavaliação: carrega o estado, aplica o SM-2 puro e
// persiste o resultado.
//
// clientID é a chave de idempotência do outbox offline. Diferente de Attempt
// (que é um log de eventos), flashcard_reviews guarda só o estado ATUAL do
// card — não dá para "ignorar inserção duplicada" porque não é um insert, é
// sempre um update na mesma linha. Por isso a idempotência aqui compara com o
// último client_id aplicado: se for o mesmo, a retentativa devolve o estado
// já salvo em vez de rodar o SM-2 de novo sobre o resultado anterior.
func (s *Service) Grade(userID, flashcardID uint, clientID string, g Grade) (Review, error) {
	if strings.TrimSpace(clientID) == "" {
		return Review{}, platform.Invalid("client_id é obrigatório")
	}
	if !g.Valid() {
		return Review{}, platform.Invalid("grade deve ser 1 (errei), 2 (difícil), 3 (bom) ou 4 (fácil)")
	}

	review, err := s.repo.FindReview(userID, flashcardID)
	if err != nil {
		return Review{}, err
	}

	if review.LastClientID == clientID {
		return review, nil
	}

	next := Schedule(review.State(), g, s.now())
	review.applyState(next, g)
	review.LastClientID = clientID
	if err := s.repo.SaveReview(&review); err != nil {
		return Review{}, err
	}
	return review, nil
}

// FindByID traz o card com a review embutida, igual List — a tela de
// detalhe (GET /flashcards/:id) precisa de interval_days/ease_factor/reps
// tanto quanto a listagem, e não tinha isso antes de a paginação separar
// "buscar um card" de "buscar a lista".
func (s *Service) FindByID(userID, id uint) (WithReview, error) {
	card, err := s.repo.FindByID(userID, id)
	if err != nil {
		return WithReview{}, err
	}

	out := WithReview{Flashcard: card}
	review, err := s.repo.FindReview(userID, card.ID)
	if err == nil {
		out.Review = &review
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return WithReview{}, err
	}
	return out, nil
}

// FlashcardPatch traz só o que muda; nil significa "não enviado".
// SourceQuestionID com 0 desfaz o vínculo com a questão de origem.
type FlashcardPatch struct {
	SubjectID        *uint   `json:"subject_id"`
	Kind             *Kind   `json:"kind"`
	Front            *string `json:"front"`
	Back             *string `json:"back"`
	SourceQuestionID *uint   `json:"source_question_id"`
}

func (s *Service) Update(userID, id uint, patch FlashcardPatch) (Flashcard, error) {
	card, err := s.repo.FindByID(userID, id)
	if err != nil {
		return Flashcard{}, err
	}

	fields := map[string]any{}
	if patch.Front != nil {
		front := strings.TrimSpace(*patch.Front)
		if front == "" {
			return Flashcard{}, platform.Invalid("frente do card é obrigatória")
		}
		fields["front"] = front
	}
	if patch.Back != nil {
		back := strings.TrimSpace(*patch.Back)
		if back == "" {
			return Flashcard{}, platform.Invalid("verso do card é obrigatório")
		}
		fields["back"] = back
	}
	if patch.Kind != nil {
		if !patch.Kind.Valid() {
			return Flashcard{}, platform.Invalid("kind deve ser pergunta_resposta ou resumo")
		}
		fields["kind"] = *patch.Kind
	}
	if err := s.validateRefs(patch.SubjectID, patch.SourceQuestionID); err != nil {
		return Flashcard{}, err
	}
	if patch.SubjectID != nil {
		fields["subject_id"] = *patch.SubjectID
	}
	if patch.SourceQuestionID != nil {
		if *patch.SourceQuestionID == 0 {
			fields["source_question_id"] = nil
		} else {
			fields["source_question_id"] = *patch.SourceQuestionID
		}
	}
	if len(fields) == 0 {
		return card, nil
	}

	if err := s.repo.Update(&card, fields); err != nil {
		return Flashcard{}, err
	}
	return card, nil
}

func (s *Service) Delete(userID, id uint) error {
	rows, err := s.repo.Delete(userID, id)
	if err != nil {
		return err
	}
	if rows == 0 {
		return platform.NotFound("flashcard não encontrado")
	}
	return nil
}

func (s *Service) validateRefs(subjectID, sourceQuestionID *uint) error {
	if subjectID != nil {
		if *subjectID == 0 {
			return platform.Invalid("subject_id é obrigatório")
		}
		if err := s.catalog.RequireSubject(*subjectID); err != nil {
			return err
		}
	}
	if sourceQuestionID != nil && *sourceQuestionID != 0 {
		if _, err := s.questions.FindByID(*sourceQuestionID); err != nil {
			return platform.Invalid("source_question_id não existe")
		}
	}
	return nil
}
