<!-- Final review of Lesson 14 (step 3.1): the answer of a fresh read-only subagent (Claude Fable 5.1) to the
prompt in prompts.txt, verbatim. The triage is in the README, section "Фінальний огляд". -->

# Lesson 14 review: live chat in the albums app

Repository: `/Users/oleksandr.tatarchuk/repo/golang-course-lesson-14`, range 82d08e8..8927043 (8 commits, 84 → 92 commits). Reviewed read-only; nothing in the working tree, index or refs was touched. File:line references below are relative to the repository root.

What I ran on the Mac (Go 1.27.1): `gofmt -l .` (clean), `go vet ./...` (ok), `go build ./...` into the scratchpad (ok), `go test -race -count=1 ./internal/chat/ ./internal/middleware/ ./internal/handler/web/` (all ok). What I read instead of running: the VM run at 8927043 (`app-tests-head.txt`: `go vet` ok, 324 PASS lines, 0 FAIL, 0 SKIP, every package ok), the CI static replay (`static-replay2.out`: 53 PASS, 0 FAIL), GitHub run 41054119445 (now fully green: static 54 s, integration 2m11s, both serverless jobs), the failed run 41049381909 (static job failed on "No non-ASCII characters in .go files", as the README says), `docs/chat/*.txt` and both screenshots.

### Strengths

- **The hub is correct under the slide 42 questions.** `h.rooms` and `h.history` are touched only by `Run` (CI pins it). `c.send` is closed in exactly one place (`drop`, hub.go:111-120) and only while the client is registered, so a late unregister, a shutdown after a drop, or a drop after a shutdown cannot double-close. `closeCode`/`closeText` are written before `close(c.send)` and read only after the receive that observes the close: a proper happens-before edge, no lock needed. Every hub send to a client is non-blocking; a full queue drops the client with 1008 and the "left" presence is queued, not recursed into. `Publish`, `Stats`, `registerClient`, `unregisterClient` and `sendError` all select on `h.done`, so nothing blocks after `Shutdown`. The one allowed second writer (gorilla's own close/ping handlers via `WriteControl`) is the documented concurrency-safe path.
- **Every goroutine has an exit path**: readPump ends on any read error (client close, abrupt drop, pong deadline, read limit); writePump ends when the hub closes `send` or a write fails; each closes the conn, which ends the other. `TestConnectDisconnectCyclesNoLeak`, `TestShutdownClosesClientsWithGoingAway`, `TestDeadPeerTimesOut`, `TestAbruptDisconnectUnregisters` prove it under `-race`, and the CI chatload cycles prove it against the real binary.
- **Test-first is real**: `docs/chat/tests-before.txt` shows the three packages failing to build, `hijack-fix.txt` shows the red 500 and the green 101. The 26 Lesson 14 tests map one-to-one onto the plan's list.
- **Security layering is right**: cookie session (HttpOnly, SameSite=Lax, Path=/) + `RequireRole` + a second claims check + account lookup (deleted account → 401) + same-origin check + CSP page, with the API stats admin-only behind Bearer. `chat.js` renders with `textContent`/`createTextNode` only; the server never reflects client-chosen `type`, `room` or `author`.
- **The decorator is a clean slide 41 answer**: no handler changed, a failed write publishes nothing, delete names the album.
- **Evidence and reporting are consistent and honest**: every number I checked (table below) traces to a file; the cut browser proof, the unverified Safari CSP behaviour and the skipped restart check are stated in the README rather than hidden.
- **CI checks are not vacuous where it matters**: the chatload cycles really run (CI log: 9 → 9 and 9 → 11 goroutines, 400/400 messages), the anonymous checks hit the real routes, and the grep checks pin the exact source lines.

### Issues

#### Critical (Must Fix)

None.

#### Important (Should Fix)

**I1. The process can exit before the chat connections have been closed with 1001.**
`cmd/api/main.go:82` and `:101-103`; `internal/chat/hub.go:207-210`.
`http.Server.Shutdown` runs every `RegisterOnShutdown` hook with `go f()` and its own doc says the hook "should not wait for shutdown to complete"; hijacked connections are removed from the server's tracking, so `closeIdleConns()` is true on the first poll and `srv.Shutdown` returns at once when no plain HTTP request is in flight. `run()` then returns, `main` exits, and whether `hub.Shutdown` has finished its sweep and the writePumps have written their 1001 frames is a race. `Hub.Shutdown` itself returns when `Run` has returned (`<-h.done`), i.e. when the queues are closed, not when the pumps have written the close frames. In the test process this is invisible (the process stays alive and `goroutinesBackTo` polls for 3 s); in `docker compose restart api` the browser will often see 1006 instead of the promised 1001. The README (`:81`, `:124`) and the plan's review-focus item 5 claim 1001 on shutdown; the manual restart check that would have shown this was cut. This is partly a gap in the plan: `RegisterOnShutdown` alone cannot give the guarantee.
Effect on a user: the page still reconnects (chat.js retries on any close), so the damage is a wrong status text and a close that is abrupt instead of clean.
Fix: (a) in `run()`, after `srv.Shutdown(shutdownCtx)` returns, call `hub.Shutdown()` again (it is idempotent) so the sweep is guaranteed; (b) make `Shutdown` wait for the pumps: have `drop` (hub goroutine only) append the client to a `closing` slice and have `Shutdown`, after `<-h.done`, wait on each client's `closed` channel (closed in writePump's defer) with `WriteWait` as the bound; or keep a `sync.WaitGroup` whose `Add` happens inside `join` and `Done` in writePump's defer.
Test: a test that wires `http.Server` + `RegisterOnShutdown(hub.Shutdown)` as main does, connects 10 clients, calls `srv.Shutdown` followed by the synchronous `hub.Shutdown()`, and asserts without polling that every client reads 1001 and `runtime.NumGoroutine()` is at the baseline the moment the call returns. `TestShutdownClosesClientsWithGoingAway` currently needs the 3 s poll; after the fix it should not.

