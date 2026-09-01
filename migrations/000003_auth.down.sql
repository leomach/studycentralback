DROP INDEX IF EXISTS idx_questions_exam;
DROP INDEX IF EXISTS idx_questions_subject;
ALTER TABLE questions ADD COLUMN user_id BIGINT;
CREATE INDEX idx_questions_user_subject ON questions (user_id, subject_id);
CREATE INDEX idx_questions_user_exam ON questions (user_id, exam_id);

ALTER TABLE subjects ADD COLUMN user_id BIGINT;
CREATE INDEX idx_subjects_user ON subjects (user_id);

DROP TABLE IF EXISTS refresh_tokens;

ALTER TABLE users ALTER COLUMN email DROP NOT NULL;
ALTER TABLE users DROP COLUMN plan;
ALTER TABLE users DROP COLUMN password_hash;
