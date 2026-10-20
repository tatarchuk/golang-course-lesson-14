# Final review of the Lesson 10 extra part (DevOps): raw report

A fresh Claude Code agent (Opus, read-only on the checkout and the running VM) reviewed the branch
`lesson10-devops` (ab22431..f49b576, 14 commits, 46 files) against the homework, the plan, the deck
and the execution ledger on 2026-10-20. The report below is unedited; the triage is in README.md,
section «Lesson 10 — додаткова частина», «Фінальне рев'ю».

---

## Review: lesson10-devops (ab22431..f49b576), 14 commits, 46 files

I read the plan (with its Global Constraints and Review Focus), the ledger rulings and the deck text. Then I read the whole diff in four passes and checked it against the repository, the Vagrant 2.4.9 gem source and the running VM.

Everything I ran was read-only:
- **Mac:** gofmt and the CI greps, the Lambda tests, the port-probe test, `VBoxManage showvminfo`, `lsof`, screenshot hashes.
- **VM:** `kubectl get` / `kubectl diff` (server dry run, nothing saved), `docker ps`, `stat`, version commands, and greps that compare secret values without printing them.

The working tree is still clean.

### Strengths
- **All five Review Focus risks are handled, and the evidence shows each one working:**
  1. The Lambda is built with GOARCH=arm64, and both invokes return `statusCode 200` with the handler's bodies.
  2. `FLOCI_SERVICES_EC2_MOCK=true` is set, and the instance shows `running`.
  3. The `pg_isready` initContainer works: its log shows "no response" twice, then "accepting connections", and RESTARTS is 0.
  4. The image is imported with `k3s ctr -n k8s.io` and runs with `IfNotPresent`.
  5. `COOKIE_SECURE=false` is set, and login returns 303 with a redacted Set-Cookie that has no `Secure` flag.
- **The running cluster matches the committed files exactly.** `kubectl diff -k /app/deploy/k8s` and `kubectl diff -f bootstrap-job.yaml` both return rc=0 with no output.
  - Both API pods and postgres-0 are 1/1 Running with RESTARTS 0, the PVC is Bound and the Job is Complete.
  - From the Mac, `/health` answers on ports 30080, 8081 (with the Host header) and 8080.
- **No secrets leak.**
  - The k8s env file is mode 600, and the DB password is hex, so it is safe inside a URL.
  - I compared values without printing them. The real DB_PASSWORD and JWT_SECRET from `~/.albums-k8s.env`, and the JWT_SECRET from the Compose `.env`, appear in no tracked file.
  - Evidence files write `<dev password>` and show only the first line of the private key.
  - The CI JWT regex finds nothing in docs, README.md or prompts.txt.
- **CI will pass.** I ran the static guards locally and all pass:
  - `gofmt -l .` is clean, including the nested Lambda module.
  - The ASCII-only check passes, and `ErrNotFound =` appears exactly once.
  - There is no `text/template`, `ParseGlob` or `AutoMigrate`, and no `.go` file at the repo root.
  - The JWT guard finds nothing, and the four new required files exist.
  - The nested module has its own go.mod, so the root `go vet`, `go build` and `go mod tidy` over `./...` skip it.
  - In the Lambda module, `go -C deploy/floci/lambda test -count=1 ./...` passes and `go vet` is clean.
- **The Vagrant port-probe patch is correct and correctly scoped.**
  - Vagrant 2.4.9's `IsPortOpen` uses `extend IsPortOpen`. Both callers (`handle_forwarded_port_collisions.rb:263` and `:278`) call it as `Vagrant::Util::IsPortOpen.is_port_open?`, so they reach the redefined method. So does the class's `include`.
  - I re-ran the test with Vagrant's Ruby 3.3.8. Without the fix: "closed port 9 -> in use (WRONG)", exit 1. With the fix: "free (right)" and "listening -> in use (right)", exit 0. The test proves both directions.
  - Its list of rescued errors contains everything the stock method rescues.
- **The manifests are well designed.**
  - PostgreSQL is a StatefulSet behind a headless Service with a matching `serviceName`; PGDATA is a subdirectory; storage is a local-path PVC.
  - The Ingress routes by host, which is needed because the templates use root-absolute paths (`/static`, `/catalog`, `/login`, `/web/albums`).
  - `runAsUser` is numeric, so the kubelet can check `runAsNonRoot`.
  - The Job is kept out of kustomize, which is right because a Job's template is immutable.
  - The executor added checks beyond the plan: data survives deleting postgres-0 (15 albums before and after), the Ingress returns 404 without a Host header, and deleted API pods come back.
- **Provisioning is sound.**
  - The provisioners are idempotent.
  - The KUBECONFIG ruling is right: k3s's kubectl prefers the root-only `/etc/rancher/k3s/k3s.yaml` when it exists. That file stays 600 root, and the vagrant user's copy is 600 vagrant (verified).