**I2. The documented Origin rule is stronger than the code: a handshake without an Origin header is accepted.**
`internal/chat/handler.go:98-105`; `README.md:76`; `albums_lesson14_homework.md:68` and `:137`; plan contract "Origin must equal Host, else 403".
`sameOrigin` returns true when the header is absent (deliberate, per the comment: a non-browser client still needs the cookie). Browsers always send Origin on a WebSocket handshake, so cross-site WebSocket hijacking is still blocked, and SameSite=Lax would not send the cookie cross-site anyway, so this is not an exploitable gap. But the README, the homework file and the plan state "Origin must equal Host (else 403)", the examples repo rejects a missing Origin (its plan test expects 403), and nothing in the app's tests pins the current behaviour either way. A grader who probes it will find the doc wrong.
Fix: either reject a missing Origin (one line; chatload and every test already send one) or state the exception in the README/homework table. Test: add a `"no Origin header"` case to `TestForeignOriginRejected` or `TestRoomAccess` asserting whichever you choose.

#### Minor (Nice to Have)

**M1. README step 8 says the letter was "written as `ж`" - the literal letter, not the escape.**
`README.md:120-121` (`grep -n u0436 README.md` finds nothing); the test has `"ж"` at `internal/chat/handler_test.go:103`. The sentence as written is self-contradictory. Same slip in the message of commit abf0a11 ("the escape ж"), which cannot be changed. Fix: write `` `ж` `` in the README.

**M2. The tests table credits `TestChatPageScriptOnly` with "CSP на місці", but that test does not check the header.**
`README.md:135` vs `internal/handler/web/chat_test.go:53-62` (the web fixture at `fakes_test.go:241` has no `SecurityHeaders`). The CSP on `/chat` is pinned by `TestChatRoutesWired` (`internal/app/chat_test.go:72-77`), and `TestWebSecurityHeadersAndRootFallback` covers only `/catalog` and `/login`. Fix: move "CSP на місці" to the `app` row (and optionally add `/chat` to the path list of `TestWebSecurityHeadersAndRootFallback`). The plan asked `TestChatPageScriptOnly` to check the header; that is a plan-vs-implementation deviation, not a missing guarantee.

