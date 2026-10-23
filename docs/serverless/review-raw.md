# Final review, raw output (verbatim)

- Date: 2026-10-23
- Reviewer: Claude Fable 5.1, a fresh Claude Code general-purpose subagent (no context from the implementing session)
- Range: `1f3a714..7b423b9` (15 commits); prompt: `prompts.txt`, section "Final review"
- Triage of the findings: `README.md`, section "Lesson 12 Report" → "Фінальне рев'ю"

---

## Review of 1f3a714..7b423b9 (Lesson 12, Go Serverless)

Reviewed in passes: (1) the own function `serverless/albums-lambda/**` against the plan's Interfaces, test contract and the five Review Focus items; (2) the Terraform draft-to-review diff aa5f094..5c8ec38 against the README table and the plan outputs; (3) `serverless/Vagrantfile` and the demo copy against the originals in `~/Downloads`; (4) README, CI workflow, prompts.txt and every evidence file; (5) the Lesson 10 removal and root-code comment repoints. Nothing in the checkout was modified; the only writes were toolchain downloads into the Go module cache and scratch copies in the scratchpad.

### What I ran (all green)
- `serverless/albums-lambda`: `gofmt -l` empty; `go vet` ok; `go test -race -count=1 -cover` → 16 tests pass, 96.3% coverage, on Go 1.27.1 **and** on Go 1.26.0 (`GOTOOLCHAIN=go1.26.0`, the floor CI's `go-version-file` will pick); ASCII-only check (perl) empty; `bash -n scripts/*.sh` ok.
- `serverless/libratrack-lambda-demo`: `go vet` + `go test -race` pass on 1.27.1; `gofmt -l` empty; `go mod tidy -diff` empty; `bash -n` ok. Under `GOTOOLCHAIN=go1.22.12` (what CI will select from its `go 1.22`): `go vet` passes and both test binaries compile for linux/amd64 (`go test -c`). Running 1.22 binaries on this Mac aborts in dyld with `missing LC_UUID load command`, a macOS 27 vs old-Go issue, not a code problem; ubuntu-latest is unaffected.
- CI static job replayed locally: all 61+ `required` files present; no `AutoMigrate`, `ErrNotFound = ` exactly once, no `"text/template"`, no `ParseGlob`, no `template.HTML` family, no JWT-like strings in docs/README/prompts.txt, no non-ASCII in `.go` outside the demo; workflow YAML parses with jobs `static`, `integration`, `serverless`; root `gofmt`/`go vet`/`go build` ok.
- Hygiene: `git log` 1f3a714..7b423b9 has no Co-Authored-By/Claude/Generated lines; `.env` untracked (`.env.example` only); no tfstate/keys/tokens tracked; `.superpowers/` and `.DS_Store` untracked (global ignore); working tree clean.
- AI draft provenance: all four `terraform/*.tf` at aa5f094 are exact substrings of `docs/serverless/albums/terraform-ai-raw.md` (checked programmatically) — "committed unchanged" is true. The prompt in prompts.txt matches `docs/ai-prompts.md` Prompt 1 + Floci add-on verbatim.
- README vs evidence: every number I checked matches (tool versions, 1828 MB, 93.7/77.8% and 96.0%, zips 2.7/3.5/2.6 MB and 5.4 vs 2.7 MB, 200/400/400 everywhere, smoke 9/0 twice, Plan 15 → Apply 15, Plan 11 before and after → Apply 11, bench 461/328, 479/337, 507/337, CLI 317–329 and 336, Init Duration 0.02–0.10 ms, handler 2–5 ms, the demo's x86_64 default, the "unexpected" plan values `skip_destroy=false`, `publish=false`, `reserved_concurrent_executions=-1`, `region` on every resource, `lambda_logs will be read during apply`, `source_arn = (known after apply)`). All README relative links resolve; all 17 PNGs and all 20 evidence files are referenced.
- GitHub: origin/main is at 5c8ec38 with three green runs of the old workflow; the new `serverless` matrix job has not run yet (nothing after 5c8ec38 is pushed).

### Strengths
- **Tests first, honestly recorded**: `docs/serverless/albums/tests-before.txt` shows the real red run (`undefined: Config/Album/normalize`), `tests-after.txt` the green one. 16 focused tests, table-driven where it matters, through both `handle` (injected cfg/clock) and `handler` (the real entry point, via the fixtures).
- **All five Review Focus items are implemented and pinned by the named tests**: `collapse` via `strings.Fields` (handles U+00A0, tabs, newlines) — `TestNormalizeCleansUpSubmission` (album_test.go:23-47, with the NBSP as an ASCII escape); `*json.UnmarshalTypeError` → `{"error":"invalid field type","field":...}` (handler.go:66-72) — `TestClientErrorsAre400Not502` rows "release_year is a string"/"artists is a string"; `RawMessage` + `{` prefix check (handler.go:62-65) for null/array/string/missing — same test; case-insensitive dedupe keeping the first spelling and the blank-name problem (album.go:67-86) — `TestNormalizeCleansUpSubmission`, `TestBlankArtistNameIsAProblem`; year bounds `[MIN, now+1]` naming both bounds (album.go:52-55) — `TestReleaseYearBounds` with all six edge cases.
- **The 400-never-502 contract is airtight**: every client path returns `respond(..., nil)`; the test helper fails on any non-nil Go error. Package-level `logger`/`cfg` with an init-phase panic on bad config (`TestMustLoadConfigPanics`, `TestPackageLevelConfig`) matches slides 22-23 and the spec's init-phase rule.
- **Terraform work is genuinely line-by-line and verifiable**: draft commit verbatim, review commit as a diff, and the README table's "Так" rows are exactly the diff (architectures → `var.lambda_architecture` + new validated variable, `function_name` default, `env.sh` `API_NAME`). The "Ні" rows carry real reasons (lock file pins 6.70.0, `make plan-tf` depends on `build`, no `logs:CreateLogGroup` as a conscious trade-off). The "what I did not expect" list is grounded in the plan output, not invented.
- **Demo discipline**: copied verbatim; the four run fixes are each their own commit with a reason, and the Floci failure is logged (`floci-first-start-failure.txt`) rather than just fixed. `diff -r` against `~/Downloads` confirms only those lines differ.
- **Vagrantfile**: `diff -u` against the original confirms only the eight header-listed changes; `require_relative "../deploy/vagrant/port_check_fix"` resolves.
- **CI design**: matrix job with per-module `go-version-file`, `defaults.run.working-directory`, `-race`, `bash -n`; ASCII exclusion scoped to the demo directory; `prompts.txt` back under the JWT guard; the Lesson 12 required files are all real.
- **Evidence discipline**: nothing in the report that I could not find in a file, with one small exception below (the digest claim).

### Issues

#### Critical (Must Fix)
None.

#### Important (Should Fix)
1. **README forward-references deliverables that do not exist yet** (planned Task 7 Steps 5-6; this review is Step 5). README.md:427 says `prompts.txt` holds "промпт фінального рев'ю" — prompts.txt (69 lines) has only the Terraform prompt. README.md:174-175 says the Lesson 12 green CI run is "у розділі Lesson 12 Report" — that section has no CI link, and the `serverless` job has never run (origin is at 5c8ec38). `docs/serverless/review-raw.md` and `### Фінальне рев'ю` are also absent. Why it matters: the spec's deliverables 3 and 4 require exactly these (green CI, final-review prompt); as the README stands, a grader following its own pointers finds nothing. Fix: append this review's prompt to prompts.txt, save the raw output to `docs/serverless/review-raw.md`, add `### Фінальне рев'ю`, push, then link the green run (the planned `docs: link to the green CI run` commit) — and verify the `serverless` matrix job specifically is green on that run.

#### Minor (Nice to Have)
1. `cmd/webhook/album_test.go:23-31` — Review Focus 1 names newline; the input covers NBSP (` `) and tab only. `strings.Fields` handles `\n`, but the pin is incomplete. Add `\n` to one field (e.g. `Label: " Columbia\n"`).
2. `cmd/webhook/handler.go:73-74` — the non-type-error fallback answers `data must be a JSON object`. It is unreachable (the RawMessage is already valid JSON starting with `{`), but if a future change makes it reachable the message would be wrong. Say `invalid data` there, or drop the branch with a comment.
3. `cmd/webhook/album.go:28` — genre is trimmed and lowercased but not `collapse`d, unlike title/label/artists; a multi-word genre with doubled internal spaces would fail the allow-list. Harmless with the default single-word genres; `collapse` + `ToLower` would be consistent.
4. `cmd/webhook/config.go:39-45` — `MIN_RELEASE_YEAR` is not checked against `now.Year()+1`; a value above it rejects every year with a self-contradicting message ("between 2100 and 2027"). An init-phase error would be in the spirit of `mustLoadConfig`.
5. `scripts/env.sh:24-25` — `API_NAME` and `STAGE` are exported but nothing in albums-lambda reads them (no deploy-cli here). Dead config; drop or document as parity with the demo.
6. README.md:529 and :600 — "2.3.0 (= `latest` за digest)" / "`latest` — той самий образ" has no evidence line in `docs/serverless/`. Add the `docker image inspect --format '{{.RepoDigests}}'` output for both tags to `floci-first-start-failure.txt` or `environment.txt`.
7. `terraform/main.tf` and `template.yaml` — neither sets `ALLOWED_GENRES`/`MIN_RELEASE_YEAR`, so the deployed function can only run on defaults; the review table does not note the missing `environment` block (the prompt never mentioned the variables, so the AI could not know). Worth one table row or a Terraform variable with the defaults.
8. `Makefile:5` — `.PHONY` omits `build-AlbumWebhookFunction` (same as the demo); a file of that name would silently skip the SAM build. Trivial.
9. `TestPackageLevelConfig`/`TestEventFixtures` depend on `ALLOWED_GENRES`/`MIN_RELEASE_YEAR` being unset in the process environment; fine in CI, could surprise a developer with a customised shell. A comment already says so; `t.Setenv` cannot help since the package var is initialised before tests run.

### Declined to judge
- Ukrainian comments in `serverless/albums-lambda/terraform/*.tf`: the plan's "commit the AI draft unchanged" outranks the English-comments convention for this folder, review notes are in English, and CI's ASCII check covers only `.go`.
- No screenshots for the own function's Terraform plan/apply: user's ruling (Task 5, "references enough"); four evidence files are linked instead.
- `API_NAME=albums-webhook-api` instead of the plan's `albums-api`: Task 5 ruling kept the AI's derived `<function>-api` name.
- `go 1.26` instead of the plan's `go 1.25.11`: aws-lambda-go v1.55.1 declares Go 1.26; the plan allowed the raise.
- The demo's own code, Makefile and scripts: course code, reviewed only on the changed lines (compose pin + named volume, two `GOARCH=arm64` lines, `Architectures: [arm64]`, go.mod/go.sum tidy), all justified and logged.
- Accuracy of the published cold-start sources and the Floci-vs-AWS absolute numbers: web sources quoted with dates and the required caveat; Task 6 ruling dropped the live data.
- The demo template's Part 3 note: not graded, spec says skipped.
- `docs/devops/demo1-ec2.txt:7` `BEGIN RSA PRIVATE KEY`: outside the range (Lesson 10), and the key body is redacted ("... (private key, 27 lines)").
- CR bytes in `docs/serverless/demo/floci.txt`: raw `curl -i` headers captured as evidence; fidelity, not a defect.

### Recommendations
- After the push, open the `Serverless (Lesson 12) vet and tests` job for both matrix entries before linking the run; it is the only CI path never exercised so far (my local 1.22.12/1.26.0 checks say it will pass).
- Consider exposing `ALLOWED_GENRES`/`MIN_RELEASE_YEAR` as Terraform variables with the current defaults; it turns the config contract into something an operator can see in `plan`.
- Keep the draft/review two-commit pattern for any future AI-generated infrastructure; it made this review's Terraform check mechanical.

### Assessment

**Ready to merge?** With fixes

**Reasoning:** The code, tests, Terraform review, evidence and CI are correct and consistent — every Review Focus item is implemented and pinned, every README number matches its evidence file, and the AI draft/review split is verifiably honest. The only blocking gap is the planned closing step: the README already points at a final-review prompt, a review log and a Lesson 12 green-CI link that do not exist yet (Important #1); land those (and optionally the minor nits) and it is ready to submit.
