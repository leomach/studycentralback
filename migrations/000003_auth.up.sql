-- Usuário-placeholder pré-auth (sem senha, nada mais referencia ele — banco
-- local confirmado vazio além dele): removido. Contas reais nascem via
-- POST /api/auth/register a partir de agora.
--
-- Nota: se este banco já tivesse usuários reais em produção, este DELETE
-- seria destrutivo — o caminho certo ali seria backfill de senha por conta
-- antes do NOT NULL, não apagar a linha.
DELETE FROM users WHERE id = 1;

ALTER TABLE users
    ADD COLUMN password_hash TEXT NOT NULL,
    ADD COLUMN plan TEXT NOT NULL DEFAULT 'free' CHECK (plan IN ('free', 'premium'));
ALTER TABLE users ALTER COLUMN email SET NOT NULL;

-- Refresh tokens: nunca guardamos o token em texto puro, só o hash (igual
-- senha) — um vazamento do banco não dá acesso direto às contas.
CREATE TABLE refresh_tokens (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    token_hash TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_refresh_tokens_user ON refresh_tokens (user_id);

-- subjects e questions viram conteúdo compartilhado entre contas — o mesmo
-- concurso/eixo/questão serve pra qualquer usuário estudando aquele edital.
-- bancas e exams já eram assim desde o início.
DROP INDEX IF EXISTS idx_subjects_user;
ALTER TABLE subjects DROP COLUMN user_id;

DROP INDEX IF EXISTS idx_questions_user_subject;
DROP INDEX IF EXISTS idx_questions_user_exam;
ALTER TABLE questions DROP COLUMN user_id;

CREATE INDEX idx_questions_subject ON questions (subject_id);
CREATE INDEX idx_questions_exam ON questions (exam_id);
