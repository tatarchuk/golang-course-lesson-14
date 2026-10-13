-- Lesson 9: bcrypt password hashes for the accounts. The column is NOT NULL with an empty
-- default: existing rows (the seed accounts) get '' and cannot log in until cmd/bootstrap
-- sets a password. No hash is ever written by a migration.
ALTER TABLE users ADD COLUMN IF NOT EXISTS password_hash TEXT NOT NULL DEFAULT '';
