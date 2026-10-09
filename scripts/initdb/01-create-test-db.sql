-- Runs once, when the postgres volume is first initialised (docker-entrypoint-initdb.d).
-- albums_test: database used by `go test ./...` (TEST_DATABASE_URL).
-- albums_migr: scratch database for checking migrations the way CI does (up, probes, down -all, up).
CREATE DATABASE albums_test;
CREATE DATABASE albums_migr;
