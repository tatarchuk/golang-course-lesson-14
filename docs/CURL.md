# Albums API: how to run it and test it with curl

Every command below was executed against the current code and the responses shown are real,
except that timestamps and the `Date` header will differ on your machine.

## 1. Run the application

Requirements: Go 1.22 or newer (`go version`), run from the repository root.

```sh
go run ./cmd/api                 # listens on :8080, logs "listening on :8080"
PORT=9090 go run ./cmd/api       # another port
go build -o api ./cmd/api && ./api   # or build a binary first (the file `api` is git-ignored)
```

Stop it with `Ctrl+C`. The store is in memory: restarting the server deletes all albums and
ids start from 1 again. Every request is logged on the server side as
`METHOD /path -> status (duration)`.

Automated tests, the same command CI runs:

```sh
go test -race -count=1 ./...                      # everything
go test ./internal/app -run TestStage4 -v         # one stage
```

## 2. Conventions used below

- Base URL: `localhost:8080/api/v1/albums`. curl adds `http://` itself.
- `-s` hides the progress bar. `-i` prints the status line and headers; add it to any command
  when you want to see the status code.
- Query strings with `&` must be quoted, otherwise the shell treats `&` as "run in background".
- Every response except `204` is JSON. To pretty-print one, append `| python3 -m json.tool`.
- The sequence in section 3 assumes a freshly started server, so ids are predictable.

## 3. Happy path, in order

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

### Create an album with every field (id 1)

```sh
curl -s -i -X POST localhost:8080/api/v1/albums -H 'Content-Type: application/json' \
  -d '{"title":"Kind of Blue","artist":"Miles Davis","label":"Columbia","genre":"jazz","release_year":1959,"notes":"Recorded in 1959"}'
```

```
HTTP/1.1 201 Created
Content-Type: application/json

{"id":1,"title":"Kind of Blue","artist":"Miles Davis","label":"Columbia","genre":"jazz","release_year":1959,"notes":"Recorded in 1959","created_at":"2026-10-06T10:30:01.236249Z","updated_at":"2026-10-06T10:30:01.236249Z"}
```

### Create with optional fields omitted: they come back as `null` (id 2)

```sh
curl -s -X POST localhost:8080/api/v1/albums -H 'Content-Type: application/json' \
  -d '{"title":"Nevermind","artist":"Nirvana","label":"DGC","genre":"rock"}'
```

```
{"id":2,"title":"Nevermind","artist":"Nirvana","label":"DGC","genre":"rock","release_year":null,"notes":null,"created_at":"...","updated_at":"..."}
```

### One more for the list examples (id 3)

```sh
curl -s -X POST localhost:8080/api/v1/albums -H 'Content-Type: application/json' \
  -d '{"title":"A Love Supreme","artist":"John Coltrane","label":"Impulse!","genre":"jazz","release_year":1965}'
```

### Get one by id

```sh
curl -s localhost:8080/api/v1/albums/1
```

```
{"id":1,"title":"Kind of Blue", ... ,"created_at":"...","updated_at":"..."}
```

### List: everything, sorted by id

```sh
curl -s localhost:8080/api/v1/albums
```

```
[{"id":1,...},{"id":2,...},{"id":3,...}]
```

### List: pagination (`page` defaults to 1, `limit` to 10, `limit` above 100 becomes 100)

```sh
curl -s 'localhost:8080/api/v1/albums?page=1&limit=2'    # [{"id":1,...},{"id":2,...}]
curl -s 'localhost:8080/api/v1/albums?page=2&limit=2'    # [{"id":3,...}]
curl -s 'localhost:8080/api/v1/albums?page=3&limit=2'    # []
```

### List: filter by `genre` (exact, case-sensitive), alone and combined with pagination

```sh
curl -s 'localhost:8080/api/v1/albums?genre=jazz'                  # [{"id":1,...},{"id":3,...}]
curl -s 'localhost:8080/api/v1/albums?genre=jazz&limit=1&page=2'   # [{"id":3,...}]
curl -s 'localhost:8080/api/v1/albums?genre='                      # empty value = no filter: all three
curl -s 'localhost:8080/api/v1/albums?genre=Jazz'                  # [] : "Jazz" is not "jazz"
```

