CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    clerk_user_id   text        PRIMARY KEY,
    email           text,
    created_at      timestamptz NOT NULL DEFAULT now(),
    updated_at      timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE mock_interviews (
    id                  uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    mock_id             text        NOT NULL UNIQUE,
    clerk_user_id       text        NOT NULL REFERENCES users(clerk_user_id) ON DELETE CASCADE,
    job_position        text        NOT NULL,
    job_description     text        NOT NULL,
    years_experience    smallint    NOT NULL CHECK (years_experience BETWEEN 0 AND 60),
    questions           jsonb       NOT NULL,
    created_at          timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_mock_interviews_user_created
    ON mock_interviews (clerk_user_id, created_at DESC);

CREATE TABLE user_answers (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    mock_id         text        NOT NULL REFERENCES mock_interviews(mock_id) ON DELETE CASCADE,
    clerk_user_id   text        NOT NULL REFERENCES users(clerk_user_id) ON DELETE CASCADE,
    question_index  smallint    NOT NULL CHECK (question_index >= 0),
    question_text   text        NOT NULL,
    correct_answer  text        NOT NULL,
    user_answer     text        NOT NULL,
    rating          smallint    NOT NULL CHECK (rating BETWEEN 1 AND 10),
    feedback        text        NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now(),
    UNIQUE (mock_id, question_index, clerk_user_id)
);

CREATE INDEX idx_user_answers_mock ON user_answers (mock_id, question_index);
