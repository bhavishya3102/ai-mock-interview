CREATE TABLE coach_reports (
    id              uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    mock_id         text        NOT NULL UNIQUE REFERENCES mock_interviews(mock_id) ON DELETE CASCADE,
    content         text        NOT NULL,
    tokens_used     integer     NOT NULL CHECK (tokens_used >= 0),
    model           text        NOT NULL,
    created_at      timestamptz NOT NULL DEFAULT now()
);
