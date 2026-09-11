-- migration.sql
CREATE TABLE IF NOT EXISTS contacts (
    id SERIAL PRIMARY KEY, 
    phone VARCHAR UNIQUE, 
    name VARCHAR, 
    company VARCHAR, 
    opt_out BOOLEAN NOT NULL DEFAULT FALSE,
    joined_at TIMESTAMP DEFAULT NOW()
);

-- contacts.opt_out is read by GetAllContacts and GetBroadcastRecipients and
-- written by the opt-out toggle, but no migration ever created it — so
-- GET /api/contacts failed on every call and returned an empty list with a
-- 200, and broadcasts selected nobody. Added here for new databases; the
-- ALTER covers databases created before this line existed.
ALTER TABLE contacts ADD COLUMN IF NOT EXISTS opt_out BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS leads (
    id SERIAL PRIMARY KEY, 
    phone VARCHAR, 
    name VARCHAR, 
    company VARCHAR, 
    requirement TEXT, 
    contact_phone VARCHAR, 
    status VARCHAR DEFAULT 'new', 
    notes TEXT, 
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS callbacks (
    id SERIAL PRIMARY KEY, 
    phone VARCHAR, 
    name VARCHAR, 
    preferred_time VARCHAR, 
    status VARCHAR DEFAULT 'pending', 
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS messages_log (
    id SERIAL PRIMARY KEY, 
    phone VARCHAR, 
    direction VARCHAR, 
    message TEXT, 
    wa_msg_id VARCHAR UNIQUE, 
    sent_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS campaigns (
    id SERIAL PRIMARY KEY,
    type VARCHAR NOT NULL,           -- 'quiz' or 'poster'
    -- Quiz fields
    question TEXT,
    option_a VARCHAR(255),
    option_b VARCHAR(255),
    option_c VARCHAR(255),
    correct_answer CHAR(1),
    explanation TEXT,
    youtube_link VARCHAR(255),
    -- Poster fields
    image_url VARCHAR(255),
    caption TEXT,
    -- Scheduling & state
    scheduled_at TIMESTAMP NOT NULL,
    status VARCHAR DEFAULT 'scheduled', -- scheduled | sending | sent | cancelled
    total_sent INT DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW()
);

-- A poster's reply buttons, as [{"id": ..., "title": ...}]. NULL means the
-- poster predates custom buttons and goes out with the original pair.
ALTER TABLE campaigns ADD COLUMN IF NOT EXISTS buttons JSONB;

CREATE TABLE IF NOT EXISTS quizzes (
    id SERIAL PRIMARY KEY,
    campaign_id INT REFERENCES campaigns(id),
    question TEXT NOT NULL,
    option_a VARCHAR(255) NOT NULL,
    option_b VARCHAR(255) NOT NULL,
    option_c VARCHAR(255) NOT NULL,
    correct_answer CHAR(1) NOT NULL,
    explanation TEXT,
    youtube_link VARCHAR(255),
    is_active BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS quiz_responses (
    id SERIAL PRIMARY KEY,
    quiz_id INT REFERENCES quizzes(id),
    phone VARCHAR NOT NULL,
    answer CHAR(1) NOT NULL,
    is_correct BOOLEAN NOT NULL,
    responded_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(quiz_id, phone)
);
CREATE TABLE IF NOT EXISTS customer_queries (
    id SERIAL PRIMARY KEY,
    phone VARCHAR,
    name VARCHAR,
    category VARCHAR,
    original_message TEXT,
    status VARCHAR DEFAULT 'open',
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS faqs (
    id SERIAL PRIMARY KEY,
    keywords TEXT NOT NULL,          -- comma separated keywords
    answer TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

-- ─────────────────────────────────────────────────────────────────────────
-- Timestamps carry a time zone.
--
-- Every timestamp column was `timestamp without time zone` and was written
-- with now(), which returns the SERVER's local time. pgx then scanned those
-- naive values as UTC and Go marshalled them with a trailing "Z", so the API
-- claimed a local time was a UTC one. On a machine running Asia/Kolkata every
-- timestamp in the product arrived 5h30m in the future — recent leads read
-- "in about 1 hour", and any report bucketed by day was wrong near midnight.
--
-- The USING clause reads each stored value as local time, which is what it
-- always was. On a server already running UTC this is a no-op, so the same
-- migration is correct for both this machine and the deployment.
-- ─────────────────────────────────────────────────────────────────────────
ALTER TABLE attendance       ALTER COLUMN check_in      TYPE TIMESTAMPTZ USING check_in      AT TIME ZONE current_setting('TimeZone');
ALTER TABLE attendance       ALTER COLUMN check_out     TYPE TIMESTAMPTZ USING check_out     AT TIME ZONE current_setting('TimeZone');
ALTER TABLE callbacks        ALTER COLUMN created_at    TYPE TIMESTAMPTZ USING created_at    AT TIME ZONE current_setting('TimeZone');
ALTER TABLE campaigns        ALTER COLUMN created_at    TYPE TIMESTAMPTZ USING created_at    AT TIME ZONE current_setting('TimeZone');
ALTER TABLE campaigns        ALTER COLUMN scheduled_at  TYPE TIMESTAMPTZ USING scheduled_at  AT TIME ZONE current_setting('TimeZone');
ALTER TABLE contacts         ALTER COLUMN joined_at     TYPE TIMESTAMPTZ USING joined_at     AT TIME ZONE current_setting('TimeZone');
ALTER TABLE customer_queries ALTER COLUMN created_at    TYPE TIMESTAMPTZ USING created_at    AT TIME ZONE current_setting('TimeZone');
ALTER TABLE faqs             ALTER COLUMN created_at    TYPE TIMESTAMPTZ USING created_at    AT TIME ZONE current_setting('TimeZone');
ALTER TABLE leads            ALTER COLUMN created_at    TYPE TIMESTAMPTZ USING created_at    AT TIME ZONE current_setting('TimeZone');
ALTER TABLE messages_log     ALTER COLUMN sent_at       TYPE TIMESTAMPTZ USING sent_at       AT TIME ZONE current_setting('TimeZone');
ALTER TABLE quiz_responses   ALTER COLUMN responded_at  TYPE TIMESTAMPTZ USING responded_at  AT TIME ZONE current_setting('TimeZone');
ALTER TABLE quizzes          ALTER COLUMN created_at    TYPE TIMESTAMPTZ USING created_at    AT TIME ZONE current_setting('TimeZone');
ALTER TABLE reminders        ALTER COLUMN due_at        TYPE TIMESTAMPTZ USING due_at        AT TIME ZONE current_setting('TimeZone');

-- ── Console sign-in ─────────────────────────────────────────────────────────
-- Accounts that can sign in to the admin panel. Only the bcrypt hash is kept;
-- the password itself is never stored or logged. The first account is seeded
-- from ADMIN_EMAIL / ADMIN_PASSWORD on a fresh deployment.
CREATE TABLE IF NOT EXISTS admin_users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(320) NOT NULL UNIQUE,
    password_hash TEXT NOT NULL,
    created_at TIMESTAMPTZ DEFAULT NOW()
);

-- ── Indexes ─────────────────────────────────────────────────────────────────
-- messages_log had none. The inbox filters by phone and the dashboard filters
-- by date, so both were sequential scans over a table that only ever grows.
CREATE INDEX IF NOT EXISTS idx_messages_log_phone_id ON messages_log (phone, id);
CREATE INDEX IF NOT EXISTS idx_messages_log_sent_at  ON messages_log (sent_at);
CREATE INDEX IF NOT EXISTS idx_leads_created_at      ON leads (created_at);
CREATE INDEX IF NOT EXISTS idx_leads_status          ON leads (status);
CREATE INDEX IF NOT EXISTS idx_callbacks_status      ON callbacks (status);
CREATE INDEX IF NOT EXISTS idx_campaigns_due         ON campaigns (status, scheduled_at);
