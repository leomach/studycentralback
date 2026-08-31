// Package dashboard reúne as leituras agregadas de desempenho e monta a fila
// do dia. É o único pacote que pode importar catalog, question e flashcard —
// a dependência nunca anda no sentido contrário.
package dashboard

import (
	"sort"

	"github.com/leomach/studycentralback/internal/question"
)

// ItemKind diz de onde veio o item da fila.
type ItemKind string

const (
	ItemFlashcard ItemKind = "flashcard"
	ItemQuestion  ItemKind = "question"
)

// Custo estimado de cada item, usado para caber a fila no tempo pedido.
// Valores grosseiros de propósito: a fila é uma sugestão, não um cronômetro.
const (
	secondsPerFlashcard = 30
	secondsPerQuestion  = 100
)

// Pesos dos três critérios de priorização, na ordem definida para o produto:
// 1) vencimento, 2) eixo pouco estudado, 3) histórico de erro.
// Escalonados para que o critério de cima domine, mas sem anular os de baixo:
// entre dois cards igualmente vencidos, ganha o do eixo mais fraco.
const (
	weightDue      = 100.0
	weightNeglect  = 40.0
	weightErrRate  = 30.0
	maxOverdueDays = 30.0 // teto: card parado há 1 ano não vale 12x um de 1 mês
	neglectTarget  = 20.0 // tentativas a partir das quais o eixo deixa de ser "pouco estudado"
	minForErrRate  = 3    // menos que isso, a taxa de acerto é ruído
)

// Candidate é um item elegível para a fila, já com o mínimo que a priorização
// precisa. Nada de ponteiro para banco aqui: BuildQueue é função pura.
type Candidate struct {
	Kind        ItemKind
	ID          uint
	SubjectID   uint
	SubjectName string
	OverdueDays int  // dias vencidos (flashcards); 0 para questões
	Due         bool // vencido hoje ou antes
	Attempted   bool // a questão já foi respondida alguma vez

	// Content é o que a tela mostra. BuildQueue só repassa, nunca inspeciona:
	// a priorização não depende do texto do card.
	Content Content
}

// Content carrega o item inteiro para dentro da fila. É o que permite baixar
// a sessão de estudo num request só e usá-la sem rede depois — o cenário do
// bloco de 40 minutos dentro do carro.
type Content struct {
	// Flashcard
	CardKind string `json:"card_kind,omitempty"`
	Front    string `json:"front,omitempty"`
	Back     string `json:"back,omitempty"`

	// Questão
	Format       string                `json:"format,omitempty"`
	Statement    string                `json:"statement,omitempty"`
	Alternatives question.Alternatives `json:"alternatives,omitempty"`
	// CorrectAnswer vai junto de propósito: offline o app precisa corrigir a
	// resposta na hora. O servidor não confia nisso — POST /attempts recalcula
	// is_correct comparando com o banco.
	CorrectAnswer string `json:"correct_answer,omitempty"`
}

// SubjectStat é o histórico do eixo temático, usado nos critérios 2 e 3.
type SubjectStat struct {
	Attempts int
	Correct  int
}

func (s SubjectStat) errorRate() float64 {
	if s.Attempts < minForErrRate {
		return 0
	}
	return 1 - float64(s.Correct)/float64(s.Attempts)
}

// QueueItem é um item já priorizado, com o porquê explícito — a fila precisa
// ser explicável, senão vira caixa-preta e o estudo não confia nela.
type QueueItem struct {
	Kind             ItemKind `json:"kind"`
	ID               uint     `json:"id"`
	SubjectID        uint     `json:"subject_id"`
	SubjectName      string   `json:"subject_name"`
	Score            float64  `json:"score"`
	Reasons          []string `json:"reasons"`
	EstimatedSeconds int      `json:"estimated_seconds"`
	Content          Content  `json:"content"`
}

// BuildQueue é o coração do produto: escolhe o que estudar nos próximos
// `minutes` minutos, combinando os três critérios de priorização.
//
// Função pura de propósito — sem banco, sem HTTP, sem relógio. Todo o I/O
// acontece antes, em quem a chama. É o que permite iterar na regra de
// priorização testando só isto.
func BuildQueue(candidates []Candidate, stats map[uint]SubjectStat, minutes int) []QueueItem {
	if minutes <= 0 || len(candidates) == 0 {
		return []QueueItem{}
	}

	scored := make([]QueueItem, 0, len(candidates))
	for _, c := range candidates {
		if item, ok := score(c, stats[c.SubjectID]); ok {
			scored = append(scored, item)
		}
	}

	// SliceStable + desempate explícito: fila igual para entrada igual, senão
	// a ordem muda a cada requisição e fica impossível confiar no resultado.
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].Score != scored[j].Score {
			return scored[i].Score > scored[j].Score
		}
		if scored[i].Kind != scored[j].Kind {
			return scored[i].Kind == ItemFlashcard // empate: revisão antes de questão nova
		}
		return scored[i].ID < scored[j].ID
	})

	budget := minutes * 60
	queue := make([]QueueItem, 0, len(scored))
	for _, item := range scored {
		if item.EstimatedSeconds > budget {
			continue // não cabe: tenta o próximo, que pode ser mais curto
		}
		queue = append(queue, item)
		budget -= item.EstimatedSeconds
		if budget <= 0 {
			break
		}
	}

	return queue
}

// score aplica os três critérios. O bool de retorno diz se o item entra na
// disputa — flashcard não vencido fica de fora, questão já respondida só entra
// se o eixo dela justificar.
func score(c Candidate, stat SubjectStat) (QueueItem, bool) {
	var total float64
	reasons := make([]string, 0, 3)

	// 1) Por tempo: o que venceu vem primeiro, mais atrasado na frente.
	if c.Kind == ItemFlashcard {
		if !c.Due {
			return QueueItem{}, false
		}
		overdue := min(float64(c.OverdueDays), maxOverdueDays)
		total += weightDue * (1 + overdue/maxOverdueDays)
		reasons = append(reasons, "vencido")
	}

	// 2) Por eixo pouco estudado: quanto menos tentativas, maior o empurrão.
	neglect := 1 - min(float64(stat.Attempts), neglectTarget)/neglectTarget
	if neglect > 0 {
		total += weightNeglect * neglect
		reasons = append(reasons, "eixo pouco estudado")
	}

	// 3) Por mais erros: taxa histórica de erro do eixo.
	if rate := stat.errorRate(); rate > 0 {
		total += weightErrRate * rate
		reasons = append(reasons, "histórico de erro")
	}

	// Questão já respondida e sem nenhum motivo forte não ocupa a fila: o
	// tempo é curto demais para repetir o que já está resolvido.
	if c.Kind == ItemQuestion && c.Attempted && total == 0 {
		return QueueItem{}, false
	}
	if c.Kind == ItemQuestion && !c.Attempted {
		reasons = append(reasons, "questão inédita")
	}

	item := QueueItem{
		Kind:             c.Kind,
		ID:               c.ID,
		SubjectID:        c.SubjectID,
		SubjectName:      c.SubjectName,
		Score:            total,
		Reasons:          reasons,
		EstimatedSeconds: secondsPerQuestion,
		Content:          c.Content,
	}
	if c.Kind == ItemFlashcard {
		item.EstimatedSeconds = secondsPerFlashcard
	}

	return item, true
}
