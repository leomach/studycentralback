# Central de Estudos

Backend em Go para estudo dirigido em micro-sessões: banco de questões com
registro de confiança, flashcards com repetição espaçada (SM-2) e uma fila do
dia que responde "tenho N minutos, o que estudo agora?".

## Rodar localmente

```sh
make db-up                       # sobe o Postgres
cp .env.example .env
make migrate-up                  # aplica as migrations
set -a && source .env && set +a
make run                         # sobe a API em :8080
```

```sh
make test                        # SM-2 e fila do dia
```

## Estrutura

```
cmd/api/            # montagem das dependências e start do servidor
internal/
├── platform/       # config, conexão, router, middleware
├── catalog/        # subject, banca, exam
├── question/       # question, attempt
├── flashcard/      # flashcard, review, sm2.go (algoritmo puro)
└── dashboard/      # agregados de desempenho + queue.go (fila do dia)
migrations/         # SQL versionado (golang-migrate)
```

São 8 tabelas: `users`, `subjects`, `bancas`, `exams`, `questions`,
`attempts`, `flashcards`, `flashcard_reviews`. `users` existe desde o início
com um registro seed (`id=1`) e todo conteúdo de estudo aponta para ele por
foreign key real, mesmo sem autenticação ainda.

Cada domínio é um pacote com model, repository, service e handler juntos. A
dependência só anda num sentido: `dashboard` importa `question` e `flashcard`;
o contrário nunca.

## Endpoints

| Método | Rota | O que faz |
| --- | --- | --- |
| GET | `/health` | healthcheck |
| GET/POST | `/api/subjects` | eixos temáticos (aceita `parent_id`) |
| GET/POST | `/api/bancas` | bancas |
| GET/POST | `/api/exams` | concursos |
| GET/POST | `/api/questions` | questões (filtros `subject_id`, `banca_id`, `limit`) |
| POST | `/api/questions/:id/attempts` | responde: `answer` + `confidence` |
| GET/POST | `/api/flashcards` | cards |
| GET | `/api/flashcards/due` | cards vencidos |
| POST | `/api/flashcards/:id/reviews` | autoavaliação: `grade` de 1 a 4 |
| **GET** | **`/api/study/queue?minutes=40`** | **a fila do dia** |
| GET | `/api/dashboard/overview` | acerto por eixo e por concurso, cards vencidos vs. maduros, confiança e volume de 7/30 dias |

## As duas peças que importam

`internal/flashcard/sm2.go` e `internal/dashboard/queue.go` são funções puras,
sem banco nem HTTP, e concentram os testes. Toda iteração de regra acontece
nelas.

A fila combina três critérios, nesta ordem de peso: vencimento do card, eixo
pouco estudado e histórico de erro. Cada item volta com `reasons` explicando
por que entrou — fila que não se explica não ganha confiança.
# studycentralback
