# Central de Estudos

Backend em Go para estudo dirigido em micro-sessões: banco de questões com
registro de confiança, flashcards com repetição espaçada (SM-2) e uma fila do
dia que responde "tenho N minutos, o que estudo agora?".

## Rodar localmente

```sh
make db-up                       # sobe o Postgres
cp .env.example .env             # gere valores reais para JWT_SECRET e ADMIN_SECRET:
                                  #   openssl rand -base64 48
make migrate-up                  # aplica as migrations
make run                         # sobe a API em :8080 (o Makefile carrega .env sozinho)
```

```sh
make test                        # SM-2 e fila do dia
```

## Estrutura

```
cmd/api/            # montagem das dependências e start do servidor
internal/
├── platform/       # config, conexão, router, middleware, JWT, rate limit
├── auth/           # contas, login/registro, refresh tokens
├── catalog/        # subject, banca, exam — compartilhado entre contas
├── question/       # question (compartilhada) e attempt (por conta)
├── flashcard/      # flashcard, review (por conta), sm2.go (algoritmo puro)
└── dashboard/      # agregados de desempenho (por conta) + queue.go (fila do dia)
migrations/         # SQL versionado (golang-migrate)
```

São 9 tabelas: `users`, `refresh_tokens`, `subjects`, `bancas`, `exams`,
`questions`, `attempts`, `flashcards`, `flashcard_reviews`.

**Multi-tenancy**: `subjects`, `bancas`, `exams` e `questions` são
compartilhadas entre todas as contas — o gabarito de uma prova é o mesmo pra
quem quer que a esteja estudando. `attempts`, `flashcards` e
`flashcard_reviews` são privadas por conta (`user_id`) — sua resposta, seus
cards, seu progresso não aparecem pra ninguém além de você.

Cada domínio é um pacote com model, repository, service e handler juntos. A
dependência só anda num sentido: `auth` e `catalog` só dependem de `platform`;
`dashboard` importa `question` e `flashcard`; o contrário nunca.

## Endpoints

**Públicas** (sem token):

| Método | Rota | O que faz |
| --- | --- | --- |
| GET | `/health` | healthcheck (serve de sonda de conectividade no PWA) |
| POST | `/api/auth/register` | cadastro: `name`, `email`, `password` (mín. 10 caracteres) |
| POST | `/api/auth/login` | `email` + `password` → `{access_token, refresh_token}` |
| POST | `/api/auth/refresh` | troca um `refresh_token` por um par novo (rotação a cada uso) |
| POST | `/api/auth/logout` | revoga um `refresh_token` |

`register`/`login` têm rate limit por IP (5 tentativas / 15 min).

**Administrativas** (header `X-Admin-Secret`, ver `ADMIN_SECRET`):

| Método | Rota | O que faz |
| --- | --- | --- |
| POST | `/api/admin/users/:id/plan` | promove a conta a premium — substituto temporário até existir cobrança real |

**Protegidas** (header `Authorization: Bearer <access_token>`, exige plano `premium` — `free` recebe 403 em tudo abaixo, por enquanto):

| Método | Rota | O que faz |
| --- | --- | --- |
| GET/POST | `/api/subjects` | eixos temáticos (aceita `parent_id`) |
| PATCH/DELETE | `/api/subjects/:id` | editar / remover |
| GET/POST | `/api/bancas` | bancas |
| PATCH/DELETE | `/api/bancas/:id` | editar / remover |
| GET/POST | `/api/exams` | concursos |
| PATCH/DELETE | `/api/exams/:id` | editar / remover |
| GET/POST | `/api/questions` | questões (filtros `subject_id`, `banca_id`, `exam_id`, `format`; paginado — ver abaixo) |
| GET/PATCH/DELETE | `/api/questions/:id` | ler / editar / remover |
| POST | `/api/questions/:id/attempts` | responde: `answer` + `confidence` |
| GET/POST | `/api/flashcards` | cards (filtro `subject_id`; paginado — ver abaixo) |
| GET | `/api/flashcards/due` | cards vencidos |
| GET/PATCH/DELETE | `/api/flashcards/:id` | ler / editar / remover |
| POST | `/api/flashcards/:id/reviews` | autoavaliação: `grade` de 1 a 4 |
| **GET** | **`/api/study/queue?minutes=40`** | **a fila do dia** |
| GET | `/api/dashboard/overview` | acerto por eixo e por concurso, cards vencidos vs. maduros, confiança e volume de 7/30 dias |

### Paginação de `/api/questions` e `/api/flashcards`

As duas únicas listas que crescem sem limite (o resto — eixos, bancas,
concursos — é pequeno e cabe inteiro numa resposta). Query params `limit`
(padrão 20, teto 100) e `offset` (padrão 0). O corpo não é mais um array
solto, é um envelope com o total que bate com o filtro (sem limit/offset),
para o cliente saber quanto falta:

```json
{ "items": [...], "total": 142, "limit": 20, "offset": 0 }
```

Página seguinte: repetir a chamada com `offset` = `offset + len(items)` da
página anterior, até `offset + len(items) >= total`. `GET /api/flashcards/due`
continua sem paginação de propósito: é uma lista já orçada por `limit` para o
prefetch da sessão do dia, não uma tela de navegação.

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
| 401 | `unauthorized` | token ausente/inválido/expirado, ou login/refresh recusado |
| 403 | `forbidden` | autenticado, mas plano não permite (hoje: qualquer coisa fora de `/auth/*` no plano free) |
| 404 | `not_found` | não existe — o app pode tirar do cache |
| 409 | `conflict` | está em uso (apagar um eixo que ainda tem questões; email já cadastrado) |
| 429 | `rate_limited` | muitas tentativas de login/cadastro pelo mesmo IP |
| 500 | `internal` | erro inesperado; o detalhe fica no log do servidor, nunca na resposta |

## Autenticação e planos

Contas nascem no plano `free` e não acessam nada além de `/auth/*` — é assim
"por enquanto", até existir cobrança de verdade. Promover alguém a `premium`
hoje é manual: `POST /api/admin/users/:id/plan` com o header
`X-Admin-Secret: $ADMIN_SECRET`.

Sessão: access token JWT curto (15 min, assinado com `JWT_SECRET`) + refresh
token de 30 dias, revogável, guardado como hash no banco. Cada uso do refresh
token o substitui por um novo (rotação) — reusar um token já trocado é
tratado como sinal de vazamento e derruba todas as sessões daquela conta.

Login nunca diferencia "conta não existe" de "senha errada" — mesma
mensagem, mesmo tempo de resposta — para não permitir descobrir quais
e-mails estão cadastrados.

## As duas peças que importam

`internal/flashcard/sm2.go` e `internal/dashboard/queue.go` são funções puras,
sem banco nem HTTP, e concentram os testes. Toda iteração de regra acontece
nelas.

A fila combina três critérios, nesta ordem de peso: vencimento do card, eixo
pouco estudado e histórico de erro. Cada item volta com `reasons` explicando
por que entrou — fila que não se explica não ganha confiança.

## Importar questões em lote

Quem faz isso é o `studycentralscraper` (projeto irmão, fora deste repo — ver
`../studycentralscraper/README.md`): raspa provas, gera um CSV, e importa via
HTTP contra estes mesmos endpoints. Este repo não sabe nada sobre scraping nem
sobre CSV — só expõe a API que qualquer cliente (esse importer incluído)
consome.
