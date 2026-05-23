ALTER TABLE users
    ADD COLUMN resume_text        text,
    ADD COLUMN resume_uploaded_at timestamptz;
