# Albums API: how to run it and test it with curl

The same requests are run automatically by `scripts/smoke.sh` (a local copy of the CI smoke test)
and by the end-to-end tests in `internal/app`. Responses below are abbreviated; ids, timestamps and
the `Date` header differ on your machine.

## 1. Run the application

```sh
cp .env.example .env
sed -i '' "s|^JWT_SECRET=.*|JWT_SECRET=$(openssl rand -base64 48)|" .env   # the example value is a placeholder the server refuses (Linux: sed -i)
docker compose up -d --build      # postgres + api on :8080; migrations (schema + demo data) run on start
make bootstrap                    # Lesson 9: gives admin@albums.local the password from ADMIN_PASSWORD in .env
docker compose logs -f api        # every SQL statement is logged (DB_LOG_LEVEL=info)
```

Host run instead of the api container: `docker compose up -d postgres`, `set -a; . ./.env; set +a`,
`go run ./cmd/api`. Stop with `Ctrl+C`; the data stays in the `pgdata` volume.

Automated checks:

```sh
make test                                         # go test ./... -race (TEST_DATABASE_URL from .env)
scripts/smoke.sh http://localhost:8080            # the CI API flow against a running server
scripts/measure_n1.sh http://localhost:8080       # SELECT count and latency of GET /api/v1/albums
```

## 2. Get a token (Lesson 9)

Reads are public. Every write (`POST`, `PUT`, `DELETE`) needs `Authorization: Bearer <token>` of a
`staff` or `admin` account, `GET /me` needs any account and `/users` needs `admin`. The seed accounts
have no password, so create the administrator first (`make bootstrap`, or
`docker compose exec -e ADMIN_PASSWORD=... api /app/bootstrap -email admin@albums.local`), then log in.
Tokens are HS256 JWTs signed with `JWT_SECRET`; they expire after 24 hours and carry the role at login
time, so a changed role takes effect at the next login.

```sh
# the administrator that make bootstrap created (ADMIN_EMAIL / ADMIN_PASSWORD from .env)
TOKEN=$(curl -s -X POST localhost:8080/api/v1/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"admin@albums.local","password":"Albums-Dev-2026"}' | jq -r .token)

# a new account is always a member: it can read and see itself, an admin can promote it
curl -s -X POST localhost:8080/api/v1/auth/register -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"Correct-Horse-Battery-1"}'
curl -s localhost:8080/api/v1/me -H "Authorization: Bearer $TOKEN"
curl -s -X PUT localhost:8080/api/v1/users/4/role -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"role":"staff"}'
curl -s localhost:8080/api/v1/users -H "Authorization: Bearer $TOKEN"
```

```
{"id":4,"email":"alice@example.com","role":"member","created_at":"...","updated_at":"..."}
{"id":1,"email":"admin@albums.local","role":"admin","created_at":"...","updated_at":"..."}
{"id":4,"email":"alice@example.com","role":"staff","created_at":"...","updated_at":"..."}
[{"id":1,"email":"admin@albums.local","role":"admin",...},{"id":2,"email":"staff@albums.local","role":"staff",...},...]
```

No response ever contains the password hash. The examples below assume `TOKEN` holds a staff or
admin token.

## 3. Conventions used below

- Base URL: `localhost:8080/api/v1`. curl adds `http://` itself.
- `-s` hides the progress bar. `-i` prints the status line and headers.
- Query strings with `&` must be quoted, otherwise the shell runs the command in the background.
- Every response except `204` is JSON; append `| python3 -m json.tool` to pretty-print.
- Writes carry `-H "Authorization: Bearer $TOKEN"` (section 2); without it they answer `401`, with a
  member token `403`. The access check runs before any other check (`PUT /albums/abc` without a token is `401`).
- The demo data already holds 3 genres, 6 artists, 10 albums and 24 copies, so new ids start at 4, 7, 11 and 25.

## 4. Happy path, in order

### Health

```sh
curl -s -i localhost:8080/health
```

```
HTTP/1.1 200 OK
Access-Control-Allow-Origin: *
Content-Type: application/json

{"status":"ok"}
```

### Genres and artists

```sh
curl -s -X POST -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/genres -H 'Content-Type: application/json' -d '{"name":"soul"}'
curl -s localhost:8080/api/v1/genres                      # all genres, ordered by id
curl -s localhost:8080/api/v1/genres/1                    # {"id":1,"name":"jazz",...}
curl -s -X POST -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/artists -H 'Content-Type: application/json' \
  -d '{"name":"Aretha Franklin","bio":"Queen of Soul"}'
curl -s -X POST -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/artists -H 'Content-Type: application/json' -d '{"name":"King Curtis"}'
curl -s localhost:8080/api/v1/artists/7                   # {"id":7,"name":"Aretha Franklin","bio":"Queen of Soul",...}
```

