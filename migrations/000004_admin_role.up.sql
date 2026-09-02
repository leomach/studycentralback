-- Papel de administrador da própria conta (gerenciar usuários/planos) — não
-- confundir com o "papel de dono do catálogo" que o CLAUDE.md documenta como
-- fora de escopo (aquele é sobre editar eixo/banca/concurso/questão
-- compartilhados; este é sobre administrar contas).
ALTER TABLE users ADD COLUMN is_admin BOOLEAN NOT NULL DEFAULT FALSE;
