# Central de Estudos — Contexto do Projeto

## Sobre o usuário e o propósito

Leandro, 25 anos, dev full stack (estagiário de eng. de software), cursando ADS
(IFPE), estudando para concursos fiscais de alto nível (ex: SEFAZ) com horizonte
de 1,5–2 anos. Rotina extremamente apertada: CLT durante o dia, faculdade à
noite, estudo em micro-sessões (bloco principal de 40min dentro do carro antes
do trabalho, mais intervalos de almoço e horas no fim de semana).

Este é um projeto **pessoal**, para uso próprio, com potencial (não prioridade
atual) de virar SaaS multiusuário no futuro. O objetivo é resolver um problema
real de tempo e objetividade de estudo — não construir uma plataforma completa.

**Princípio guia: simplicidade sobre completude.** Leandro está aprendendo Go
neste projeto. Prefira sempre a solução mais simples que resolve o problema
real, mesmo que uma solução mais sofisticada exista. Não sugira complexidade
adicional (novos domínios, camadas extras, otimizações prematuras) sem que o
usuário peça.

## Stack técnica

- **Backend**: Go + Gin + Gorm + PostgreSQL
- **Frontend**: Next.js como PWA (offline-first — uso principal é dentro do
  carro, com conectividade não confiável)
- **Deploy**: simples, um único VPS ou serviço tipo Fly.io/Railway. Nada de
  Kubernetes ou infraestrutura distribuída para este estágio.

## Escopo do MVP — 5 domínios

O projeto foi deliberadamente reduzido de um desenho inicial mais amplo (que
incluía legislação, edital verticalizado, ingestão automatizada) para o
essencial. **Não reintroduza esses domínios a menos que o usuário peça
explicitamente.** Multi-tenancy real (contas de verdade, autenticação, dados
isolados por conta) foi pedido explicitamente e está implementado — não é
mais item de fora-de-escopo.

```
internal/
├── platform/     # config, conexão db, router, middleware, JWT, rate limit
├── auth/         # contas, login/registro, refresh tokens
├── catalog/      # subject (eixo temático), banca, exam (concurso) — compartilhado
├── question/     # question (compartilhada), attempt (por conta)
├── flashcard/    # flashcard, flashcard_review (por conta), algoritmo SM-2
└── dashboard/    # queries agregadas de desempenho (por conta)
```

Cada domínio é um pacote Go auto-contido (model + repository + service +
handler no mesmo diretório), seguindo a convenção de "package by domain"
idiomática em Go — não camadas horizontais globais (`models/`, `handlers/`
como pastas separadas cruzando todos os domínios).

Regra de dependência: **domínios de estudo dependem de domínios de conteúdo,
nunca o contrário.** `flashcard` e `question` podem ser importados por
`dashboard`, mas `catalog`/`question` nunca importam `flashcard`/`dashboard`.

## Modelo de dados (9 tabelas) e multi-tenancy

```sql
users              -- name, email (único), password_hash, plan (free|premium)
refresh_tokens     -- sessão de longa duração: token_hash, expires_at, revoked_at
subjects           -- eixo temático, com parent_id para subeixos — COMPARTILHADO
bancas             -- Cebraspe, FGV, FCC etc. — COMPARTILHADO
exams              -- concurso (nome, banca, ano) — COMPARTILHADO
questions          -- statement, alternatives (jsonb), correct_answer,
                    -- subject_id, banca_id, exam_id, format — COMPARTILHADA
attempts           -- tentativa de resposta, is_correct, confidence
                    -- (certeza | duvida | chute — campo crítico, não remover)
                    -- POR CONTA (user_id)
flashcards         -- front, back, kind (pergunta_resposta | resumo),
                    -- source_question_id opcional — POR CONTA (user_id)
flashcard_reviews  -- estado do algoritmo: due_date, interval_days,
                    -- ease_factor, reps, lapses — POR CONTA (user_id)
```

São 9 tabelas, exatamente essas. Se a contagem não bater com o que foi
implementado, isso é um erro a ser sinalizado e perguntado — nunca corrigido
silenciosamente ajustando o número ou substituindo a FK por um valor fixo
(`DEFAULT 1` sem FK, por exemplo).

**O que é compartilhado vs. por conta** (decidido explicitamente, não
assumir): `subjects`/`bancas`/`exams`/`questions` não têm `user_id` — o
conteúdo de referência e o banco de questões são os mesmos para qualquer
conta estudando o mesmo edital. `attempts`/`flashcards`/`flashcard_reviews`
têm `user_id` como FK real para `users.id` e toda query é filtrada por ele —
é a resposta, o card e o progresso de UMA conta, nunca visível a outra.

