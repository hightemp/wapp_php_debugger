-- seq of the last event read from a v2 file: the next chunk continues from it.
ALTER TABLE log_files ADD COLUMN last_seq INTEGER NOT NULL DEFAULT 0 CHECK (last_seq >= 0);
