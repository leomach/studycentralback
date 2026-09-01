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
| GET | `/health` | healthcheck (serve de sonda de conectividade no PWA) |
| GET/POST | `/api/subjects` | eixos temáticos (aceita `parent_id`) |
| PATCH/DELETE | `/api/subjects/:id` | editar / remover |
| GET/POST | `/api/bancas` | bancas |
| PATCH/DELETE | `/api/bancas/:id` | editar / remover |
| GET/POST | `/api/exams` | concursos |
| PATCH/DELETE | `/api/exams/:id` | editar / remover |
| GET/POST | `/api/questions` | questões (filtros `subject_id`, `banca_id`, `limit`) |
| GET/PATCH/DELETE | `/api/questions/:id` | ler / editar / remover |
| POST | `/api/questions/:id/attempts` | responde: `answer` + `confidence` |
| GET/POST | `/api/flashcards` | cards |
| GET | `/api/flashcards/due` | cards vencidos |
| GET/PATCH/DELETE | `/api/flashcards/:id` | ler / editar / remover |
| POST | `/api/flashcards/:id/reviews` | autoavaliação: `grade` de 1 a 4 |
| **GET** | **`/api/study/queue?minutes=40`** | **a fila do dia** |
| GET | `/api/dashboard/overview` | acerto por eixo e por concurso, cards vencidos vs. maduros, confiança e volume de 7/30 dias |

### A fila do dia é autossuficiente

Cada item de `/api/study/queue` já vem com o conteúdo inteiro — `front`/`back`
do card, ou `statement`/`alternatives`/`correct_answer` da questão — mais o
nome do eixo e os `reasons` da priorização. Um request só: dá para o service
worker cachear a sessão antes de sair de casa e estudar sem rede.

O `correct_answer` viaja junto para o app corrigir offline. O servidor não
confia nisso: `POST /attempts` recalcula `is_correct` contra o banco.

Cada card de flashcard na fila também traz `ease_factor`, `interval_days`,
`reps` e `lapses` — o estado do SM-2 no momento em que a fila foi montada.
É o que permite ao app calcular offline o próximo intervalo antes de o usuário
escolher uma nota (o preview "3 d" nos botões de avaliação), sem consultar o
servidor.

### Idempotência (`client_id`)

`POST /questions/:id/attempts` e `POST /flashcards/:id/reviews` exigem
`client_id`, um UUID gerado pelo cliente. É a chave que faz retentativas
offline seguras: se a mesma sincronização for reenviada porque o app não viu
a resposta a tempo, a segunda chamada não duplica a tentativa nem reaplica o
SM-2 sobre o mesmo evento — devolve o resultado já gravado.

Os dois domínios resolvem isso de formas diferentes porque as tabelas têm
formas diferentes: `attempts` é um log de eventos, então `client_id` é
`UNIQUE` e a segunda inserção vira busca pela linha existente. Já
`flashcard_reviews` guarda só o estado *atual* do card (uma linha por
flashcard, sempre sobrescrita) — não dá pra "ignorar inserção duplicada"
porque não é um insert. Por isso ali a idempotência compara com o último
`client_id` aplicado: se bater, devolve o estado já salvo sem rodar o SM-2 de
novo.

### PATCH parcial

Envie só os campos que mudam. Campo ausente não é tocado; em campos opcionais
(`banca_id`, `exam_id`, `parent_id`, `source_question_id`), o valor `0`
desvincula.

### Erros

Toda falha responde `{"error": "...", "code": "..."}`:

| Status | `code` | Quando |
| --- | --- | --- |
| 400 | `invalid` | dado errado no pedido (`subject_id não existe`, campo obrigatório vazio) |
| 404 | `not_found` | não existe — o app pode tirar do cache |
| 409 | `conflict` | está em uso (apagar um eixo que ainda tem questões) |
| 500 | `internal` | erro inesperado; o detalhe fica no log do servidor, nunca na resposta |

## As duas peças que importam

`internal/flashcard/sm2.go` e `internal/dashboard/queue.go` são funções puras,
sem banco nem HTTP, e concentram os testes. Toda iteração de regra acontece
nelas.

A fila combina três critérios, nesta ordem de peso: vencimento do card, eixo
pouco estudado e histórico de erro. Cada item volta com `reasons` explicando
por que entrou — fila que não se explica não ganha confiança.
# studycentralback
