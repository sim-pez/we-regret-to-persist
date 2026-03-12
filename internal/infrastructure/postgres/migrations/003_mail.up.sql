BEGIN;

CREATE TYPE mail_classification AS ENUM ('rejection', 'other');

CREATE TABLE history (
    id SERIAL PRIMARY KEY,
    classification mail_classification NOT NULL,
    received_at TIMESTAMP WITH TIME ZONE NOT NULL
);

COMMIT;