- **The README holds up.**
  - Versions match the VM: aws-cli 2.38.2, awscli-local 0.22.2, k3s v1.36.5+k3s1, Docker 29.9.1.
  - Both smoke files show 102 PASS and 0 FAIL.
  - The deck errata match the slide text.
  - All 60 relative links resolve.
  - The anchor `#lesson-10--додаткова-частина-devops-basics-vm-docker-floci-k3s` equals the GitHub slug (I computed it).
  - The tables are well formed, including the escaped `\|` inside a code span.
  - Every VirtualBox listener on the Mac is bound to 127.0.0.1, so the "loopback only" claim is true.
- **Commit hygiene is clean.** All 14 are Conventional Commits with no Co-Authored-By or other trailer, by one author. `main` (ab22431) can fast-forward, and the branch is not pushed.

### Issues

#### Critical (Must Fix)
None.

#### Important (Should Fix)
**I1. The two Compose screenshots are byte-for-byte copies of the Lesson 10 screenshots, and the planned `compose-ps.png` was dropped without a ruling.**
- **Where:** `docs/screenshots/devops/devops-compose-catalog.png` and `devops-compose-login.png`, cited at README.md:708 and README.md:844-845. The missing screenshot is plan Task 4 Step 5.
- **The duplication:**
  - `devops-compose-catalog.png` has the same sha256 as `docs/screenshots/catalog.png` (77243ea8…).
  - `devops-compose-login.png` has the same sha256 as `docs/screenshots/login.png` (712026c3…).
  - `cmp` confirms both pairs are identical.
  - README.md:61 describes the originals as Lesson 10 renders of the binary started from an empty directory.
- **Why it matters:**
  - These are the only screenshots the report gives for the graded +30 part.
  - They are headless renders with no address bar, so nothing in them shows Compose or the VM. A grader who compares files sees the previous lesson's images again.
  - The executor's own Task 9 ruling dropped `k8s-catalog.png` because it was byte-identical to `devops-compose-catalog.png`. The same check, taken one step further, catches these two.
  - The only screenshot that does show the Compose stack is `environment.png` (docker ps lists `app-api-1` and `app-postgres-1 (healthy)`).
- **Fix:**
  - Take the planned `compose-ps.png`: a terminal with `vagrant ssh -c 'docker compose -f /app/docker-compose.yml ps'` and `curl -s localhost:8080/health` run on the Mac.
  - Or take a browser capture that shows `localhost:8080` in the address bar, ideally logged in as admin, like `k8s-browser.png`.
  - Point README.md:708 at the new screenshot and at `environment.png`, then drop or relabel the duplicate pair.

#### Minor (Nice to Have)
**M1. The homework file gives a screenshot path that does not exist.**
- `albums_lesson10_devops_homework.md:14` (Ukrainian half) and `:61` (English half) say the screenshots are `docs/screenshots/devops-*.png`.
- The Task 4 ruling moved them into `docs/screenshots/devops/`, and no `devops-*.png` file exists. Fix both halves.

**M2. The README says more is pinned than the code pins.**
- README.md:767 says "`latest` ніде не використовується" (latest is not used anywhere).
- README.md:725 lists awslocal "0.22.2" next to the pinned AWS CLI, as if both were pinned.
- In the code:
  - `deploy/vagrant/aws-tools.sh:27` installs whatever awscli-local is current; only the AWS CLI is pinned (:7).
  - `deploy/vagrant/docker-engine.sh:26` installs the current docker-ce.
  - `Vagrantfile:39` pulls the moving `provided:al2023` tag on every provision. So README.md:718, "повторний vagrant provision нічого не змінює" (a repeat `vagrant provision` changes nothing), is not strictly true.
  - `docs/devops/compose.txt:13` shows the Compose-built image as `app-api:latest`.
- **Fix:** use `pipx install awscli-local==0.22.2`, and either reword those sentences or pin the rest.

**M3. The README's reproduction steps start the bootstrap Job without waiting for the database (README.md:751-753).**
- The Job's own header (`deploy/k8s/bootstrap-job.yaml:2`) says "Applied after the API is up", and the verified run in `k8s.txt` waited for both rollouts first.
- The Job has no wait-for-postgres initContainer, and `repository.Open` gives up after 10 tries, 1 s apart (`internal/repository/db.go:27-28`). On a first run, k3s must still pull `postgres:16-alpine` and initialise the database.
- **What the reader sees:**
  - Failed Job pods.
  - On a slow network the Job can use up `backoffLimit: 4`. A second `kubectl apply` then says "unchanged", and admin login fails with no hint.
  - The `open …/catalog` lines can also reach pods that are not ready yet.
- **Fix:** copy the two `kubectl -n albums rollout status … --timeout=300s` lines from k8s.txt into the block, optionally with `kubectl -n albums wait --for=condition=complete job/albums-bootstrap`. Or give the Job the same initContainer the API has.

