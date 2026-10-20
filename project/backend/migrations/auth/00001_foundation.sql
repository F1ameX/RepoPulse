-- +goose Up
CREATE SCHEMA auth;

CREATE TABLE auth.principals (
    id uuid PRIMARY KEY,
    kind text NOT NULL,
    login text UNIQUE,
    password_hash text,
    created_at timestamptz NOT NULL
);

CREATE TABLE auth.sessions (
    id uuid PRIMARY KEY,
    principal_id uuid NOT NULL REFERENCES auth.principals(id),
    kind text NOT NULL,
    token_hash text NOT NULL UNIQUE,
    issued_at timestamptz NOT NULL,
    expires_at timestamptz NOT NULL,
    revoked_at timestamptz
);

CREATE INDEX sessions_principal ON auth.sessions(principal_id);
CREATE UNIQUE INDEX sessions_one_guest ON auth.sessions(principal_id) WHERE kind = 'guest';

-- +goose Down
DROP INDEX auth.sessions_one_guest;
DROP INDEX auth.sessions_principal;
DROP TABLE auth.sessions;
DROP TABLE auth.principals;
DROP SCHEMA auth;
