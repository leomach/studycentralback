-- Schema inicial da Central de Estudos: as 8 tabelas do MVP.
--
-- users existe desde o começo e user_id é foreign key real em toda tabela de
-- conteúdo do estudo, mesmo com um único usuário. Não há autenticação ainda:
-- o registro seed (id=1) representa o Leandro e platform.CurrentUser() é o
-- ponto único de mudança quando houver login.

CREATE TABLE users (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT        NOT NULL,
    email      TEXT        UNIQUE, -- nulo enquanto não existir login real
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Usuário único do MVP. O setval evita que o próximo INSERT tente reusar o
-- id=1: a sequência não avança quando o id é informado à mão.
INSERT INTO users (id, name) VALUES (1, 'Leandro');
SELECT setval(pg_get_serial_sequence('users', 'id'), 1, true);

CREATE TABLE subjects (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    parent_id  BIGINT      REFERENCES subjects (id) ON DELETE SET NULL,
    name       TEXT        NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_subjects_user ON subjects (user_id);
CREATE INDEX idx_subjects_parent ON subjects (parent_id);

-- bancas e exams são dados de referência compartilhados, não conteúdo de um
-- usuário: não levam user_id.
CREATE TABLE bancas (
    id         BIGSERIAL PRIMARY KEY,
    name       TEXT        NOT NULL UNIQUE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE exams (
    id         BIGSERIAL PRIMARY KEY,
    banca_id   BIGINT      NOT NULL REFERENCES bancas (id) ON DELETE RESTRICT,
    name       TEXT        NOT NULL,
    year       INTEGER     NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_exams_banca ON exams (banca_id);

CREATE TABLE questions (
    id             BIGSERIAL PRIMARY KEY,
    user_id        BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    subject_id     BIGINT      NOT NULL REFERENCES subjects (id) ON DELETE RESTRICT,
    banca_id       BIGINT      REFERENCES bancas (id) ON DELETE SET NULL,
    exam_id        BIGINT      REFERENCES exams (id) ON DELETE SET NULL,
    format         TEXT        NOT NULL DEFAULT 'multipla_escolha'
                   CHECK (format IN ('multipla_escolha', 'certo_errado')),
    statement      TEXT        NOT NULL,
    alternatives   JSONB       NOT NULL DEFAULT '[]'::jsonb,
    correct_answer TEXT        NOT NULL,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_questions_user_subject ON questions (user_id, subject_id);
CREATE INDEX idx_questions_user_exam ON questions (user_id, exam_id);
CREATE INDEX idx_questions_banca ON questions (banca_id);

CREATE TABLE attempts (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    question_id BIGINT      NOT NULL REFERENCES questions (id) ON DELETE CASCADE,
    answer      TEXT        NOT NULL,
    is_correct  BOOLEAN     NOT NULL,
    -- confidence é o campo que dá sentido ao acerto: acertar no chute e errar
    -- com certeza são sinais opostos, e a fila do dia depende dessa diferença.
    confidence  TEXT        NOT NULL
                CHECK (confidence IN ('certeza', 'duvida', 'chute')),
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_attempts_user_question ON attempts (user_id, question_id);
CREATE INDEX idx_attempts_created ON attempts (user_id, created_at DESC);

CREATE TABLE flashcards (
    id                 BIGSERIAL PRIMARY KEY,
    user_id            BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    subject_id         BIGINT      NOT NULL REFERENCES subjects (id) ON DELETE RESTRICT,
    kind               TEXT        NOT NULL DEFAULT 'pergunta_resposta'
                       CHECK (kind IN ('pergunta_resposta', 'resumo')),
    front              TEXT        NOT NULL,
    back               TEXT        NOT NULL,
    source_question_id BIGINT      REFERENCES questions (id) ON DELETE SET NULL,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_flashcards_user_subject ON flashcards (user_id, subject_id);

-- Estado do SM-2: uma linha por card.
CREATE TABLE flashcard_reviews (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    flashcard_id  BIGINT      NOT NULL UNIQUE REFERENCES flashcards (id) ON DELETE CASCADE,
    due_date      TIMESTAMPTZ NOT NULL DEFAULT now(),
    interval_days INTEGER     NOT NULL DEFAULT 0,
    ease_factor   DOUBLE PRECISION NOT NULL DEFAULT 2.5,
    reps          INTEGER     NOT NULL DEFAULT 0,
    lapses        INTEGER     NOT NULL DEFAULT 0,
    last_grade    INTEGER     NOT NULL DEFAULT 0,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Índice que a fila do dia usa a cada requisição.
CREATE INDEX idx_flashcard_reviews_due ON flashcard_reviews (user_id, due_date);
