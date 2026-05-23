ALTER TABLE users
    DROP COLUMN IF EXISTS resume_text,
    DROP COLUMN IF EXISTS resume_uploaded_at;
