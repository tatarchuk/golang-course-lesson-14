-- Reverts 000003: drops the password column. Accounts stay, their passwords are lost.
ALTER TABLE users DROP COLUMN IF EXISTS password_hash;
