ALTER TABLE flashcard_reviews DROP COLUMN IF EXISTS last_client_id;
DROP INDEX IF EXISTS idx_attempts_client_id;
ALTER TABLE attempts DROP COLUMN IF EXISTS client_id;