```
{"id":4,"name":"soul","created_at":"...","updated_at":"..."}
{"id":7,"name":"Aretha Franklin","bio":"Queen of Soul","created_at":"...","updated_at":"..."}
{"id":8,"name":"King Curtis","bio":null,"created_at":"...","updated_at":"..."}
```

### Create an album that references them (id 11)

```sh
curl -s -i -X POST -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/albums -H 'Content-Type: application/json' \
  -d '{"title":"Live at Fillmore West","label":"Atlantic","genre_id":4,"artist_ids":[7,8],"release_year":1971}'
```

```
HTTP/1.1 201 Created
Content-Type: application/json

{"id":11,"title":"Live at Fillmore West","label":"Atlantic","genre_id":4,
 "genre":{"id":4,"name":"soul",...},
 "artists":[{"id":7,"name":"Aretha Franklin",...},{"id":8,"name":"King Curtis",...}],
 "release_year":1971,"notes":null,"created_at":"...","updated_at":"..."}
```

### Get one, list, filter, paginate

```sh
curl -s localhost:8080/api/v1/albums/11
curl -s 'localhost:8080/api/v1/albums?limit=100'            # all albums with genre and artists
curl -s 'localhost:8080/api/v1/albums?genre_id=4'           # only soul: [{"id":11,...}]
curl -s 'localhost:8080/api/v1/albums?page=2&limit=5'       # albums 6..10
curl -s 'localhost:8080/api/v1/albums?page=99'              # []
```

### Copies and availability

```sh
curl -s -X POST -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/albums/11/copies -H 'Content-Type: application/json' -d '{"inventory_no":"INV-0025"}'
curl -s -X POST -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/albums/11/copies -H 'Content-Type: application/json' -d '{"inventory_no":"INV-0026","status":"lost"}'
curl -s localhost:8080/api/v1/albums/11/copies
curl -s localhost:8080/api/v1/albums/11/availability
```

```
{"id":25,"album_id":11,"inventory_no":"INV-0025","status":"available","created_at":"...","updated_at":"..."}
{"id":26,"album_id":11,"inventory_no":"INV-0026","status":"lost","created_at":"...","updated_at":"..."}
[{"id":25,...},{"id":26,...}]
{"album_id":11,"total_copies":2,"available_copies":1}
```

### Update: PUT is a full replacement, artist_ids replace the artists

```sh
curl -s -X PUT -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/albums/11 -H 'Content-Type: application/json' \
  -d '{"title":"Live at Fillmore West","label":"Atlantic","genre_id":4,"artist_ids":[7]}'
```

```
{"id":11,...,"artists":[{"id":7,...}],"release_year":null,"notes":null,"created_at":"<unchanged>","updated_at":"<newer>"}
```

### Delete: 204, copies and artist links go with the album

```sh
curl -s -i -X DELETE -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/albums/11       # HTTP/1.1 204 No Content
curl -s localhost:8080/api/v1/albums/11                     # {"error":{"code":"not_found","message":"Album not found"}}
curl -s localhost:8080/api/v1/artists/7                     # the artist still exists
```

## 5. Error cases

All error responses have the same shape: `{"error":{"code":"...","message":"..."}}`.

### 409 `conflict`: UNIQUE constraints

```sh
curl -s -X POST -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/genres -H 'Content-Type: application/json' -d '{"name":"jazz"}'
curl -s -X POST -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/albums -H 'Content-Type: application/json' \
  -d '{"title":"Kind of Blue","label":"Columbia","genre_id":1,"artist_ids":[1]}'
curl -s -X POST -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/albums/1/copies -H 'Content-Type: application/json' -d '{"inventory_no":"INV-0001"}'
```

```
{"error":{"code":"conflict","message":"Genre already exists"}}
{"error":{"code":"conflict","message":"Album already exists"}}
{"error":{"code":"conflict","message":"Copy already exists"}}
```

### 422 `invalid_reference`: unknown genre or artist (nothing is created)

```sh
curl -s -X POST -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/albums -H 'Content-Type: application/json' \
  -d '{"title":"Bad","label":"L","genre_id":1,"artist_ids":[999999]}'
curl -s localhost:8080/api/v1/artists/999999                # still 404: no phantom artist
```

```
{"error":{"code":"invalid_reference","message":"referenced row does not exist: artists [999999]"}}
```

### 422 `validation_error`

