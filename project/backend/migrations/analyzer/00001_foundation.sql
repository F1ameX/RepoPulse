-- +goose Up
CREATE SCHEMA analyzer;

CREATE TABLE analyzer.submissions (
    id uuid PRIMARY KEY,
    owner_id uuid NOT NULL REFERENCES auth.principals(id),
    request_id uuid NOT NULL,
    canonical_url text NOT NULL,
    assessment_at timestamptz NOT NULL,
    state text NOT NULL,
    analysis_id uuid,
    report_id uuid,
    error jsonb
);

CREATE UNIQUE INDEX submissions_owner_request ON analyzer.submissions(owner_id, request_id);

CREATE TABLE analyzer.analyses (
    id uuid PRIMARY KEY,
    submission_id uuid NOT NULL UNIQUE REFERENCES analyzer.submissions(id),
    owner_id uuid NOT NULL REFERENCES auth.principals(id),
    canonical_url text NOT NULL,
    repository jsonb NOT NULL,
    assessment_at timestamptz NOT NULL,
    window_from timestamptz NOT NULL,
    window_until timestamptz NOT NULL,
    versions jsonb NOT NULL,
    status text NOT NULL DEFAULT 'queued',
    stage text,
    created_at timestamptz NOT NULL,
    started_at timestamptz,
    deadline_at timestamptz,
    completed_at timestamptz,
    report_id uuid,
    error jsonb
);

CREATE INDEX analyses_queue ON analyzer.analyses(created_at, id) WHERE status = 'queued';
CREATE INDEX analyses_expired ON analyzer.analyses(deadline_at) WHERE status = 'running';

CREATE TABLE analyzer.snapshots (
    id uuid PRIMARY KEY,
    analysis_id uuid NOT NULL UNIQUE REFERENCES analyzer.analyses(id),
    owner_id uuid NOT NULL REFERENCES auth.principals(id),
    payload jsonb NOT NULL,
    created_at timestamptz NOT NULL
);

CREATE TABLE analyzer.snapshot_dependencies (
    snapshot_id uuid NOT NULL REFERENCES analyzer.snapshots(id),
    ecosystem text NOT NULL,
    package text NOT NULL,
    version text NOT NULL,
    origins jsonb NOT NULL,
    PRIMARY KEY (snapshot_id, ecosystem, package, version)
);

CREATE TABLE analyzer.snapshot_vulnerabilities (
    snapshot_id uuid NOT NULL,
    vulnerability_id text NOT NULL,
    ecosystem text NOT NULL,
    package text NOT NULL,
    version text NOT NULL,
    severity text,
    obtained_at timestamptz NOT NULL,
    evidence_url text,
    PRIMARY KEY (snapshot_id, vulnerability_id, ecosystem, package, version),
    FOREIGN KEY (snapshot_id, ecosystem, package, version)
        REFERENCES analyzer.snapshot_dependencies(snapshot_id, ecosystem, package, version)
);

-- +goose Down
DROP INDEX analyzer.analyses_expired;
DROP INDEX analyzer.analyses_queue;
DROP INDEX analyzer.submissions_owner_request;
DROP TABLE analyzer.snapshot_vulnerabilities;
DROP TABLE analyzer.snapshot_dependencies;
DROP TABLE analyzer.snapshots;
DROP TABLE analyzer.analyses;
DROP TABLE analyzer.submissions;
DROP SCHEMA analyzer;
