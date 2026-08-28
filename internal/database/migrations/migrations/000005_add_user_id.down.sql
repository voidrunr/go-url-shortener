DROP INDEX IF EXISTS idx_shortener_urls_user_id;

ALTER TABLE shortener_urls
    DROP COLUMN IF EXISTS user_id;
