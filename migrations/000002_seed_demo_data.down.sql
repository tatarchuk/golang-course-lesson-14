-- Removes the demo data in reverse dependency order and resets the sequences.
-- Only the seeded rows are touched; rows created later through the API stay
-- (an API-created album that still references a seeded genre or artist makes
-- this rollback fail on the RESTRICT foreign keys, which is intended).

DELETE FROM users WHERE email IN ('admin@albums.local', 'staff@albums.local', 'member@albums.local');
DELETE FROM copies WHERE id BETWEEN 1 AND 24;
DELETE FROM album_artists WHERE album_id BETWEEN 1 AND 10;
DELETE FROM albums WHERE id BETWEEN 1 AND 10;
DELETE FROM artists WHERE id BETWEEN 1 AND 6;
DELETE FROM genres WHERE id BETWEEN 1 AND 3;

-- setval(seq, value, is_called): with the table empty the next id is 1 again,
-- otherwise it continues after the highest remaining id.
SELECT setval(pg_get_serial_sequence('genres',  'id'), COALESCE((SELECT max(id) FROM genres),  1), (SELECT max(id) FROM genres)  IS NOT NULL);
SELECT setval(pg_get_serial_sequence('artists', 'id'), COALESCE((SELECT max(id) FROM artists), 1), (SELECT max(id) FROM artists) IS NOT NULL);
SELECT setval(pg_get_serial_sequence('albums',  'id'), COALESCE((SELECT max(id) FROM albums),  1), (SELECT max(id) FROM albums)  IS NOT NULL);
SELECT setval(pg_get_serial_sequence('copies',  'id'), COALESCE((SELECT max(id) FROM copies),  1), (SELECT max(id) FROM copies)  IS NOT NULL);
SELECT setval(pg_get_serial_sequence('users',   'id'), COALESCE((SELECT max(id) FROM users),   1), (SELECT max(id) FROM users)   IS NOT NULL);