### List: very large values do not break anything

```sh
curl -s 'localhost:8080/api/v1/albums?page=9223372036854775807'    # []  (page far past the end)
curl -s 'localhost:8080/api/v1/albums?limit=99999999999999999999'  # all albums (limit clamped to 100)
```

### Update: PUT is a full replacement

```sh
curl -s -X PUT localhost:8080/api/v1/albums/2 -H 'Content-Type: application/json' \
  -d '{"title":"Nevermind","artist":"Nirvana","label":"DGC","genre":"grunge","release_year":1991}'
```

```
{"id":2,"title":"Nevermind","artist":"Nirvana","label":"DGC","genre":"grunge","release_year":1991,"notes":null,"created_at":"<unchanged>","updated_at":"<newer>"}
```

Omit the optional fields and they become `null` again, because nothing is merged:

```sh
curl -s -X PUT localhost:8080/api/v1/albums/2 -H 'Content-Type: application/json' \
  -d '{"title":"Nevermind","artist":"Nirvana","label":"DGC","genre":"grunge"}'
```

```
{"id":2, ... ,"release_year":null,"notes":null, ... }
```

### Delete: 204 with no body, then 404 on repeat

```sh
curl -s -i -X DELETE localhost:8080/api/v1/albums/3
```

```
HTTP/1.1 204 No Content
Access-Control-Allow-Origin: *
```

```sh
curl -s -i -X DELETE localhost:8080/api/v1/albums/3
```

```
HTTP/1.1 404 Not Found
Content-Type: application/json

{"error":{"code":"not_found","message":"Album not found"}}
```

### Ids are never reused: the next album gets id 4, not 3

```sh
curl -s -X POST localhost:8080/api/v1/albums -H 'Content-Type: application/json' \
  -d '{"title":"In Rainbows","artist":"Radiohead","label":"XL","genre":"rock"}'
```

```
{"id":4,"title":"In Rainbows", ... }
```

## 4. Error cases

All error responses have the same shape: `{"error":{"code":"...","message":"..."}}`.

### Invalid id: 400 `invalid_id` (must be an integer from 0 to 4294967295)

```sh
curl -s -i localhost:8080/api/v1/albums/abc
curl -s localhost:8080/api/v1/albums/-1
curl -s localhost:8080/api/v1/albums/1.5
curl -s localhost:8080/api/v1/albums/4294967296
```

```
HTTP/1.1 400 Bad Request
Content-Type: application/json

{"error":{"code":"invalid_id","message":"id must be an integer between 0 and 4294967295"}}
```

### Valid id, no such album: 404 `not_found` (0 is a valid id that nothing has)

```sh
curl -s localhost:8080/api/v1/albums/0
curl -s localhost:8080/api/v1/albums/999
```

```
{"error":{"code":"not_found","message":"Album not found"}}
```

### Validation: 422 `validation_error`

Missing, empty, whitespace-only or `null` required field:

```sh
curl -s -i -X POST localhost:8080/api/v1/albums -H 'Content-Type: application/json' \
  -d '{"title":"X","artist":"Y","label":"Z"}'
curl -s -X POST localhost:8080/api/v1/albums -H 'Content-Type: application/json' \
  -d '{"title":"  ","artist":"Y","label":"Z","genre":"g"}'
curl -s -X POST localhost:8080/api/v1/albums -H 'Content-Type: application/json' \
  -d '{"title":null,"artist":"Y","label":"Z","genre":"g"}'
```

```
HTTP/1.1 422 Unprocessable Entity
Content-Type: application/json

{"error":{"code":"validation_error","message":"validation error: required fields missing or empty: genre"}}
```

Wrong type of an optional field (string instead of integer, fraction, number instead of text):

```sh
curl -s -X POST localhost:8080/api/v1/albums -H 'Content-Type: application/json' \
  -d '{"title":"X","artist":"Y","label":"Z","genre":"g","release_year":"1999"}'
curl -s -X POST localhost:8080/api/v1/albums -H 'Content-Type: application/json' \
  -d '{"title":"X","artist":"Y","label":"Z","genre":"g","release_year":1999.5}'
curl -s -X POST localhost:8080/api/v1/albums -H 'Content-Type: application/json' \
  -d '{"title":"X","artist":"Y","label":"Z","genre":"g","notes":42}'
```

