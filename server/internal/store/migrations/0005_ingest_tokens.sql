-- A project can take logs pushed over HTTP (the library HttpWriter, the JS client, the agent).
-- Only the SHA-256 of its token is kept: the token is shown once, when it is made.
ALTER TABLE projects ADD COLUMN ingest_token_hash TEXT;
CREATE UNIQUE INDEX projects_ingest_token ON projects (ingest_token_hash) WHERE ingest_token_hash IS NOT NULL;
