package dashboard

import (
	"time"

	"github.com/leomach/studycentralback/internal/catalog"
	"github.com/leomach/studycentralback/internal/flashcard"
	"github.com/leomach/studycentralback/internal/question"
)

// Quanto material buscar antes de priorizar. Teto generoso: a seleção fina é
// do BuildQueue, mas não faz sentido carregar o banco inteiro.
const (
	maxFlashcardCandidates = 200
	maxQuestionCandidates  = 200
)

// Service junta os domínios de estudo para montar a fila e os agregados.
// dashboard importa question e flashcard; o contrário nunca acontece.
type Service struct {
	catalog    *catalog.Service
	questions  *question.Service
	flashcards *flashcard.Service
	now        func() time.Time
}

func NewService(c *catalog.Service, q *question.Service, f *flashcard.Service) *Service {
	return &Service{catalog: c, questions: q, flashcards: f, now: time.Now}
}

// Queue é o endpoint central do produto: "tenho N minutos, o que estudo?".
// Este método só faz I/O e tradução; a regra de priorização é do BuildQueue.
func (s *Service) Queue(userID uint, minutes int) ([]QueueItem, error) {
	due, err := s.flashcards.DueCards(userID, maxFlashcardCandidates)
	if err != nil {
		return nil, err
	}

	questions, err := s.questions.Candidates(userID, maxQuestionCandidates)
	if err != nil {
		return nil, err
	}

	subjectStats, err := s.questions.SubjectStats(userID)
	if err != nil {
		return nil, err
	}

	subjectNames, err := s.subjectNames()
	if err != nil {
		return nil, err
	}

	stats := make(map[uint]SubjectStat, len(subjectStats))
	for _, st := range subjectStats {
		stats[st.SubjectID] = SubjectStat{Attempts: st.Attempts, Correct: st.Correct}
	}

	now := s.now()
	candidates := make([]Candidate, 0, len(due)+len(questions))
	for _, d := range due {
		state := flashcard.State{DueDate: d.DueDate}
		candidates = append(candidates, Candidate{
			Kind:        ItemFlashcard,
			ID:          d.FlashcardID,
			SubjectID:   d.SubjectID,
			SubjectName: subjectNames[d.SubjectID],
			OverdueDays: state.Overdue(now),
			Due:         state.IsDue(now),
			Content: Content{
				CardKind:     string(d.Kind),
				Front:        d.Front,
				Back:         d.Back,
				IntervalDays: d.IntervalDays,
				EaseFactor:   d.EaseFactor,
				Lapses:       d.Lapses,
				Reps:         d.Reps,
			},
		})
	}
	for _, q := range questions {
		candidates = append(candidates, Candidate{
			Kind:        ItemQuestion,
			ID:          q.ID,
			SubjectID:   q.SubjectID,
			SubjectName: subjectNames[q.SubjectID],
			Attempted:   q.Attempts > 0,
			Content: Content{
				BancaID:       q.BancaID,
				ExamID:        q.ExamID,
				Format:        string(q.Format),
				Statement:     q.Statement,
				Alternatives:  q.Alternatives,
				CorrectAnswer: q.CorrectAnswer,
			},
		})
	}

	return BuildQueue(candidates, stats, minutes), nil
}

// subjectNames monta o mapa id -> nome. A fila e o overview devolvem o nome
// junto para o app não precisar cruzar ids em toda tela.
func (s *Service) subjectNames() (map[uint]string, error) {
	subjects, err := s.catalog.ListSubjects()
	if err != nil {
		return nil, err
	}

	names := make(map[uint]string, len(subjects))
	for _, subject := range subjects {
		names[subject.ID] = subject.Name
	}
	return names, nil
}

func (s *Service) examNames() (map[uint]string, error) {
	exams, err := s.catalog.ListExams()
	if err != nil {
		return nil, err
	}

	names := make(map[uint]string, len(exams))
	for _, exam := range exams {
		names[exam.ID] = exam.Name
	}
	return names, nil
}

