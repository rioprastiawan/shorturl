ALTER TABLE links
    DROP CONSTRAINT links_created_via_valid;

ALTER TABLE links
    ADD CONSTRAINT links_created_via_valid
        CHECK (created_via IN ('dashboard', 'api'));
