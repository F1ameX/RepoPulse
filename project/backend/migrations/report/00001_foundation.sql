-- +goose Up
CREATE SCHEMA report;

CREATE TABLE report.reports (
    id uuid PRIMARY KEY,
    analysis_id uuid NOT NULL UNIQUE REFERENCES analyzer.analyses(id),
    snapshot_id uuid NOT NULL UNIQUE REFERENCES analyzer.snapshots(id),
    owner_id uuid NOT NULL REFERENCES auth.principals(id),
    completed_at timestamptz NOT NULL,
    overall_score numeric,
    payload jsonb NOT NULL,
    freshness_key jsonb,
    freshness_hash text,
    freshness_confirmed boolean NOT NULL DEFAULT false
);

CREATE INDEX reports_history ON report.reports(owner_id, completed_at DESC, id);
CREATE INDEX reports_freshness ON report.reports(owner_id, freshness_hash) WHERE freshness_confirmed;

CREATE TABLE report.history_views (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES auth.principals(id),
    created_at timestamptz NOT NULL
);

CREATE TABLE report.history_view_items (
    view_id uuid NOT NULL REFERENCES report.history_views(id),
    position bigint NOT NULL,
    report_id uuid NOT NULL REFERENCES report.reports(id),
    PRIMARY KEY (view_id, position),
    UNIQUE (view_id, report_id)
);

-- +goose Down
DROP INDEX report.reports_freshness;
DROP INDEX report.reports_history;
DROP TABLE report.history_view_items;
DROP TABLE report.history_views;
DROP TABLE report.reports;
DROP SCHEMA report;