// Overview é o resumo de desempenho: onde estou forte, onde estou fraco, se a
// minha certeza corresponde ao acerto real e se o baralho está amadurecendo.
type Overview struct {
	Flashcards FlashcardOverview `json:"flashcards"`
	Subjects   []SubjectOverview `json:"subjects"`
	Exams      []ExamOverview    `json:"exams"`
	Confidence []ConfidenceSlice `json:"confidence"`
	Volume     VolumeOverview    `json:"volume"`
}

type FlashcardOverview struct {
	Due    int64 `json:"due"`
	Mature int64 `json:"mature"`
	Total  int64 `json:"total"`
}

type SubjectOverview struct {
	SubjectID   uint    `json:"subject_id"`
	SubjectName string  `json:"subject_name"`
	Attempts    int     `json:"attempts"`
	Correct     int     `json:"correct"`
	Accuracy    float64 `json:"accuracy"`
}

type ExamOverview struct {
	ExamID   uint    `json:"exam_id"`
	ExamName string  `json:"exam_name"`
	Attempts int     `json:"attempts"`
	Correct  int     `json:"correct"`
	Accuracy float64 `json:"accuracy"`
}

type ConfidenceSlice struct {
	Confidence string  `json:"confidence"`
	Attempts   int     `json:"attempts"`
	Correct    int     `json:"correct"`
	Accuracy   float64 `json:"accuracy"`
}

// VolumeOverview mede constância: quantas questões foram respondidas nas
// últimas semanas.
type VolumeOverview struct {
	Last7Days  int64 `json:"last_7_days"`
	Last30Days int64 `json:"last_30_days"`
}

func (s *Service) Overview(userID uint) (Overview, error) {
	cards, err := s.flashcards.CountCards(userID)
	if err != nil {
		return Overview{}, err
	}

	subjectStats, err := s.questions.SubjectStats(userID)
	if err != nil {
		return Overview{}, err
	}

	examStats, err := s.questions.ExamStats(userID)
	if err != nil {
		return Overview{}, err
	}

	confidenceStats, err := s.questions.ConfidenceStats(userID)
	if err != nil {
		return Overview{}, err
	}

	volume, err := s.questions.AttemptVolume(userID, s.now())
	if err != nil {
		return Overview{}, err
	}

	subjectNames, err := s.subjectNames()
	if err != nil {
		return Overview{}, err
	}

	examNames, err := s.examNames()
	if err != nil {
		return Overview{}, err
	}

	out := Overview{
		Flashcards: FlashcardOverview{Due: cards.Due, Mature: cards.Mature, Total: cards.Total},
		Subjects:   make([]SubjectOverview, 0, len(subjectStats)),
		Exams:      make([]ExamOverview, 0, len(examStats)),
		Confidence: make([]ConfidenceSlice, 0, len(confidenceStats)),
		Volume:     VolumeOverview{Last7Days: volume.Last7Days, Last30Days: volume.Last30Days},
	}
	for _, st := range subjectStats {
		out.Subjects = append(out.Subjects, SubjectOverview{
			SubjectID:   st.SubjectID,
			SubjectName: subjectNames[st.SubjectID],
			Attempts:    st.Attempts,
			Correct:     st.Correct,
			Accuracy:    accuracy(st.Correct, st.Attempts),
		})
	}
	for _, st := range examStats {
		out.Exams = append(out.Exams, ExamOverview{
			ExamID:   st.ExamID,
			ExamName: examNames[st.ExamID],
			Attempts: st.Attempts,
			Correct:  st.Correct,
			Accuracy: accuracy(st.Correct, st.Attempts),
		})
	}
	for _, st := range confidenceStats {
		out.Confidence = append(out.Confidence, ConfidenceSlice{
			Confidence: string(st.Confidence),
			Attempts:   st.Attempts,
			Correct:    st.Correct,
			Accuracy:   accuracy(st.Correct, st.Attempts),
		})
	}

	return out, nil
}

func accuracy(correct, attempts int) float64 {
	if attempts == 0 {
		return 0
	}
	return float64(correct) / float64(attempts)
}
