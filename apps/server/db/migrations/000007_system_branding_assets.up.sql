CREATE TABLE system_branding_assets (
    kind         VARCHAR(32) PRIMARY KEY,
    content_type VARCHAR(64) NOT NULL,
    data         BYTEA       NOT NULL,
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT system_branding_asset_kind_valid
        CHECK (kind IN ('logo_light', 'logo_dark', 'logo_compact', 'favicon'))
);

CREATE TRIGGER system_branding_assets_set_updated_at BEFORE UPDATE ON system_branding_assets
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
