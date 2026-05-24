CREATE EXTENSION IF NOT EXISTS vector;

ALTER TABLE user_answers
    ADD COLUMN embedding vector(768);

CREATE INDEX idx_user_answers_embedding
    ON user_answers
    USING hnsw (embedding vector_cosine_ops);
