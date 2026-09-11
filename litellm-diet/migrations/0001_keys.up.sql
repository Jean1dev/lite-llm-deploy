CREATE TABLE keys (
    hash TEXT PRIMARY KEY,
    alias TEXT NOT NULL DEFAULT '',
    models TEXT[] NOT NULL DEFAULT '{}',
    spend DOUBLE PRECISION NOT NULL DEFAULT 0,
    max_budget DOUBLE PRECISION,
    budget_duration TEXT NOT NULL DEFAULT '',
    budget_reset_at TIMESTAMPTZ,
    last_active TIMESTAMPTZ,
    expires_at TIMESTAMPTZ,
    blocked BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
