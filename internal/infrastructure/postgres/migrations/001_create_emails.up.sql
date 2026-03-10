BEGIN;

CREATE TYPE application_status AS ENUM ('applied', 'rejected', 'advanced');

CREATE TABLE application (
    id SERIAL PRIMARY KEY,
    company TEXT NOT NULL UNIQUE,
    applied_at TIMESTAMP WITH TIME ZONE,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    rejected_at TIMESTAMP WITH TIME ZONE,
    status application_status NOT NULL
);

CREATE INDEX idx_application_status ON application(status);

COMMIT;