**Planos**: toda conta nasce `free` e não acessa nenhuma rota fora de
`/api/auth/*` (403 em tudo o resto) — é assim "por enquanto", até existir
cobrança de verdade. `premium` acessa tudo. Promoção hoje é manual, via
`POST /api/admin/users/:id/plan` protegido por `ADMIN_SECRET` — não existe
integração de pagamento ainda; se for pedida, é o próximo passo natural.

Use `golang-migrate` com SQL puro versionado. Não usar `AutoMigrate` do Gorm em
produção — apenas em desenvolvimento local, se necessário.

Module path: **[definir aqui — ex: github.com/seu-usuario/central-estudos]**.
Não inferir a partir de email ou qualquer outro dado — se o campo estiver
vazio ou ambíguo, perguntar antes de rodar `go mod init`.

## Algoritmo de repetição espaçada

**SM-2 clássico** (o mesmo do Anki original), não FSRS. FSRS é mais preciso mas
adiciona complexidade desnecessária para este estágio de aprendizado — SM-2 é
simples de implementar e entender por completo.

```
grade 1 (errei)     → reps=0, interval=1 dia, ease -= 0.2
grade 2 (difícil)   → interval = interval * 1.2
grade 3 (bom)       → interval = interval * ease
grade 4 (fácil)     → interval = interval * ease * 1.3, ease += 0.1
```

Isso deve viver como função pura dentro de `internal/flashcard/sm2.go`, sem
dependência de HTTP ou banco de dados — facilita testes.

## A lógica mais importante do sistema: fila do dia

O endpoint central é `GET /api/study/queue?minutes=N`. Ele combina três
critérios de priorização (nessa ordem):

1. **Por tempo**: flashcards com `due_date <= hoje`
2. **Por eixo pouco estudado**: subjects com poucas tentativas registradas
3. **Por mais erros**: subjects/questões com taxa de acerto historicamente baixa

Essa função (`BuildQueue`) é o coração do produto e deve ser o foco de maior
cuidado e iteração — mais do que qualquer CRUD.

## O que está fora de escopo por agora

Não sugerir nem implementar, a menos que solicitado:
- Legislação/versionamento de normas jurídicas
- Edital verticalizado / cobertura de tópicos
- Ingestão automatizada de PDFs / OCR / parsing de provas (isso é
  responsabilidade do projeto irmão `studycentralscraper`)
- FSRS ou qualquer algoritmo de SRS mais sofisticado que SM-2
- Sincronização offline com resolução de conflito (cache PWA simples resolve
  por agora)
- Verificação de e-mail, "esqueci minha senha", listagem de sessões ativas —
  fora do primeiro corte de autenticação, entram se pedidas
- Papel de admin/dono sobre catálogo e questões compartilhadas — qualquer
  conta premium pode editar/apagar qualquer eixo/banca/concurso/questão hoje;
  risco aceito e documentado, não esquecido
- Cobrança real (Stripe, Mercado Pago etc.) — o endpoint admin
  (`POST /api/admin/users/:id/plan`) é o substituto temporário

## Contratos por domínio

Cada pacote abaixo tem responsabilidade, importações permitidas e limites
explícitos. Ao gerar ou alterar código, verificar que a mudança respeita o
contrato do pacote em que está entrando — se uma tarefa parecer exigir violar
um contrato (ex: `catalog` precisando importar `question`), isso é sinal para
parar e perguntar, não para improvisar uma exceção.

### `platform`
**Responsabilidade**: config (env vars, incluindo `JWT_SECRET`/`ADMIN_SECRET`
obrigatórios), conexão com o banco (Gorm setup), router (Gin), middleware
genérico (logging, recovery, CORS), JWT (assinar/validar access token),
rate limit em memória, e os middlewares de autorização: `RequireAuth`
(exige token válido, grava `user_id`/`plan` no contexto), `RequirePremium`
(403 se o plano não for premium), `RequireAdminSecret` (protege `/api/admin`).
**Pode importar**: nada de outros domínios.
**Não pode**: conter regra de negócio de nenhum domínio, nem SQL de domínio.

