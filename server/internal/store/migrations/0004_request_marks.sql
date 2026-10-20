-- Marks the user puts on a request: pinned ones survive retention, a note says why it matters.
ALTER TABLE requests ADD COLUMN pinned INTEGER NOT NULL DEFAULT 0 CHECK (pinned IN (0, 1));
ALTER TABLE requests ADD COLUMN note TEXT;
CREATE INDEX requests_pinned ON requests (pinned) WHERE pinned = 1;