**M3. A client the hub dropped with 1008 can keep publishing until its socket actually closes.**
`internal/chat/hub.go:77-79` vs `:80-83`; `internal/chat/client.go:52-57`. The `direct` path checks membership, the `broadcast` path does not. After `drop`, the client's readPump still runs until writePump has drained the backlog (up to 256 frames at ≤ 10 s each) and sent the close, so the room can see "X left" followed by messages from X, and those messages enter the history. Rare in practice (it needs a full queue). Fix: give `Client` a `gone chan struct{}` closed in `drop` (hub goroutine, exactly once, like `send`) and have readPump skip `Publish` when it is closed; or route client messages through a `direct`-like struct carrying `*Client` so `Run` can check membership. Test: in `TestStuckClientDropped`-style setup, have the stuck client publish after the drop and assert the healthy client gets nothing.

**M4. chatload reports "back to the baseline" while the delta is +1 or +2.**
`cmd/chatload/main.go:85-99`. The poll stops as soon as the tolerance is met; the CI abrupt run printed `goroutines=11 (+2 against the start)` followed by `OK: ... back to the baseline`. Within the plan's `before + 2` rule, but the message overstates. Fix: keep polling until the delta is 0 or the settle deadline passes, then judge with the tolerance; print "within tolerance" when it is not 0.

**M5. E-mail addresses go into the server log on every join and leave.**
`internal/chat/hub.go:100`, `:118`, `client.go:29`. The Lesson 12 `Logging` line deliberately prints only method/path/status; the chat adds PII to the log of a server that CI greps for tokens. Fix: log the user id (or a hash) in `Client.String()` and keep the e-mail for the frames only.

**M6. chat.js reconnects forever after a refused handshake.**
`web/static/chat.js:66-72`. After `/logout` in another tab or a 24 h token expiry, every retry gets a 401 (`close 1006`) and the page shows "reconnecting in 30 s" indefinitely while the server logs a `GET /ws/chat -> 401` line every 30 s. The WebSocket API hides the HTTP status, but a bounded number of retries, or giving up on a close code the server chooses (e.g. 1008 or a 4xxx code sent before refusing), would end it. Nice to have.

**M7. `Options.Validate` accepts `SendBuffer = 1`, which drops every client at its own join.**
`internal/chat/message.go:70-73`. The history frame fills the queue, the client's own "joined" presence overflows it, and `publish` drops it with 1008 before it has read anything. The error text already describes the symptom for 0; require ≥ 2 (history + join) and extend `TestOptionsValidate`.

### Numbers checked

