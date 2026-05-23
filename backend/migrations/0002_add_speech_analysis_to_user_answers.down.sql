ALTER TABLE user_answers
    DROP COLUMN IF EXISTS filler_count,
    DROP COLUMN IF EXISTS words_per_minute,
    DROP COLUMN IF EXISTS long_pause_count;
