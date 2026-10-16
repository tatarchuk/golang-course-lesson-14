#!/usr/bin/env bash
# Replays the API and web UI smoke test of .github/workflows/theme8-check.yml against a
# running server, so the CI result can be reproduced locally before pushing.
#
# Usage: scripts/smoke.sh [BASE_URL]            (default http://localhost:8080)
#   SQL_LOG_CMD  command that prints the server log; its SELECT lines are counted for
#                the N+1 check. Default: docker compose logs --no-log-prefix api
#                For a host run: SQL_LOG_CMD="cat app.log" scripts/smoke.sh
#
# Names carry a timestamp so the script can run repeatedly against the same database.
set -uo pipefail

BASE_URL=${1:-http://localhost:8080}
BASE="$BASE_URL/api/v1"
SQL_LOG_CMD=${SQL_LOG_CMD:-"docker compose logs --no-log-prefix api"}
TAG=$(date +%s)
RESP=$(mktemp)
JARS=$(mktemp -d)
trap 'rm -rf "$RESP" "$RESP.h" "$JARS"' EXIT
FAILED=0

pass() { echo "PASS: $1"; }
fail() { FAILED=1; echo "FAIL: $1${2:+ -- $2}"; }

# Lesson 9: the bootstrapped administrator (run `make bootstrap` first) and the token
# api() attaches to every request while AUTH_TOKEN is non-empty.
ADMIN_EMAIL=${ADMIN_EMAIL:-admin@albums.local}
ADMIN_PASSWORD=${ADMIN_PASSWORD:-Albums-Dev-2026}
AUTH_TOKEN=""

# api METHOD PATH [BODY] -> prints the status code, body goes to $RESP
api() {
  local m="$1" p="$2" b="${3:-}"
  local -a args=(-s -o "$RESP" -w '%{http_code}' -X "$m" "$BASE$p")
  [ -n "$AUTH_TOKEN" ] && args+=(-H "Authorization: Bearer $AUTH_TOKEN")
  [ -n "$b" ] && args+=(-H 'Content-Type: application/json' -d "$b")
  curl "${args[@]}"
}
# http NAME EXPECTED_STATUS METHOD PATH [BODY]
http() {
  local name="$1" want="$2"; shift 2
  local got
  got=$(api "$@")
  if [ "$got" = "$want" ]; then pass "$name"; else fail "$name" "expected HTTP $want, got $got: $(head -c 300 "$RESP")"; fi
}
# jp NAME PYTHON_EXPR EXPECTED   (d is the decoded JSON of the last response)
jp() {
  local name="$1" expr="$2" want="$3" got
  got=$(python3 -c "import json,sys; d=json.load(open(sys.argv[1])); print($expr)" "$RESP" 2>/dev/null || echo "invalid json")
  if [ "$got" = "$want" ]; then pass "$name"; else fail "$name" "expected '$want', got '$got'"; fi
}
field() { python3 -c "import json,sys; print(json.load(open(sys.argv[1]))$2)" "$RESP"; }

echo "== smoke test against $BASE_URL (tag $TAG)"

# health, swagger, CORS
[ "$(curl -s "$BASE_URL/health" | python3 -c 'import json,sys; print(json.load(sys.stdin)["status"])')" = "ok" ] \
  && pass "/health returns status ok" || fail "/health returns status ok"
[ "$(curl -s -o /dev/null -w '%{http_code}' "$BASE_URL/swagger/index.html")" = "200" ] \
  && pass "Swagger UI is served" || fail "Swagger UI is served"
status=$(curl -s -D "$RESP.h" -o /dev/null -w '%{http_code}' -X OPTIONS "$BASE/albums" -H 'Origin: http://example.com' -H 'Access-Control-Request-Method: POST')
{ [ "$status" = "204" ] || [ "$status" = "200" ]; } && grep -qi '^access-control-allow-origin:' "$RESP.h" \
  && pass "CORS preflight is handled" || fail "CORS preflight is handled"
rm -f "$RESP.h"

# --- Lesson 9: authentication and roles ---------------------------------------------
http "Anonymous POST /genres -> 401" 401 POST /genres '{"name":"Anon"}'
jp "Error code is unauthenticated" "d['error']['code']" unauthenticated
MEMBER="smoke-$TAG@albums.local"
http "POST /auth/register creates a member" 201 POST /auth/register "{\"email\":\"$MEMBER\",\"password\":\"$ADMIN_PASSWORD\"}"
jp "Registered role is member" "d['role']" member
jp "No password material in the response" "any(s in open(sys.argv[1]).read().lower() for s in ('password','hash','\$2a\$'))" False
MEMBER_ID=$(field value "['id']")
http "Register the same e-mail again -> 422" 422 POST /auth/register "{\"email\":\"$MEMBER\",\"password\":\"$ADMIN_PASSWORD\"}"
http "Login with a wrong password -> 401" 401 POST /auth/login "{\"email\":\"$MEMBER\",\"password\":\"wrong-password\"}"
http "Login with an unknown e-mail -> 401" 401 POST /auth/login "{\"email\":\"nobody-$TAG@albums.local\",\"password\":\"wrong-password\"}"
http "Login as the new member" 200 POST /auth/login "{\"email\":\"$MEMBER\",\"password\":\"$ADMIN_PASSWORD\"}"
AUTH_TOKEN=$(field value "['token']")
http "GET /me as member" 200 GET /me
jp "GET /me answers with the token's account" "d['email']" "$MEMBER"
http "Member POST /genres -> 403" 403 POST /genres '{"name":"Member"}'
jp "Error code is forbidden" "d['error']['code']" forbidden
http "Member GET /users -> 403" 403 GET /users
AUTH_TOKEN=""
ADMIN_LOGIN="{\"email\":\"$ADMIN_EMAIL\",\"password\":\"$ADMIN_PASSWORD\"}"
status=$(api POST /auth/login "$ADMIN_LOGIN")
if [ "$status" != "200" ]; then
  fail "Login as $ADMIN_EMAIL" "run 'make bootstrap' (or: docker compose exec -e ADMIN_PASSWORD=... api /app/bootstrap) first"
  echo "== done: some checks FAILED"; exit 1
fi
pass "Login as the bootstrapped admin"
AUTH_TOKEN=$(field value "['token']")
http "Admin GET /users" 200 GET /users
http "Admin promotes the member to staff" 200 PUT "/users/$MEMBER_ID/role" '{"role":"staff"}'
jp "Role is now staff" "d['role']" staff
http "PUT /users/{id}/role with a bad role -> 422" 422 PUT "/users/$MEMBER_ID/role" '{"role":"boss"}'
# The catalogue checks below run with the admin token (admin passes the staff gate).

# genres and artists
http "POST /genres creates a genre" 201 POST /genres "{\"name\":\"Smoke Genre $TAG\"}"
GEN=$(field "" '["id"]')
http "Duplicate genre name -> 409" 409 POST /genres "{\"name\":\"Smoke Genre $TAG\"}"
jp "Error envelope: error.code is conflict" 'd["error"]["code"]' conflict
http "POST /genres with an empty name -> 422" 422 POST /genres '{}'
http "GET /genres/{id}" 200 GET "/genres/$GEN"
http "GET /genres lists genres" 200 GET /genres
http "POST /artists creates an artist" 201 POST /artists "{\"name\":\"Smoke Artist $TAG\"}"
ART1=$(field "" '["id"]')
http "POST /artists creates a second artist" 201 POST /artists "{\"name\":\"Smoke Artist Two $TAG\"}"
ART2=$(field "" '["id"]')
http "GET /artists/{id}" 200 GET "/artists/$ART1"
http "GET /artists lists artists" 200 GET /artists
http "GET /artists/{unknown} -> 404" 404 GET /artists/999999

# albums
ALBUMBODY="{\"title\":\"Smoke Album $TAG\",\"label\":\"Smoke Label\",\"genre_id\":$GEN,\"artist_ids\":[$ART1]}"
http "POST /albums creates an album with relations" 201 POST /albums "$ALBUMBODY"
ALBUM=$(field "" '["id"]')
jp "Created album contains its artist" 'len(d["artists"])' 1
jp "Created album contains its genre" 'd["genre"]["id"]' "$GEN"
http "Duplicate title + label -> 409" 409 POST /albums "$ALBUMBODY"
http "Unknown artist id -> 422" 422 POST /albums "{\"title\":\"Bad Artist $TAG\",\"label\":\"Smoke Label\",\"genre_id\":$GEN,\"artist_ids\":[999999]}"
jp "Error envelope: error.code is invalid_reference" 'd["error"]["code"]' invalid_reference
http "Unknown artist was NOT silently created" 404 GET /artists/999999
http "Unknown genre -> 422" 422 POST /albums "{\"title\":\"Bad Genre $TAG\",\"label\":\"Smoke Label\",\"genre_id\":999999,\"artist_ids\":[$ART1]}"
http "Missing title -> 422" 422 POST /albums '{"label":"Smoke Label"}'
jp "Error envelope: error.code is validation_error" 'd["error"]["code"]' validation_error
http "Empty artist_ids -> 422" 422 POST /albums "{\"title\":\"Bad Empty $TAG\",\"label\":\"Smoke Label\",\"genre_id\":$GEN,\"artist_ids\":[]}"
http "GET /albums/{id}" 200 GET "/albums/$ALBUM"
jp "GET /albums/{id} returns artists" 'len(d["artists"])' 1
http "GET /albums/{unknown} -> 404" 404 GET /albums/999999
jp "Error envelope: error.code is not_found" 'd["error"]["code"]' not_found

# copies and availability
http "POST /albums/{id}/copies creates a copy" 201 POST "/albums/$ALBUM/copies" "{\"inventory_no\":\"SMOKE-$TAG-1\"}"
jp "Copy status defaults to available" 'd["status"]' available
http "POST second copy" 201 POST "/albums/$ALBUM/copies" "{\"inventory_no\":\"SMOKE-$TAG-2\",\"status\":\"available\"}"
http "POST third copy (lost)" 201 POST "/albums/$ALBUM/copies" "{\"inventory_no\":\"SMOKE-$TAG-3\",\"status\":\"lost\"}"
http "Duplicate inventory number -> 409" 409 POST "/albums/$ALBUM/copies" "{\"inventory_no\":\"SMOKE-$TAG-1\"}"
http "Copy for an unknown album -> 404" 404 POST /albums/999999/copies '{"inventory_no":"SMOKE-X"}'
http "Invalid copy status -> 422" 422 POST "/albums/$ALBUM/copies" "{\"inventory_no\":\"SMOKE-$TAG-4\",\"status\":\"broken\"}"
http "GET /albums/{id}/copies" 200 GET "/albums/$ALBUM/copies"
jp "The album has 3 copies" 'len(d)' 3
http "GET /albums/{id}/availability" 200 GET "/albums/$ALBUM/availability"
jp "availability.album_id" 'd["album_id"]' "$ALBUM"
jp "availability.total_copies is 3" 'd["total_copies"]' 3
jp "availability.available_copies is 2" 'd["available_copies"]' 2
http "availability of an unknown album -> 404" 404 GET /albums/999999/availability

# update
http "PUT /albums/{id} updates the album and replaces artists" 200 PUT "/albums/$ALBUM" "{\"title\":\"Smoke Album Updated $TAG\",\"label\":\"Smoke Label\",\"genre_id\":$GEN,\"artist_ids\":[$ART2]}"
jp "Title was updated" 'd["title"]' "Smoke Album Updated $TAG"
jp "Artists were replaced (count)" 'len(d["artists"])' 1
jp "Artists were replaced (id)" 'd["artists"][0]["id"]' "$ART2"
http "PUT /albums/{unknown} -> 404" 404 PUT /albums/999999 "{\"title\":\"X\",\"label\":\"Smoke Label\",\"genre_id\":$GEN,\"artist_ids\":[$ART1]}"

# lists
http "GET /albums?genre_id filters by genre" 200 GET "/albums?genre_id=$GEN"
jp "Genre filter returns exactly one album" 'len(d)' 1
http "GET /albums?limit=100" 200 GET "/albums?limit=100"
jp "Seeded data is visible (at least 8 albums)" 'len(d) >= 8' True
jp "Every listed album has a genre and artists" 'len([a for a in d if a["genre"] is None or len(a["artists"]) == 0])' 0
http "Pagination: limit=2 returns 2 albums" 200 GET "/albums?limit=2&page=1"
jp "limit=2 gives exactly 2 items" 'len(d)' 2

# N+1: the number of SELECTs for the list must not depend on the number of albums
count_selects() { $SQL_LOG_CMD 2>/dev/null | grep -c 'SELECT' || true; }
measure() {
  local b a
  sleep 0.3; b=$(count_selects)
  curl -s -o /dev/null "$BASE/albums?limit=100"
  sleep 0.3; a=$(count_selects)
  echo $((a - b))
}
Q1=$(measure)
for i in 1 2 3 4 5; do
  api POST /albums "{\"title\":\"Scale $i $TAG\",\"label\":\"Smoke Label\",\"genre_id\":$GEN,\"artist_ids\":[$ART1,$ART2]}" > /dev/null
done
Q2=$(measure)
if [ "$Q1" -ge 1 ] && [ "$Q1" = "$Q2" ] && [ "$Q1" -le 6 ]; then
  pass "GET /albums uses a constant number of SELECTs ($Q1) regardless of the number of albums"
else
  fail "GET /albums uses a constant number of SELECTs (no N+1)" "SELECTs before=$Q1, after adding 5 albums=$Q2"
fi

# delete and cascade
http "DELETE /albums/{id} -> 204" 204 DELETE "/albums/$ALBUM"
http "GET deleted album -> 404" 404 GET "/albums/$ALBUM"
http "DELETE /albums/{unknown} -> 404" 404 DELETE /albums/999999

# --- Lesson 10: the web UI (html/template + htmx) behind the cookie session ------------
PAGE="$JARS/page.html"; HDR="$JARS/page.h"
page() { curl -s -o "$PAGE" -D "$HDR" -w '%{http_code}' "$@"; } # status to stdout, body and headers to files
[ "$(page "$BASE_URL/catalog")" = "200" ] && grep -qi '^content-type: text/html; charset=utf-8' "$HDR" \
  && grep -q '<!doctype html>' "$PAGE" && grep -q 'id="album-list"' "$PAGE" && grep -q 'hx-get="/web/albums"' "$PAGE" \
  && grep -q 'src="/static/htmx.min.js"' "$PAGE" && ! grep -q 'hx-delete' "$PAGE" \
  && pass "GET /catalog renders the layout and the search form without delete buttons for a visitor" \
  || fail "GET /catalog renders the layout and the search form without delete buttons for a visitor"
[ "$(page "$BASE_URL/static/app.css")" = "200" ] && grep -qi '^content-type: text/css' "$HDR" \
  && pass "GET /static/app.css is served from the binary" || fail "GET /static/app.css is served from the binary"
[ "$(page "$BASE_URL/static/htmx.min.js")" = "200" ] && grep -qi '^content-type: text/javascript' "$HDR" && grep -q 'version:"2.0.11"' "$PAGE" \
  && pass "GET /static/htmx.min.js is served from the binary (htmx 2.0.11)" || fail "GET /static/htmx.min.js is served from the binary (htmx 2.0.11)"
[ "$(page "$BASE_URL/static/")" = "404" ] && [ "$(page "$BASE_URL/static/missing.css")" = "404" ] \
  && pass "No directory listing and a 404 for a missing asset" || fail "No directory listing and a 404 for a missing asset"
http "POST /albums creates the album the web checks search for and delete" 201 POST /albums "{\"title\":\"Web Delete $TAG\",\"label\":\"Smoke Label\",\"genre_id\":$GEN,\"artist_ids\":[$ART1]}"
WEB_ALBUM=$(field "" '["id"]')
[ "$(page -H 'HX-Request: true' "$BASE_URL/web/albums?q=Web+Delete+$TAG")" = "200" ] && [ "$(tr -d '[:space:]' < "$PAGE" | head -c 3)" = "<tr" ] \
  && grep -q "Web Delete $TAG" "$PAGE" && ! grep -qi '<html' "$PAGE" \
  && pass "HX-Request GET /web/albums?q= answers the matching rows only" || fail "HX-Request GET /web/albums?q= answers the matching rows only"
[ "$(page "$BASE_URL/web/albums?q=blue")" = "303" ] && grep -qi '^location: /catalog?q=blue' "$HDR" \
  && pass "A plain GET /web/albums redirects to /catalog?q=" || fail "A plain GET /web/albums redirects to /catalog?q="
[ "$(page -X DELETE "$BASE_URL/web/albums/$WEB_ALBUM")" = "401" ] && grep -q '"unauthenticated"' "$PAGE" \
  && pass "Anonymous DELETE /web/albums/{id} -> 401" || fail "Anonymous DELETE /web/albums/{id} -> 401"
web_login() { curl -s -o /dev/null -c "$1" -w '%{http_code}' -X POST "$BASE_URL/login" --data-urlencode "email=$2" --data-urlencode "password=$3"; }
[ "$(web_login "$JARS/admin.jar" "$ADMIN_EMAIL" "$ADMIN_PASSWORD")" = "303" ] && grep -q '^#HttpOnly_' "$JARS/admin.jar" && grep -q 'albums_token' "$JARS/admin.jar" \
  && pass "POST /login as admin -> 303 with an HttpOnly albums_token cookie" || fail "POST /login as admin -> 303 with an HttpOnly albums_token cookie"
[ "$(web_login "$JARS/bad.jar" "$ADMIN_EMAIL" wrong-password)" = "401" ] && ! grep -qs 'albums_token' "$JARS/bad.jar" \
  && pass "POST /login with a wrong password -> 401 and no cookie" || fail "POST /login with a wrong password -> 401 and no cookie"
[ "$(page -b "$JARS/admin.jar" "$BASE_URL/catalog")" = "200" ] && grep -q "$ADMIN_EMAIL (admin)" "$PAGE" && grep -q 'hx-delete="/web/albums/' "$PAGE" && grep -q 'action="/logout"' "$PAGE" \
  && pass "GET /catalog with the admin cookie shows the account and the delete buttons" || fail "GET /catalog with the admin cookie shows the account and the delete buttons"
WEB_MEMBER="web-member-$TAG@albums.local"
http "Register a member for the web checks" 201 POST /auth/register "{\"email\":\"$WEB_MEMBER\",\"password\":\"$ADMIN_PASSWORD\"}"
[ "$(web_login "$JARS/member.jar" "$WEB_MEMBER" "$ADMIN_PASSWORD")" = "303" ] \
  && pass "POST /login as a member -> 303" || fail "POST /login as a member -> 303"
[ "$(page -b "$JARS/member.jar" -X DELETE "$BASE_URL/web/albums/$WEB_ALBUM")" = "403" ] && grep -q '"forbidden"' "$PAGE" \
  && pass "Member DELETE /web/albums/{id} -> 403" || fail "Member DELETE /web/albums/{id} -> 403"
[ "$(page -H "Authorization: Bearer $AUTH_TOKEN" -X DELETE "$BASE_URL/web/albums/$WEB_ALBUM")" = "401" ] \
  && pass "A bearer token is not a web session: DELETE /web/albums/{id} -> 401" || fail "A bearer token is not a web session: DELETE /web/albums/{id} -> 401"
[ "$(page -b "$JARS/admin.jar" -H 'Sec-Fetch-Site: cross-site' -X DELETE "$BASE_URL/web/albums/$WEB_ALBUM")" = "403" ] && grep -q 'cross-origin' "$PAGE" \
  && pass "Cross-site DELETE with a valid cookie -> 403 (CrossOriginProtection)" || fail "Cross-site DELETE with a valid cookie -> 403 (CrossOriginProtection)"
http "The album survived the rejected deletes" 200 GET "/albums/$WEB_ALBUM"
[ "$(page -b "$JARS/admin.jar" -X DELETE "$BASE_URL/web/albums/$WEB_ALBUM")" = "200" ] && [ ! -s "$PAGE" ] \
  && pass "Admin DELETE /web/albums/{id} -> empty 200 (htmx swaps the row away)" || fail "Admin DELETE /web/albums/{id} -> empty 200 (htmx swaps the row away)"
http "The deleted album is gone from the API" 404 GET "/albums/$WEB_ALBUM"
[ "$(page -H 'HX-Request: true' "$BASE_URL/web/albums?q=Web+Delete+$TAG")" = "200" ] && grep -q 'class="empty"' "$PAGE" \
  && pass "The fragment no longer lists the deleted album" || fail "The fragment no longer lists the deleted album"
[ "$(curl -s -o /dev/null -b "$JARS/admin.jar" -c "$JARS/admin.jar" -w '%{http_code}' -X POST "$BASE_URL/logout")" = "303" ] && ! grep -q 'albums_token' "$JARS/admin.jar" \
  && pass "POST /logout -> 303 and the cookie is expired" || fail "POST /logout -> 303 and the cookie is expired"
catalog_selects() {
  local b a
  sleep 0.3; b=$(count_selects)
  curl -s -o /dev/null "$BASE_URL/catalog"
  sleep 0.3; a=$(count_selects)
  echo $((a - b))
}
QC=$(catalog_selects)
if [ "$QC" -ge 1 ] && [ "$QC" -le 5 ]; then
  pass "GET /catalog costs $QC SELECTs: the page, its relations and one availability query"
else
  fail "GET /catalog costs at most 5 SELECTs (no N+1 for the availability column)" "SELECTs=$QC"
fi

echo "== done: $([ "$FAILED" = 0 ] && echo 'all checks passed' || echo 'some checks FAILED')"
exit $FAILED
