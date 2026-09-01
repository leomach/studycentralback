-- Idempotência para escritas offline (outbox do PWA): o cliente gera um UUID
-- por evento e o servidor ignora retentativas com o mesmo id.
--
-- attempts é log de eventos (uma linha por tentativa): client_id único
-- garante que reenviar a mesma tentativa não duplica a linha.
ALTER TABLE attempts ADD COLUMN client_id TEXT;
CREATE UNIQUE INDEX idx_attempts_client_id ON attempts (client_id);

-- flashcard_reviews é estado atual do card (uma linha por flashcard), não log:
-- aqui basta lembrar o último client_id aplicado para não reprocessar o SM-2
-- duas vezes sobre a mesma avaliação.
ALTER TABLE flashcard_reviews ADD COLUMN last_client_id TEXT;
