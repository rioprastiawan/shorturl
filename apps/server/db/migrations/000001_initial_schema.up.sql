-- Initial schema for ShortURL.
--
-- Conventions:
--   * Public-facing identifiers are UUIDs.
--   * Hostnames and emails are compared case-insensitively via LOWER() indexes.
--   * Every mutable table carries updated_at, maintained by a trigger rather
--     than by application code, so a missed UPDATE cannot leave it stale.

CREATE OR REPLACE FUNCTION set_updated_at() RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;


-- ============================================================================
-- Users and sessions
-- ============================================================================

CREATE TABLE users (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(120) NOT NULL,
    email         VARCHAR(255) NOT NULL,
    password_hash TEXT         NOT NULL,
    is_admin      BOOLEAN      NOT NULL DEFAULT FALSE,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- Case-insensitive uniqueness: Alice@x.com and alice@x.com are one account.
CREATE UNIQUE INDEX users_email_unique ON users (LOWER(email));

CREATE TRIGGER users_set_updated_at BEFORE UPDATE ON users
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();


-- Server-side sessions. Only the hash of the cookie value is stored, so a
-- database leak does not hand out live sessions.
CREATE TABLE sessions (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash   TEXT        NOT NULL,
    user_agent   TEXT,
    ip_hash      VARCHAR(64),
    expires_at   TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX sessions_token_hash_unique ON sessions (token_hash);
CREATE INDEX sessions_user_idx ON sessions (user_id);
CREATE INDEX sessions_expires_idx ON sessions (expires_at);


-- ============================================================================
-- Workspaces and membership
-- ============================================================================

CREATE TABLE workspaces (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name          VARCHAR(120) NOT NULL,
    slug          VARCHAR(64)  NOT NULL,
    owner_user_id UUID         NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX workspaces_slug_unique ON workspaces (LOWER(slug));
CREATE INDEX workspaces_owner_idx ON workspaces (owner_user_id);

CREATE TRIGGER workspaces_set_updated_at BEFORE UPDATE ON workspaces
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();


CREATE TABLE workspace_members (
    workspace_id UUID        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    user_id      UUID        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role         VARCHAR(16) NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (workspace_id, user_id),
    CONSTRAINT workspace_members_role_valid
        CHECK (role IN ('owner', 'admin', 'member'))
);

-- Drives "list my workspaces", which runs on nearly every dashboard load.
CREATE INDEX workspace_members_user_idx ON workspace_members (user_id, workspace_id);

CREATE TRIGGER workspace_members_set_updated_at BEFORE UPDATE ON workspace_members
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();


-- ============================================================================
-- Domains
-- ============================================================================

CREATE TABLE domains (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id        UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    hostname            VARCHAR(255) NOT NULL,
    status              VARCHAR(16)  NOT NULL DEFAULT 'pending',
    verification_token  VARCHAR(64)  NOT NULL,
    verification_method VARCHAR(16)  NOT NULL DEFAULT 'dns_txt',
    verification_error  TEXT,
    ssl_status          VARCHAR(16)  NOT NULL DEFAULT 'pending',
    is_default          BOOLEAN      NOT NULL DEFAULT FALSE,
    verified_at         TIMESTAMPTZ,
    last_checked_at     TIMESTAMPTZ,
    created_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT domains_status_valid
        CHECK (status IN ('pending', 'verifying', 'active', 'failed', 'disabled')),
    CONSTRAINT domains_ssl_status_valid
        CHECK (ssl_status IN ('pending', 'active', 'failed')),
    CONSTRAINT domains_verification_method_valid
        CHECK (verification_method IN ('dns_txt', 'dns_cname', 'dns_a'))
);

-- Globally unique: a hostname resolves to exactly one workspace, otherwise
-- redirect lookup by Host header would be ambiguous.
CREATE UNIQUE INDEX domains_hostname_unique ON domains (LOWER(hostname));
CREATE INDEX domains_workspace_idx ON domains (workspace_id, created_at DESC);

-- At most one default domain per workspace.
CREATE UNIQUE INDEX domains_one_default_per_workspace
    ON domains (workspace_id) WHERE is_default;

CREATE TRIGGER domains_set_updated_at BEFORE UPDATE ON domains
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();


-- ============================================================================
-- Links
-- ============================================================================

CREATE TABLE links (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id       UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    domain_id          UUID         NOT NULL REFERENCES domains(id) ON DELETE CASCADE,
    slug               VARCHAR(128) NOT NULL,
    destination_url    TEXT         NOT NULL,
    title              VARCHAR(255),
    status             VARCHAR(16)  NOT NULL DEFAULT 'active',
    redirect_type      SMALLINT     NOT NULL DEFAULT 302,
    password_hash      TEXT,
    expires_at         TIMESTAMPTZ,
    max_clicks         BIGINT,
    click_count        BIGINT       NOT NULL DEFAULT 0,
    external_reference VARCHAR(255),
    metadata           JSONB,
    created_by         UUID         REFERENCES users(id) ON DELETE SET NULL,
    created_via        VARCHAR(16)  NOT NULL DEFAULT 'dashboard',
    created_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ  NOT NULL DEFAULT now(),

    CONSTRAINT links_status_valid
        CHECK (status IN ('active', 'disabled', 'archived')),
    CONSTRAINT links_redirect_type_valid
        CHECK (redirect_type IN (301, 302, 307, 308)),
    CONSTRAINT links_created_via_valid
        CHECK (created_via IN ('dashboard', 'api')),
    CONSTRAINT links_max_clicks_positive
        CHECK (max_clicks IS NULL OR max_clicks > 0)
);

CREATE UNIQUE INDEX links_domain_slug_unique ON links (domain_id, slug);
CREATE INDEX links_workspace_created_idx ON links (workspace_id, created_at DESC);
CREATE INDEX links_domain_idx ON links (domain_id);

-- Lets an integration ask "did I already shorten invoice:12345?" and makes a
-- duplicate call a conflict rather than a second link.
CREATE UNIQUE INDEX links_workspace_external_reference_unique
    ON links (workspace_id, external_reference)
    WHERE external_reference IS NOT NULL;

-- Backs dashboard search over title and destination.
CREATE INDEX links_search_idx ON links
    USING gin (to_tsvector('simple', coalesce(title, '') || ' ' || destination_url));

CREATE TRIGGER links_set_updated_at BEFORE UPDATE ON links
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();


-- ============================================================================
-- Analytics
-- ============================================================================

-- Raw click log, kept for detail views and export.
--
-- The primary key is a bigint identity rather than the UUID the plan sketched:
-- this is the highest-write table in the system and is never addressed by a
-- public identifier, and random UUID keys fragment the B-tree badly at volume.
--
-- workspace_id is denormalised so every analytics query can avoid a join.
CREATE TABLE click_events (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    link_id       UUID        NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    workspace_id  UUID        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    clicked_at    TIMESTAMPTZ NOT NULL,
    ip_hash       VARCHAR(64),
    country       VARCHAR(2),
    city          VARCHAR(120),
    device        VARCHAR(16),
    os            VARCHAR(48),
    browser       VARCHAR(48),
    referrer_host VARCHAR(255),
    referrer      TEXT,
    utm_source    VARCHAR(120),
    utm_medium    VARCHAR(120),
    utm_campaign  VARCHAR(120)
);

CREATE INDEX click_events_link_clicked_idx ON click_events (link_id, clicked_at DESC);
CREATE INDEX click_events_workspace_clicked_idx ON click_events (workspace_id, clicked_at DESC);


-- Pre-aggregated counters, maintained by the analytics worker as it drains the
-- Redis stream. Dashboard queries read these instead of scanning click_events,
-- which is what keeps analytics fast on PostgreSQL: a 90-day chart reads about
-- 2,000 rows per link rather than every click ever recorded.
CREATE TABLE click_hourly (
    workspace_id UUID        NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    link_id      UUID        NOT NULL REFERENCES links(id) ON DELETE CASCADE,
    bucket       TIMESTAMPTZ NOT NULL,
    clicks       BIGINT      NOT NULL DEFAULT 0,

    PRIMARY KEY (link_id, bucket)
);

CREATE INDEX click_hourly_workspace_bucket_idx ON click_hourly (workspace_id, bucket DESC);


-- Dimensional breakdowns (referrer, utm_source, device, ...) rolled up per day.
-- One generic table rather than one per dimension: adding a dimension later
-- becomes a new value in this column instead of a migration.
CREATE TABLE click_dimension_daily (
    workspace_id UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    day          DATE         NOT NULL,
    dimension    VARCHAR(24)  NOT NULL,
    value        VARCHAR(255) NOT NULL,
    clicks       BIGINT       NOT NULL DEFAULT 0,

    PRIMARY KEY (workspace_id, day, dimension, value)
);

CREATE INDEX click_dimension_daily_lookup_idx
    ON click_dimension_daily (workspace_id, dimension, day DESC);


-- ============================================================================
-- API keys and idempotency
-- ============================================================================

CREATE TABLE api_keys (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name         VARCHAR(120) NOT NULL,
    key_prefix   VARCHAR(32)  NOT NULL,
    key_hash     TEXT         NOT NULL,
    scopes       TEXT[]       NOT NULL DEFAULT ARRAY['links:read', 'links:write'],
    last_used_at TIMESTAMPTZ,
    expires_at   TIMESTAMPTZ,
    revoked_at   TIMESTAMPTZ,
    created_by   UUID         REFERENCES users(id) ON DELETE SET NULL,
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);

-- The prefix is the lookup key on every authenticated API call, so it must
-- identify exactly one row.
CREATE UNIQUE INDEX api_keys_prefix_unique ON api_keys (key_prefix);
CREATE INDEX api_keys_workspace_idx ON api_keys (workspace_id, created_at DESC);


CREATE TABLE idempotency_keys (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id    UUID         NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    idempotency_key VARCHAR(255) NOT NULL,
    request_hash    VARCHAR(64)  NOT NULL,
    link_id         UUID         REFERENCES links(id) ON DELETE CASCADE,
    created_at      TIMESTAMPTZ  NOT NULL DEFAULT now(),
    expires_at      TIMESTAMPTZ  NOT NULL,

    UNIQUE (workspace_id, idempotency_key)
);

CREATE INDEX idempotency_keys_expires_idx ON idempotency_keys (expires_at);


-- ============================================================================
-- Application state
-- ============================================================================

-- Setup completion lives here rather than in an environment variable so that
-- the "create the first administrator" endpoint cannot be reopened by editing
-- .env and restarting.
CREATE TABLE app_settings (
    key        VARCHAR(64) PRIMARY KEY,
    value      JSONB       NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TRIGGER app_settings_set_updated_at BEFORE UPDATE ON app_settings
    FOR EACH ROW EXECUTE FUNCTION set_updated_at();
