Act as a senior Go and PostgreSQL database engineer.
Your task is to generate a highly robust and production-ready up/down SQL migration pair (using golang-migrate format) based on the following Go struct(s):

---
[INSERT_YOUR_GO_STRUCTS_HERE]
---

Please generate two separate migration files:
1. `{{VERSION}}_{{NAME}}.up.sql`
2. `{{VERSION}}_{{NAME}}.down.sql`

Strict Database Design & Constraint Requirements:
1. Primary Keys: Use `BIGSERIAL` (autoincrementing 64-bit integer) for IDs.
2. Nullability: Every column must be `NOT NULL` by default. If a field in the Go struct is a pointer (e.g., *string, *int) or is explicitly optional, map it to a nullable database column.
3. Timestamps: Include `created_at TIMESTAMP NOT NULL DEFAULT now()` and `updated_at TIMESTAMP NOT NULL DEFAULT now()` on every created table.
4. Foreign Keys (FK):
    - Every foreign key must have an explicit, named constraint (e.g., `CONSTRAINT fk_albums_genre FOREIGN KEY...`).
    - Use `ON DELETE RESTRICT` as the default safety behavior for FKs to prevent accidental data loss, unless cascade deletion is explicitly reasonable (state why).
5. Performance Indexing:
    - Explicitly write SQL to create individual indexes on all Foreign Key columns.
    - Systematic naming: `idx_{table}_{column}` (e.g., `idx_albums_genre_id`).
6. Data Integrity: Add `CHECK` constraints where fields have a restricted set of valid string options (e.g., statuses, priority ranks).
7. Idempotency: Use `CREATE TABLE IF NOT EXISTS` for table generation, and `DROP TABLE IF EXISTS` for rollbacks.
8. Down Migration Rollback: The `down.sql` must completely and cleanly revert the `up.sql`. Ensure you drop foreign keys, indexes, and tables in the EXACT reverse order of their creation to avoid dependency locks.

Output Format:
Provide only two clean, copy-pasteable SQL code blocks (one for UP, one for DOWN) without conversational fluff.
