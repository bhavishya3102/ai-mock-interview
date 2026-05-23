ALTER TABLE user_answers
    ADD COLUMN filler_count     integer NOT NULL DEFAULT 0 CHECK (filler_count >= 0),
    ADD COLUMN words_per_minute integer NOT NULL DEFAULT 0 CHECK (words_per_minute >= 0),
    ADD COLUMN long_pause_count integer NOT NULL DEFAULT 0 CHECK (long_pause_count >= 0);
