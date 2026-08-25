CREATE UNIQUE INDEX IF NOT EXISTS idx_shortener_urls_original_url
    ON shortener_urls (original_url);
