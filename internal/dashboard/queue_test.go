package dashboard

import "testing"

func TestBuildQueueRespeitaOOrcamentoDeTempo(t *testing.T) {
	// 10 flashcards vencidos a 30s cada = 300s. Em 2 minutos cabem 4.
	cands := make([]Candidate, 0, 10)
	for i := 1; i <= 10; i++ {
		cands = append(cands, Candidate{
			Kind: ItemFlashcard, ID: uint(i), SubjectID: 1, Due: true, OverdueDays: 1,
		})
	}

	queue := BuildQueue(cands, map[uint]SubjectStat{1: {Attempts: 50, Correct: 45}}, 2)

	if len(queue) != 4 {
		t.Fatalf("fila com %d itens, quer 4", len(queue))
	}
	total := 0
	for _, item := range queue {
		total += item.EstimatedSeconds
	}
	if total > 120 {
		t.Errorf("fila estourou o orçamento: %ds em 120s", total)
	}
}

func TestBuildQueueVencimentoTemPrioridadeSobreOsDemais(t *testing.T) {
	stats := map[uint]SubjectStat{
		1: {Attempts: 100, Correct: 95}, // eixo dominado
		2: {Attempts: 0, Correct: 0},    // eixo intocado, com erro alto potencial
	}
	cands := []Candidate{
		{Kind: ItemQuestion, ID: 99, SubjectID: 2},                            // inédita, eixo esquecido
		{Kind: ItemFlashcard, ID: 1, SubjectID: 1, Due: true, OverdueDays: 0}, // vence hoje
	}

	queue := BuildQueue(cands, stats, 30)

	if queue[0].Kind != ItemFlashcard {
		t.Errorf("primeiro item = %s, quer o flashcard vencido", queue[0].Kind)
	}
}

func TestBuildQueueMaisAtrasadoVemPrimeiro(t *testing.T) {
	cands := []Candidate{
		{Kind: ItemFlashcard, ID: 1, SubjectID: 1, Due: true, OverdueDays: 0},
		{Kind: ItemFlashcard, ID: 2, SubjectID: 1, Due: true, OverdueDays: 15},
		{Kind: ItemFlashcard, ID: 3, SubjectID: 1, Due: true, OverdueDays: 3},
	}

	queue := BuildQueue(cands, map[uint]SubjectStat{1: {Attempts: 50, Correct: 40}}, 30)

	want := []uint{2, 3, 1}
	for i, id := range want {
		if queue[i].ID != id {
			t.Errorf("posição %d = card %d, quer %d", i, queue[i].ID, id)
		}
	}
}

func TestBuildQueueDesempataPeloEixoMaisFraco(t *testing.T) {
	// Mesmo vencimento: ganha o eixo com pior histórico.
	stats := map[uint]SubjectStat{
		1: {Attempts: 40, Correct: 38}, // 5% de erro
		2: {Attempts: 40, Correct: 10}, // 75% de erro
	}
	cands := []Candidate{
		{Kind: ItemFlashcard, ID: 1, SubjectID: 1, Due: true, OverdueDays: 2},
		{Kind: ItemFlashcard, ID: 2, SubjectID: 2, Due: true, OverdueDays: 2},
	}

	queue := BuildQueue(cands, stats, 30)

	if queue[0].SubjectID != 2 {
		t.Errorf("primeiro item veio do eixo %d, quer o eixo 2 (mais erros)", queue[0].SubjectID)
	}
}

func TestBuildQueueIgnoraFlashcardNaoVencido(t *testing.T) {
	cands := []Candidate{
		{Kind: ItemFlashcard, ID: 1, SubjectID: 1, Due: false},
		{Kind: ItemFlashcard, ID: 2, SubjectID: 1, Due: true},
	}

	queue := BuildQueue(cands, map[uint]SubjectStat{}, 30)

	if len(queue) != 1 || queue[0].ID != 2 {
		t.Errorf("fila = %+v, quer só o card 2", queue)
	}
}

func TestBuildQueueDescartaQuestaoJaRespondidaSemMotivo(t *testing.T) {
	// Eixo bem estudado e com bom desempenho: repetir a questão é desperdício.
	stats := map[uint]SubjectStat{1: {Attempts: 100, Correct: 100}}
	cands := []Candidate{
		{Kind: ItemQuestion, ID: 1, SubjectID: 1, Attempted: true},
		{Kind: ItemQuestion, ID: 2, SubjectID: 1, Attempted: false},
	}

	queue := BuildQueue(cands, stats, 30)

	if len(queue) != 1 || queue[0].ID != 2 {
		t.Errorf("fila = %+v, quer só a questão inédita", queue)
	}
}

func TestBuildQueueEDeterministica(t *testing.T) {
	cands := []Candidate{
		{Kind: ItemQuestion, ID: 7, SubjectID: 1},
		{Kind: ItemQuestion, ID: 3, SubjectID: 1},
		{Kind: ItemQuestion, ID: 5, SubjectID: 1},
	}

	first := BuildQueue(cands, map[uint]SubjectStat{}, 30)
	for range 5 {
		again := BuildQueue(cands, map[uint]SubjectStat{}, 30)
		for i := range first {
			if first[i].ID != again[i].ID {
				t.Fatalf("ordem instável na posição %d: %d != %d", i, first[i].ID, again[i].ID)
			}
		}
	}
	if first[0].ID != 3 {
		t.Errorf("empate desempatado por ID: primeiro = %d, quer 3", first[0].ID)
	}
}

func TestBuildQueueCasosVazios(t *testing.T) {
	if got := BuildQueue(nil, nil, 40); len(got) != 0 {
		t.Errorf("sem candidatos: fila = %+v, quer vazia", got)
	}
	cands := []Candidate{{Kind: ItemFlashcard, ID: 1, SubjectID: 1, Due: true}}
	if got := BuildQueue(cands, nil, 0); len(got) != 0 {
		t.Errorf("0 minutos: fila = %+v, quer vazia", got)
	}
}

func TestBuildQueueExplicaAPrioridade(t *testing.T) {
	cands := []Candidate{{Kind: ItemFlashcard, ID: 1, SubjectID: 1, Due: true, OverdueDays: 4}}
	stats := map[uint]SubjectStat{1: {Attempts: 5, Correct: 1}}

	queue := BuildQueue(cands, stats, 30)

	if len(queue[0].Reasons) != 3 {
		t.Errorf("motivos = %v, quer os três critérios", queue[0].Reasons)
	}
}