```
{"error":{"code":"validation_error","message":"field \"release_year\" has an invalid type"}}
{"error":{"code":"validation_error","message":"field \"release_year\" has an invalid type"}}
{"error":{"code":"validation_error","message":"field \"notes\" has an invalid type"}}
```

Body that is not a JSON object (broken JSON, empty body, array):

```sh
curl -s -X POST localhost:8080/api/v1/albums -H 'Content-Type: application/json' -d '{not json'
curl -s -X POST localhost:8080/api/v1/albums -H 'Content-Type: application/json'
curl -s -X POST localhost:8080/api/v1/albums -H 'Content-Type: application/json' -d '[1,2,3]'
```

```
{"error":{"code":"validation_error","message":"request body must be a valid JSON object"}}
```

### PUT check order: id first (400), then body (422), then existence (404)

```sh
curl -s -i -X PUT localhost:8080/api/v1/albums/abc -H 'Content-Type: application/json' -d '{not json'
# 400 invalid_id: the id loses before the body is even read
curl -s -X PUT localhost:8080/api/v1/albums/999 -H 'Content-Type: application/json' -d '{not json'
# 422 validation_error: the body loses before existence is checked
curl -s -X PUT localhost:8080/api/v1/albums/999 -H 'Content-Type: application/json' \
  -d '{"title":"X","artist":"Y","label":"Z","genre":"g"}'
# 404 not_found: valid request, no such album
```

### Invalid pagination: 400 `invalid_pagination`

```sh
curl -s -i 'localhost:8080/api/v1/albums?page=0'
curl -s 'localhost:8080/api/v1/albums?limit=abc'
curl -s 'localhost:8080/api/v1/albums?page=-1'
```

```
HTTP/1.1 400 Bad Request
Content-Type: application/json

{"error":{"code":"invalid_pagination","message":"page and limit must be integers >= 1"}}
```

`>` is how Go's JSON encoder escapes `>`; any JSON client decodes it back to `>=`.
If you prefer the literal character, call `SetEscapeHTML(false)` on the encoder in `WriteJSON`.

### Unknown route and unsupported method: 404 as JSON

```sh
curl -s -i localhost:8080/no-such-route
curl -s -i -X PATCH localhost:8080/api/v1/albums/1
```

```
HTTP/1.1 404 Not Found
Content-Type: application/json

{"error":{"code":"not_found","message":"route not found"}}
```

## 5. CORS

Preflight: 204, no body, with the allowed methods:

```sh
curl -s -i -X OPTIONS localhost:8080/api/v1/albums \
  -H 'Origin: http://example.com' -H 'Access-Control-Request-Method: POST'
```

```
HTTP/1.1 204 No Content
Access-Control-Allow-Headers: Content-Type
Access-Control-Allow-Methods: GET, POST, PUT, DELETE, OPTIONS
Access-Control-Allow-Origin: *
```

Every regular response carries `Access-Control-Allow-Origin` too:

```sh
curl -s -i localhost:8080/health -H 'Origin: http://example.com'
```

```
HTTP/1.1 200 OK
Access-Control-Allow-Origin: *
Content-Type: application/json

{"status":"ok"}
```

## 6. Concurrency smoke test

Fifty parallel creates must produce fifty distinct ids. With the server running:

```sh
for i in $(seq 1 50); do
  curl -s -X POST localhost:8080/api/v1/albums -H 'Content-Type: application/json' \
    -d "{\"title\":\"T$i\",\"artist\":\"A\",\"label\":\"L\",\"genre\":\"g\"}" &
done; wait
curl -s 'localhost:8080/api/v1/albums?limit=100' | python3 -c 'import json,sys; ids=[a["id"] for a in json.load(sys.stdin)]; print(len(ids), "albums,", len(set(ids)), "distinct ids")'
```

The authoritative check for data races is still `go test -race ./...`, which runs the same
scenario in-process with the race detector enabled.
