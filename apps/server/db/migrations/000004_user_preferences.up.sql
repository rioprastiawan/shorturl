ALTER TABLE users
    ADD COLUMN language VARCHAR(5) NOT NULL DEFAULT 'en',
    ADD COLUMN timezone VARCHAR(64) NOT NULL DEFAULT 'UTC';

ALTER TABLE users
    ADD CONSTRAINT users_language_check CHECK (language IN ('en', 'id'));
