-- Demo data for local runs, the N+1 measurements and the CI smoke test.
-- Explicit ids keep the link rows readable; the sequences are advanced at the end so
-- rows inserted later through the API do not collide with the seeded ids.

INSERT INTO genres (id, name) VALUES
    (1, 'jazz'),
    (2, 'rock'),
    (3, 'electronic');

INSERT INTO artists (id, name, bio) VALUES
    (1, 'Miles Davis',   'Trumpeter and bandleader, one of the central figures of jazz.'),
    (2, 'John Coltrane', 'Saxophonist and composer.'),
    (3, 'Nirvana',       'Rock band from Aberdeen, Washington.'),
    (4, 'Radiohead',     'Rock band from Abingdon, Oxfordshire.'),
    (5, 'Daft Punk',     NULL),
    (6, 'Bill Evans',    'Jazz pianist.');

INSERT INTO albums (id, title, label, genre_id, release_year, notes) VALUES
    (1,  'Kind of Blue',           'Columbia',   1, 1959, 'Recorded in 1959 with Coltrane and Evans.'),
    (2,  'Sketches of Spain',      'Columbia',   1, 1960, NULL),
    (3,  'A Love Supreme',         'Impulse!',   1, 1965, 'Four-part suite.'),
    (4,  'Nevermind',              'DGC',        2, 1991, NULL),
    (5,  'In Utero',               'DGC',        2, 1993, NULL),
    (6,  'OK Computer',            'Parlophone', 2, 1997, 'Recorded at St Catherine''s Court.'),
    (7,  'Kid A',                  'Parlophone', 2, 2000, NULL),
    (8,  'Discovery',              'Virgin',     3, 2001, NULL),
    (9,  'Random Access Memories', 'Columbia',   3, 2013, 'Grammy Album of the Year 2014.'),
    (10, 'Waltz for Debby',        'Riverside',  1, NULL, 'Live at the Village Vanguard.');

-- Album 1 has three artists; artist 1 appears on albums 1 and 2; every album has at least one.
INSERT INTO album_artists (album_id, artist_id) VALUES
    (1, 1), (1, 2), (1, 6),
    (2, 1),
    (3, 2),
    (4, 3),
    (5, 3),
    (6, 4),
    (7, 4),
    (8, 5),
    (9, 5),
    (10, 6);

INSERT INTO copies (id, album_id, inventory_no, status) VALUES
    (1,  1,  'INV-0001', 'available'),
    (2,  1,  'INV-0002', 'available'),
    (3,  1,  'INV-0003', 'borrowed'),
    (4,  1,  'INV-0004', 'lost'),
    (5,  2,  'INV-0005', 'available'),
    (6,  2,  'INV-0006', 'borrowed'),
    (7,  3,  'INV-0007', 'available'),
    (8,  3,  'INV-0008', 'available'),
    (9,  4,  'INV-0009', 'available'),
    (10, 4,  'INV-0010', 'borrowed'),
    (11, 4,  'INV-0011', 'available'),
    (12, 5,  'INV-0012', 'available'),
    (13, 5,  'INV-0013', 'lost'),
    (14, 6,  'INV-0014', 'available'),
    (15, 6,  'INV-0015', 'available'),
    (16, 6,  'INV-0016', 'borrowed'),
    (17, 7,  'INV-0017', 'available'),
    (18, 7,  'INV-0018', 'available'),
    (19, 8,  'INV-0019', 'available'),
    (20, 8,  'INV-0020', 'borrowed'),
    (21, 9,  'INV-0021', 'available'),
    (22, 9,  'INV-0022', 'available'),
    (23, 10, 'INV-0023', 'available'),
    (24, 10, 'INV-0024', 'lost');

INSERT INTO users (id, email, role) VALUES
    (1, 'admin@albums.local',  'admin'),
    (2, 'staff@albums.local',  'staff'),
    (3, 'member@albums.local', 'member');

-- Advance the sequences past the explicit ids.
SELECT setval(pg_get_serial_sequence('genres',  'id'), (SELECT max(id) FROM genres));
SELECT setval(pg_get_serial_sequence('artists', 'id'), (SELECT max(id) FROM artists));
SELECT setval(pg_get_serial_sequence('albums',  'id'), (SELECT max(id) FROM albums));
SELECT setval(pg_get_serial_sequence('copies',  'id'), (SELECT max(id) FROM copies));
SELECT setval(pg_get_serial_sequence('users',   'id'), (SELECT max(id) FROM users));
