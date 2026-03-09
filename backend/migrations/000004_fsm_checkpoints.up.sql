-- FSM checkpoint persistence for agent build rollback support
CREATE TABLE IF NOT EXISTS fsm_checkpoints (
    id            TEXT        NOT NULL PRIMARY KEY,
    build_id      TEXT        NOT NULL,
    state         TEXT        NOT NULL,
    step_index    INTEGER     NOT NULL DEFAULT 0,
    description   TEXT        NOT NULL DEFAULT '',
    snapshot_json TEXT        NOT NULL DEFAULT '',
    can_restore   BOOLEAN     NOT NULL DEFAULT TRUE,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_fsm_checkpoints_build_id ON fsm_checkpoints (build_id, created_at DESC);
