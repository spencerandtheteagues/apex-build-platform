CREATE TABLE IF NOT EXISTS proposed_edits (
    id               TEXT        NOT NULL PRIMARY KEY,
    build_id         TEXT        NOT NULL,
    agent_id         TEXT        NOT NULL DEFAULT '',
    agent_role       TEXT        NOT NULL DEFAULT '',
    task_id          TEXT        NOT NULL DEFAULT '',
    file_path        TEXT        NOT NULL,
    original_content TEXT        NOT NULL DEFAULT '',
    proposed_content TEXT        NOT NULL DEFAULT '',
    language         TEXT        NOT NULL DEFAULT '',
    status           TEXT        NOT NULL DEFAULT 'pending',
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    reviewed_at      TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_proposed_edits_build_id ON proposed_edits (build_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_proposed_edits_status ON proposed_edits (build_id, status);