| Claim | Where it appears | Source checked | Match? |
|---|---|---|---|
| 8 commits in the range, 84 commits before, history rewrite kept the count | brief; README step 1; history-rewrite.txt | `git rev-list --count 82d08e8` = 84, head = 92; `git log --format='%ae %ce' \| sort -u` = gmail + the one noreply committer | yes |
| 83 commits re-authored, trees/dates/messages unchanged | README step 1 | docs/chat/history-rewrite.txt (uniq -c 83/1, identical tree hash, empty diff) | yes |
| 296 baseline tests with -race | README step 2, Докази | baseline-tests.txt "PASS lines: 296" (includes the Hijack test: the run was taken after 50d6c46) | yes |
| 324 tests, 0 SKIP, 0 FAIL after the chat | README step 5, Докази; commit dd4350d | tests-after.txt; VM run at head: 153 top-level + 171 subtest PASS lines = 324 | yes (324 counts subtests; 26 counts top-level only) |
| 26 Lesson 14 tests | README step 4, Докази | tests-after.txt list; `grep -c '^func Test'`: 19 chat + 3 web + 2 app + TestNewRouterPanicsWithoutChatHub + TestLoggingPassesHijackerThrough = 26; 296 + 25 + 3 subtests = 324 | yes |
| 19 tests in package `chat` (C1) | README screenshots table | c01-tests.png shows 19 `--- PASS` lines; files: 7 + 7 + 2 + 3 | yes |
| 107 of 107 smoke checks | README step 7, Докази; ledger | smoke.txt: 107 `PASS:`, 0 `FAIL:` | yes |
| 2 × 100 connections, 400 of 400 messages per cycle, goroutines 9 → 9 | README step 7 | smoke.txt (20 clients × 5 cycles per mode; 20 × 20 = 400) | yes |
| CI runs chatload 20 × 5 with and without a close frame | README CI paragraph | theme8-check.yml:647-651; run 41054119445 log: both PASS, 9 → 9 and 9 → 11 (+2, within tolerance) | yes |
| 0 clients, 9 goroutines after the browser closed; 2 clients / 17 goroutines during | README step 6, C2 row | run.txt section 2 | yes |
| stats: admin 200, member 403, no token 401 | README step 6; CURL.md §9 | run.txt section 1 (staff 403 in CURL.md is by `RequireRole(admin)` only, not in the log) | yes |
| member → `staff` room 403 before upgrade | README step 6; CURL.md §9 | run.txt (`handshake refused: 403 Forbidden ... the staff room is for staff and admin`) | yes |
| stats body `{"clients":0,"rooms":{"general":0,"staff":0},"goroutines":9}` | CURL.md §9 | run.txt | yes |
| Ubuntu 24.04.3 arm64, 4 CPU, Docker 29.9.2, Compose v5.6.1, Go 1.27.3, gcc; macOS 27.2, Go 1.27.1, Chrome 156 | README Середовище | environment.txt (Chrome 156.0.8078.74, Safari 27.2 listed but not run) | yes |
| gorilla/websocket v1.5.3 | README, CI check | go.mod | yes |
| ping 54 s, pong 60 s, write 10 s, queue 256, history 50, 1-1000 runes, 4096 bytes | README table, homework, CURL.md | message.go `DefaultOptions` | yes |
| close codes 1008 slow consumer, 1001 shutdown, 1009 too big | README, homework, CURL.md | hub.go:133/147 (`ClosePolicyViolation`), :89 (`CloseGoingAway`), gorilla read limit; `TestStuckClientDropped`, `TestShutdownIsIdempotentAndUnblocks`, `TestTextValidation` | yes (but see I1 for the process-level 1001 guarantee) |
| reconnect backoff 1 → 30 s | README step 5; chat.js header | chat.js:67 `min(30, 2^retry)` | yes |
| `/chat` anonymous → 303 `/login` | README, smoke | web/chat.go:20-23; smoke.txt; CI log | yes |
| `/ws/chat`: 401 no session / deleted account, 404 unknown room, 403 role or Origin, 503 hub stopped | README table | handler.go:46-77; `TestRoomAccess`, `TestForeignOriginRejected`, `TestChatRoutesWired` (503 path untested) | yes |
| "Origin must equal Host, else 403" | README:76, homework:68/137 | handler.go:98-105: a missing Origin passes | **no** (I2) |
| `TestChatPageScriptOnly` proves "CSP на місці" | README:135 | web/chat_test.go checks script tags only; CSP pinned in app `TestChatRoutesWired` | **no** (M2) |
| the Cyrillic letter "written as `ж`" | README:121 | handler_test.go:103 has `ж` | **no** (M1) |
| static job of the pushed commit failed on the ASCII check | README step 8; ledger | `gh run view 41049381909`: "X No non-ASCII characters in .go files" | yes |
| 16 and 64 overflowed on the slide 43 scenario with 100 clients | README row "28" | examples README lines 172-175 | yes |
| 100 clients over 5 cycles, graceful and abrupt, goroutines ≤ baseline + 2 within 3 s | README tests table | leak_test.go: 5 × 20, even/odd split, `goroutinesBackTo(3 s)` | yes |
| 51 messages → joiner gets 50, oldest dropped | README tests table | `TestHistoryReplayedOnJoin` (m2..m51) | yes |
| queue 2 stuck client, healthy gets 10, 1008 | README tests table | `TestStuckClientDropped` | yes |
| 1001 runes → error frame, 5000 bytes → 1009 | README tests table | `TestTextValidation` | yes |
| notice text `Album "Blue Train" (Blue Note, jazz) was added to the catalogue` | README, CURL.md, plan | albums.go:36, `TestNotifyOnCreate`; `Create` reloads the album with Genre (album_repository.go:63) | yes |
| 53 PASS / 0 FAIL static replay | ledger | static-replay2.out | yes |
| VM `albums-lesson14`, provisioners `docker-engine,go`, port 8080 → Mac 127.0.0.1 | README Як запустити | Vagrantfile:17-18, :26, :38-39; deploy/vagrant/go.sh | yes |
| `make test` = `go test ./... -race`; `make up`, `make bootstrap`, `make chatload` exist; admin from .env | README, CURL.md | Makefile:8-10 (include .env, export), :16, :34, :38, :52-53 | yes |
| C1, C2 screenshots exist and show what the rows say | README, homework | c01-tests.png (19 PASS + full suite ok), c02-chat.png (2 online, both messages once on both pages, no `#staff` for member1) | yes |

