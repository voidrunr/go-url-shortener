DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1 FROM pg_attribute
        WHERE attrelid = 'shortener_urls'::regclass
          AND attname = 'id'
          AND attidentity <> ''
    ) THEN
        ALTER TABLE shortener_urls ALTER COLUMN id DROP DEFAULT;
        DROP SEQUENCE IF EXISTS shortener_urls_id_seq;
        ALTER TABLE shortener_urls ALTER COLUMN id ADD GENERATED ALWAYS AS IDENTITY;
        PERFORM setval(pg_get_serial_sequence('shortener_urls', 'id'),
                       GREATEST((SELECT MAX(id) FROM shortener_urls), 1));
    END IF;
END $$;
