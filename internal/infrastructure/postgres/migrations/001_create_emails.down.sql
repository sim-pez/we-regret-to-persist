BEGIN;

DROP INDEX IF EXISTS idx_application_status;

DROP TABLE IF EXISTS application;

DROP TYPE IF EXISTS application_status;

COMMIT;