### Rulings

1. **Task 2.5, browser run cut to one headless-Chrome case** - stands: the user's decision, disclosed in the README with the exact gaps (Safari CSP, restart/reconnect). Note that the cut restart check is precisely where I1 would have surfaced; fix I1 in code rather than re-running the browser.
2. **Task 2.6, ASCII fix in its own commit** - stands; the fix is right (`ж`, same rune count). The README sentence describing it is wrong (M1).
3. **Task 2.6, no prompts.txt until 3.1** - stands; the CI token guard uses `grep -s`, so the absent file causes no spurious failure.
4. **Task 2.6, homework file updated to the built design (256, basic browser case, C1-C2)** - stands, with a caveat: editing the task text to match the build is only acceptable because the README table "Чим відрізняється від слайдів" records the 16 → 256 reasoning with its evidence; keep that row if the homework file is ever trimmed.

### Declined to judge

- The final presentation (slide 48 item 3): excluded by the user's scope decision.
- Per-user connection limits and rate limiting (a 4 KB message from one client fans out to the whole room): the plan lists them as out of scope.
- Chat persistence across restarts, message editing, typing indicators, TLS/`wss://`: out of scope by the plan.
- A role change or account deletion not reaching an already-open connection, and `CanJoin` using the token's role rather than the freshly loaded account's: the Lessons 9-10 JWT model (24 h TTL) applies to every route; the web page uses the same `claims.Role`, so page and socket agree.
- Other members seeing each other's e-mail addresses in frames: the spec makes the account e-mail the author.
- `go.mod` still says `go 1.25.11` while the plan says Go 1.27: unchanged Lesson 12 toolchain; CI builds with it and the code compiles under it.
- `CORS` answering `Access-Control-Allow-Origin: *` on the API: Lesson 9 design, not touched by this lesson (and irrelevant to the upgrade response, which gorilla writes itself).
- Node 20 deprecation and the tar cache warning in the Actions annotations: runner-image noise, not this repository.
- The examples repository: reviewed separately.

### Recommendations

1. Fix I1 and tighten `TestShutdownClosesClientsWithGoingAway` to assert the goroutine count immediately after `Shutdown` returns; add the `cmd/api`-style wiring test.
2. Decide I2 (reject a missing Origin, or document the exception) and pin it with a test case.
3. Fix the three doc mismatches (M1, M2, and the Origin wording from I2) in the same "final review findings" commit; they are one-line edits.
4. Consider the explicit `connect-src 'self'` (or `ws:`/`wss:` for the chat page) that the plan named as the fallback: it removes the dependency on how Safari resolves `'self'` for `ws://`, which this run did not verify, at no cost to Chrome. Pin it in `TestWebSecurityHeadersAndRootFallback` by adding `/chat` to its path list.
5. CI checks (item 4 of the brief): all nine Theme 14 checks catch what their names claim; `rooms_one_owner` and `one_writer` pass vacuously only if the identifiers are renamed (accepted for grep checks), `one_writer` scans `internal` only (fine, no server code under `cmd` writes frames), and the literal `mux.Handle(...)` greps are brittle to reformatting but not vacuous. Spurious-failure risk on the GitHub runner is low: the chatload verdict polls for 5 s before applying the `+2` tolerance, which the abrupt run already needed (+2); if the runner ever gets slower, raise `-settle` rather than the tolerance.
6. M3-M7 at leisure; none affects the grading.

### Assessment

**Ready to merge?** With fixes. **Reasoning:** the hub, pumps, handshake and tests are sound and race-free, the evidence matches the report, and CI is green; but the one guarantee the report makes about the running server that the tests cannot see, 1001 on shutdown, is racy in `cmd/api` (I1), and two statements in the README/homework contradict the code (I2, M1-M2). All are small, local changes.
