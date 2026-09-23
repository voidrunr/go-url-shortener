ALTER TABLE shortener_urls
    ADD COLUMN IF NOT EXISTS user_id VARCHAR(64) NOT NULL DEFAULT '';

CREATE INDEX IF NOT EXISTS idx_shortener_urls_user_id
    ON shortener_urls (user_id);