**M4. The inline `floci` provisioner does not stop on errors (Vagrantfile:37-40).**
- If `docker compose … up -d` fails (for example a Docker Hub rate limit) and the next `docker pull` succeeds, the script exits 0.
- `vagrant up` then reports success with no Floci running.
- **Fix:** start the heredoc with `set -euo pipefail`, or move it into `deploy/vagrant/floci.sh` like the other three provisioners.

**M5. The port-probe patch has no version guard (deploy/vagrant/port_check_fix.rb:11-24).**
- It applies on every host and every Vagrant version, and it stays after an upstream fix.
- If a future Vagrant changes the method's signature, `vagrant up` fails with an error far from its cause.
- **Fix:** apply it only when `Vagrant::VERSION == "2.4.9"` (or only on macOS), and warn once otherwise. On 2.4.9 its behaviour is verified correct.

### Declined to judge
1. PostgreSQL starts as root with no securityContext or fsGroup and has no livenessProbe. The initContainer and the Job have no resource requests or limits, and no pod sets a seccompProfile. Reason: the plan's manifest values don't ask for these, and they change nothing in a single-node lab.
2. Editing the ConfigMap or a Secret does not restart the pods (no configMapGenerator hash). Reason: standard kubectl behaviour, and the plan specifies plain resources.
3. `kubectl apply` stores a copy of each Secret's data in the `last-applied-configuration` annotation; verified on all three Secrets. Reason: standard behaviour; anyone who can read the annotation can already read the Secret.
4. Secret values are passed as `--from-literal` arguments, so they are briefly visible in `ps` inside the VM. Reason: single-user lab VM.
5. `create-secrets.sh` finds its env file through `$HOME`, so running it with sudo would create a second file and a new DB password. Reason: the script header says to run it as the VM user.
6. The setup reproduces only on Apple Silicon (`box_architecture = "arm64"`). Reason: documented plan decision ("arm64 everywhere").
7. `k3s.sh:10` uses `grep -qF` under `pipefail`. `k3s --version` prints two lines, so in theory `grep` can exit first and the check can fail falsely. Reason: 0 failures in 300 runs in the VM, and the worst case only re-runs the installer.
8. The Docker build context includes about 5 MB of screenshots. Reason: performance only, and `docs/` also holds the swag Go package, so it cannot be excluded as a whole.
9. `demo2-lambda.sh` does not notice a `function.zip` older than `main.go`, and log streams and terminated instances accumulate across demo re-runs. Reason: cosmetic, and the script header says to build first.
10. CI does not run the Lambda tests or check the manifests or shell scripts. Reason: the plan deliberately keeps the nested module outside CI.
11. The root cause of the macOS 27 probe bug is not pinned down: OS behaviour or Ruby's non-blocking connect handling (`nc -G` reports "refused" correctly in vm.txt). Reason: the workaround's behaviour is verified either way.
12. The `get.k3s.io` and AWS `install.sh` installer scripts are fetched unpinned and run as root, and Floci is given the Docker socket. Reason: this is the deck's own method, and the installed versions are pinned.
13. VirtualBox remote display (VRDE) is switched on by the bento box. Reason: it listens only on 127.0.0.1 (verified), and it is not part of the diff.
14. Task 10 Steps 4–7 (triage table, push, link to the green CI run, submission text). Reason: pending by design.
15. Reading "Flossy" as Floci and "applications 1 and 2" as Demo 1 and Demo 2. Reason: decided with the user in the plan.

I agree with every ruling in the ledger, but two are incomplete:
- The Task 9 ruling dropped the k8s screenshots for being byte-identical. The same test catches the Compose pair (I1).
- The Task 4 ruling moved the screenshots into `docs/screenshots/devops/`, but the homework file was not updated (M1).

Plan Task 4 Step 5 (`compose-ps.png`) has no ruling at all.

### Recommendations
- Fix I1 before submitting. M1–M3 are five-minute edits worth putting in the same commit.
- Run the plan's optional Task 10 Step 5 once: `vagrant destroy -f && vagrant up`. Each provisioner was verified separately with `--provision-with`, but the single `vagrant up` that README.md:732 promises has never been run from start to finish.
- Optional CI additions:
  - `docker compose -f deploy/floci/compose.floci.yaml config -q`
  - `bash -n` over `deploy/**/*.sh`
  - `go -C deploy/floci/lambda test ./...`, with setup-go reading `deploy/floci/lambda/go.mod`
- Optional: use `kubectl apply --server-side` in `create-secrets.sh` so no copy of the Secret data lands in an annotation.

### Assessment
**Ready to merge?** With fixes

**Reasoning:** Every graded item is met with real, consistent evidence. The CI guards pass, no secret leaks, and the running cluster matches the committed files. The one Important item is the +30 screenshot evidence: two duplicates of Lesson 10 images, and the planned `compose-ps.png` is missing. Fix it before submitting; the rest are minor accuracy and robustness edits.