### `auth`
**Responsabilidade**: `User` (nome, email, hash de senha, plano) e
`RefreshToken` (sessão de longa duração, revogável, guardado como hash).
Cadastro, login, emissão/rotação de refresh token, promoção de plano.
Login nunca diferencia "conta não existe" de "senha errada" (mensagem e
tempo de resposta genéricos — proteção contra enumeração de e-mail). Reuso de
um refresh token já rotacionado revoga todas as sessões daquela conta.
**Pode importar**: `platform` (JWT, hashing não — bcrypt é usado direto aqui).
**Não pode importar**: `catalog`, `question`, `flashcard`, `dashboard`.

### `catalog`
**Responsabilidade**: `Subject` (eixo temático, hierárquico via `parent_id`),
`Banca`, `Exam`. CRUD simples. **Compartilhado entre todas as contas** — sem
`user_id`, é a base de dados de referência que outros domínios apontam pra cá,
nunca o contrário.
**Pode importar**: `platform`.
**Não pode importar**: `auth`, `question`, `flashcard`, `dashboard`. Se uma
feature de `catalog` parecer precisar disso, o problema está em outro lugar.

### `question`
**Responsabilidade**: `Question` (statement, alternatives jsonb,
correct_answer, format certo_errado|multipla_escolha — **compartilhada entre
contas**, sem `user_id`) e `Attempt` (tentativa, `is_correct`, `confidence`:
certeza|duvida|chute — este campo é obrigatório, não opcional, porque
alimenta a lógica de priorização de flashcards — **por conta**, com `user_id`).
**Pode importar**: `platform`, `catalog` (para validar `subject_id`, `banca_id`,
`exam_id` existentes).
**Não pode importar**: `auth`, `flashcard`, `dashboard`.

### `flashcard`
**Responsabilidade**: `Flashcard` (front, back, `kind`:
pergunta_resposta|resumo, `source_question_id` opcional) e
`FlashcardReview` (estado do SM-2: `due_date`, `interval_days`,
`ease_factor`, `reps`, `lapses`). Contém `sm2.go` — a função `Schedule(state,
grade, now)` deve ser pura (sem I/O, sem Gorm, sem Gin), testável isoladamente.
**Pode importar**: `platform`, `catalog`, `question` (para vincular
`source_question_id` e ler o `subject_id` da questão de origem).
**Não pode importar**: `dashboard`.
**Regra do algoritmo**: SM-2 clássico, não FSRS. Ease mínimo de 1.3. Intervalo
mínimo de 1 dia mesmo em cards novos (interval=0 nunca deve multiplicar por
zero e travar o card).

### `dashboard`
**Responsabilidade**: o único domínio autorizado a **ler de todos os outros**
para compor agregações. Contém duas coisas centrais:
- `Overview` — taxa de acerto por concurso/eixo, contagem de flashcards
  vencidos vs maduros, volume de questões respondidas (7/30 dias).
- `BuildQueue(candidates, stats, minutes)` — combina os três critérios de
  priorização (vencimento > eixo pouco estudado > mais erros), determinístico,
  cada item retorna `reasons` explicando por que entrou na fila.
**Pode importar**: `platform`, `catalog`, `question`, `flashcard`.
**Não pode**: escrever dados de outros domínios — é só leitura e composição.
Se uma funcionalidade nova exigir escrever em `question` ou `flashcard`
a partir de `dashboard`, o código pertence a outro lugar.

### Regra geral entre domínios

```
platform ← auth
platform ← catalog ← question ← flashcard ← dashboard
```

`auth` e `catalog` são irmãos: os dois só dependem de `platform`, nenhum dos
dois depende do outro. Cada seta é "pode ser importado por". Uma importação
na direção contrária (ex: `catalog` importando `question`, ou qualquer
domínio importando `auth`) é sempre um erro de design, não uma exceção
válida — pare e avise antes de implementar.

## Convenções de código Go

- Nomes de pacote em minúsculo, sem underscore, sem plural (`question`, não
  `questions` ou `Questions`)
- Evitar pacotes genéricos tipo `utils` ou `common`
- Cada domínio expõe apenas o necessário (letra maiúscula = público); manter
  privado (minúsculo) tudo que for detalhe de implementação interna
- Testes unitários prioritários para `flashcard/sm2.go` e para `BuildQueue`
  (lógica de negócio pura, sem I/O)

## Tom de colaboração

Leandro está aprendendo Go pela primeira vez vindo de um background forte em
Django/Python. Ao gerar código, comente decisões que sejam específicas do
idioma Go quando divergirem de convenções de Django/Python (ex: por que não
existe um "admin panel" automático, por que erros são valores e não exceções).
Não assuma conhecimento prévio de convenções Go, mas também não repita
explicações básicas de Go já cobertas em sessões anteriores dentro do mesmo
projeto.