```sh
curl -s -X POST -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/albums -H 'Content-Type: application/json' -d '{"label":"L"}'
curl -s -X POST -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/albums -H 'Content-Type: application/json' \
  -d '{"title":"T","label":"L","genre_id":1,"artist_ids":[]}'
curl -s -X POST -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/albums -H 'Content-Type: application/json' \
  -d '{"title":"T","label":"L","genre_id":"1","artist_ids":[1]}'
curl -s -X POST -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/albums/1/copies -H 'Content-Type: application/json' \
  -d '{"inventory_no":"X","status":"broken"}'
curl -s -X POST -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/albums -H 'Content-Type: application/json' -d '{not json'
```

```
{"error":{"code":"validation_error","message":"validation error: required fields missing or empty: title; genre_id must be a positive integer; artist_ids must contain at least one artist id"}}
{"error":{"code":"validation_error","message":"validation error: artist_ids must contain at least one artist id"}}
{"error":{"code":"validation_error","message":"field \"genre_id\" has an invalid type"}}
{"error":{"code":"validation_error","message":"validation error: status must be one of: available, borrowed, lost"}}
{"error":{"code":"validation_error","message":"request body must be a valid JSON object"}}
```

### 400 and 404

```sh
curl -s localhost:8080/api/v1/albums/abc                    # {"error":{"code":"invalid_id",...}}
curl -s 'localhost:8080/api/v1/albums?page=0'               # {"error":{"code":"invalid_pagination",...}}
curl -s 'localhost:8080/api/v1/albums?genre_id=abc'         # {"error":{"code":"validation_error",...}}
curl -s localhost:8080/api/v1/albums/999999                 # {"error":{"code":"not_found","message":"Album not found"}}
curl -s localhost:8080/api/v1/albums/999999/availability    # {"error":{"code":"not_found","message":"Album not found"}}
curl -s -i -X PATCH localhost:8080/api/v1/albums/1          # 404 route not found (JSON)
```

### PUT check order: id (400), then body (422), then existence (404), then references (422)

```sh
curl -s -X PUT -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/albums/abc -H 'Content-Type: application/json' -d '{not json'
curl -s -X PUT -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/albums/999 -H 'Content-Type: application/json' -d '{not json'
curl -s -X PUT -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/albums/999 -H 'Content-Type: application/json' \
  -d '{"title":"X","label":"L","genre_id":1,"artist_ids":[1]}'
curl -s -X PUT -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/albums/1 -H 'Content-Type: application/json' \
  -d '{"title":"X","label":"L","genre_id":1,"artist_ids":[999999]}'   # 422 and album 1 is unchanged
```

### 401 `unauthenticated` and 403 `forbidden`

```sh
MEMBER_TOKEN=$(curl -s -X POST localhost:8080/api/v1/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"Correct-Horse-Battery-1"}' | jq -r .token)   # a member from section 2
curl -s -i -X POST localhost:8080/api/v1/genres -H 'Content-Type: application/json' -d '{"name":"x"}'
curl -s -i -X POST localhost:8080/api/v1/genres -H "Authorization: Bearer $MEMBER_TOKEN" -H 'Content-Type: application/json' -d '{"name":"x"}'
curl -s -i localhost:8080/api/v1/me -H 'Authorization: Bearer not.a.jwt'
curl -s -i -X POST localhost:8080/api/v1/auth/login -H 'Content-Type: application/json' -d '{"email":"nobody@albums.local","password":"wrong"}'
```

```
HTTP/1.1 401 Unauthorized   {"error":{"code":"unauthenticated","message":"a bearer token is required"}}
HTTP/1.1 403 Forbidden      {"error":{"code":"forbidden","message":"this action requires role staff or admin"}}
HTTP/1.1 401 Unauthorized   {"error":{"code":"unauthenticated","message":"invalid or expired token"}}
HTTP/1.1 401 Unauthorized   {"error":{"code":"unauthenticated","message":"invalid credentials"}}
```

An unknown e-mail and a wrong password get the same `401` body, so the endpoint cannot enumerate
accounts; an expired token, another secret, a tampered payload or `alg: none` all get `invalid or
expired token`.

## 6. CORS

```sh
curl -s -i -X OPTIONS localhost:8080/api/v1/albums -H 'Origin: http://example.com' -H 'Access-Control-Request-Method: POST'
```

```
HTTP/1.1 204 No Content
Access-Control-Allow-Headers: Content-Type, Authorization
Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
Access-Control-Allow-Origin: *
```

## 7. N+1 in the logs

```sh
scripts/measure_n1.sh http://localhost:8080                 # SELECT count + ab latency, reads docker compose logs
docker compose logs --no-log-prefix api | grep -c SELECT     # raw count since the container started
```

Before the fix, with the relations loaded in a loop, one `GET /api/v1/albums?limit=100` over the 10 demo albums
issued 21 SELECTs (1 + 2 per album); with `Preload("Artists").Preload("Genre")` it issues 4, however many
albums there are. Raw outputs of both measurements and of the smoke replay: `docs/n1/`.
