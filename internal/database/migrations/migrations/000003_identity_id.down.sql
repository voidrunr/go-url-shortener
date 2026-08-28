ALTER TABLE shortener_urls ALTER COLUMN id DROP IDENTITY IF EXISTS;
CREATE SEQUENCE IF NOT EXISTS shortener_urls_id_seq;
ALTER SEQUENCE shortener_urls_id_seq OWNED BY shortener_urls.id;
ALTER TABLE shortener_urls ALTER COLUMN id SET DEFAULT nextval('shortener_urls_id_seq');
SELECT setval('shortener_urls_id_seq', GREATEST((SELECT MAX(id) FROM shortener_urls), 1));
