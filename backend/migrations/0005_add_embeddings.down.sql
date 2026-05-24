DROP INDEX IF EXISTS idx_user_answers_embedding;

ALTER TABLE user_answers
    DROP COLUMN IF EXISTS embedding;

-- The vector extension is left installed: other tables added in later
-- migrations may depend on it. Drop it manually if no other consumers.
