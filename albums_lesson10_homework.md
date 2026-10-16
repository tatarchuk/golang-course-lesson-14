# Домашнє завдання: Тема 10 — Full-Stack Go з HTMX та go:embed

## Наскрізний проєкт: Albums API (система обліку музичних альбомів, варіант `albums`)

> Умову адаптовано з курсового проєкту LibraTrack (варіант `books`). `Book` → `Album`, тому каталог
> книг стає каталогом альбомів: `book_row.html` → `album_row.html`, `#book-list` → `#album-list`,
> `/web/books` → `/web/albums`, «Delete this book?» → «Delete this album?»; ролі — ті, що вже є в
> таблиці `users` (`member` = reader, `staff` = librarian, `admin`); бінарник `libratrack` → `bin/api`
> (так його збирають CI і `.gitignore`); `design_v10.txt` → розділ «Lesson 10 Report» у `README.md`
> (як `security_design` у Темі 9); маршрутизатор лишається `net/http`. Вебінтерфейс аутентифікується
> HttpOnly-cookie, яку ставить `POST /login` (у роздатковому матеріалі немає сторінки входу — її
> додано), а REST API і далі приймає лише `Authorization: Bearer`. Сторінка каталогу публічна, як і
> `GET /api/v1/albums`; кнопки видалення бачать лише `staff` та `admin`. Коди помилок — як у Темі 9:
> анонімний запит → `401`, недостатня роль → `403` (у роздатковому матеріалі для обох випадків
> написано `403`). У локальній копії умови замість «Delete Row (Librarian)» стояло «Delete Row (Artist)»;
> `artist` — сутність каталогу, а не роль, тож видаляти рядки можуть `staff` і `admin`, як і в
> `DELETE /api/v1/albums/{id}`. Тестів разом із завданням цього разу не роздавали: приймальні тести
> `internal/app/web_test.go` пишуться першими на наявному каркасі `internal/app` і будуть червоними,
> доки завдання не виконано; `internal/handler/web/catalog_test.go` генерує агент у завданні 3.2.
> Контракт, який фіксують тести, описано в останньому розділі.

**Часовий ліміт:** до 2 годин.

**Мета роботи:** перетворити захищений REST API **Albums API** на повноцінний вебзастосунок: сторінки на `html/template`, динамічна інтерактивність через HTMX без фронтенд-фреймворків на JavaScript, і пакування всього застосунку в один переносний бінарник через `go:embed` з перевіркою його незалежності в ізольованому середовищі.

---

### ЧАСТИНА 1: Ручні завдання (архітектура, безпека та перевірка ізоляції) [~35 хв]

*Завдання, що потребують людських проєктних рішень, перевірки безпеки автоекранування та перевірки розгортання бінарника.*

* **Завдання 1.1: Архітектура вебшару та аудит автоекранування (лише людина):**
  1. У розділі «Lesson 10 Report» файлу `README.md` (у завданні — `design_v10.txt`) опишіть архітектурний перехід на HttpOnly-cookie для зберігання JWT. Поясніть, чому зберігання JWT у `localStorage` браузера створює ризик XSS для застосунків із серверним рендерингом (скрипт, що потрапив на сторінку, читає `localStorage`, але не може прочитати HttpOnly-cookie). Опишіть захист cookie від CSRF: атрибут `SameSite=Lax` і `http.CrossOriginProtection` зі стандартної бібліотеки Go 1.25 для небезпечних методів; прапорець `Secure` вмикається змінною `COOKIE_SECURE` (за TLS-проксі). Зафіксуйте, що API лишається на Bearer-токенах і cookie не приймає, а веб-маршрути не приймають заголовок `Authorization`.
  2. Проведіть ручний аудит усіх файлів шаблонів: підтвердьте, що використовується виключно `html/template`, а не `text/template` (`grep -rn '"text/template"' --include='*.go' .` має нічого не знайти).
  3. Переконайтеся, що `template.HTML` не застосовується до недовірених даних користувача чи бази (`grep -rn 'template.HTML' internal web`). Задокументуйте результати аудиту у звіті та в `docs/web/template-audit.txt`.

* **Завдання 1.2: Перевірка ізоляції єдиного бінарника:**
  1. Зберіть статично злінкований бінарник: `CGO_ENABLED=0 go build -o bin/api ./cmd/api` (у завданні — `-o libratrack`; `make build` робить те саме з `-trimpath -ldflags='-s -w'`).
  2. Створіть чисту тимчасову теку (наприклад, `/tmp/verify_albums`) і скопіюйте в неї **лише** виконуваний файл `api`.
  3. Запустіть бінарник із цієї теки (де немає ні `./web`, ні `./templates`, ні `./migrations`), передавши лише `DATABASE_URL` і `JWT_SECRET` — без `MIGRATIONS_PATH`: міграції теж мають бути вбудовані, бо в цьому проєкті вони — єдина дискова залежність бінарника. Переконайтеся в браузері, що `/catalog`, `/static/app.css` та `/static/htmx.min.js` завантажуються. Збережіть вивід у `docs/web/isolation.txt`, а скриншоти — у `docs/screenshots/`.

---

### ЧАСТИНА 2: Промпт-інжиніринг (шаблони, партіали та HTMX) [~40 хв]

*Складання точних, детальних промптів для генерації макета, партіалів та HTMX-атрибутів.*

* **Завдання 2.1: Промпт для макета і сторінки каталогу (`layout.html` + `catalog.html` + `album_row.html`):**
  * Напишіть промпт, який генерує макет `web/templates/layout.html` із `{{block "content" .}}` (і `{{block "title" .}}`), що підключає `/static/app.css` та `/static/htmx.min.js`; сторінку `web/templates/catalog.html`; партіали `web/templates/partials/album_row.html` (один `<tr>`) та `partials/album_rows.html` (рядки або один порожній рядок `<tr class="empty">`, коли нічого не знайдено); сторінку входу `web/templates/login.html`. Кожна сторінка — окремий набір шаблонів (клон макета + файл сторінки), інакше останній розібраний `content` перекриє решту.
  * *Вимоги:* сторінка каталогу показує таблицю альбомів (назва, виконавці, жанр, рік, лейбл) з індикатором наявності «доступно/усього примірників» і форму пошуку з полем `name="q"`; значення `q` повертається в поле після серверної фільтрації (`GET /catalog?q=`), щоб сторінка працювала і без JavaScript. У шапці — e-mail і роль залогіненого користувача та кнопка виходу або посилання на `/login`.
  * *Результат:* збережіть промпт у `prompts.txt`, а згенеровані шаблони покладіть у `web/templates/`. HTMX (`web/static/htmx.min.js`) — зафіксована версія 2.x як статичний файл, не Go-залежність (CI вимагає, щоб `go mod tidy` нічого не змінював); версію і sha256 запишіть у `docs/web/htmx.sha256`.

* **Завдання 2.2: Промпт для інтерактивності HTMX (живий пошук і видалення рядка):**
  * Складіть промпт, який генерує HTMX-атрибути та відповідні обробники в `internal/handler/web/`:
    1. **Живий пошук:** поле вводу з `hx-get="/web/albums"`, `hx-trigger="keyup changed delay:300ms"`, `hx-target="#album-list"`. Обробник `GET /web/albums?q=` із заголовком `HX-Request: true` відповідає лише фрагментом рядків (`album_rows`); без заголовка — `303` на `/catalog?q=`. Пошук — за назвою альбому або ім'ям виконавця без урахування регістру (`ILIKE`) через нове поле `Query` у `repository.ListOptions`; наявність примірників для всіх рядків сторінки читається одним запитом (`AvailabilityForAlbums`), а не по одному на альбом (пам'ятайте про N+1 з Теми 8).
    2. **Видалення рядка (`staff`, `admin`):** кнопка в `album_row.html` з `hx-delete="/web/albums/{{.ID}}"`, `hx-target="closest tr"`, `hx-swap="outerHTML"`, `hx-confirm="Delete this album?"`; рендериться лише для `staff` та `admin`.
    3. **Cookie-аутентифікація:** middleware `CookieAuth` читає cookie `albums_token`, перевіряє JWT наявним `auth.ParseToken` і кладе claims у контекст через `auth.WithUser`; невалідна або відсутня cookie означає анонімний запит, а `401`/`403` дає наявний `RequireRole`. Обробники `GET /login` (форма), `POST /login` (form-encoded `email`, `password`; перевірка через спільну з API функцію, однакова відповідь для невідомого e-mail і неправильного пароля; `303` на `/catalog` і `Set-Cookie: albums_token=<jwt>; Path=/; Max-Age=86400; HttpOnly; SameSite=Lax`) та `POST /logout` (прострочує cookie). Веб-група маршрутів обгорнута `http.CrossOriginProtection`.
  * *Результат:* обробник видалення відповідає порожнім `200 OK` (не `204`: HTMX не виконує swap на `204`), щоб HTMX прибрав цільовий `<tr>`. Збережіть промпт у `prompts.txt`.

---

### ЧАСТИНА 3: Агентне використання AI (інтеграція go:embed та наскрізне тестування) [~45 хв]

*Використання AI-агента для інтеграції віртуальних файлових систем і написання HTML-інтеграційних тестів.*

* **Завдання 3.1: `go:embed` для шаблонів, статики та міграцій:**
  * Доручіть агентові застосувати директиву `//go:embed` у `web/templates.go` та `web/static.go`:
    * використати `embed.FS` і `template.ParseFS` замість дискового `ParseGlob`;
    * використати `fs.Sub(staticFS, "static")`, щоб прибрати префікс шляху під час роздачі статики (`http.StripPrefix("/static/", http.FileServer(http.FS(...)))`).
  * Додатково для цього проєкту: `migrations/embed.go` з `//go:embed *.sql` і `repository.MigrateFS(dsn, fsys)`; `cmd/api/main.go` та `cmd/bootstrap/main.go` використовують вбудовані міграції, а `MIGRATIONS_PATH` стає необов'язковим перевизначенням; `Dockerfile` і `docker-compose.yml` більше не копіюють `migrations/`.
  * *Виконання:* підключіть вбудовані файлові системи в `cmd/api/main.go` через `app.Config` (`Templates`, `Static`, `CookieSecure`), яку `main` передає в `app.NewRouter`.

* **Завдання 3.2: HTML/HTMX-тести обробників, згенеровані AI:**
  * Доручіть агентові написати табличні інтеграційні тести в `internal/handler/web/catalog_test.go` на `httptest` з фейковими репозиторіями (без БД, за зразком `internal/handler/fakes_test.go`).
  * *Тестові кейси:*
    1. `GET /catalog` повертає `HTTP 200` з HTML, що містить розмітку макета і таблицю альбомів.
    2. HTMX-запит `GET /web/albums?q=blue` із заголовком `HX-Request: true` повертає лише відрендерені фрагменти `album_row`.
    3. Спроба `hx-delete` без cookie отримує `HTTP 401 Unauthenticated`, із cookie ролі `member` — `HTTP 403 Forbidden`.
  * *Виконання:* приймальні тести `internal/app/web_test.go` (справжній роутер і PostgreSQL) запускаються до початку роботи (`go test ./internal/app/ -run Web -v`) і падають на `404`; після реалізації `go test ./... -race -count=1` із `TEST_DATABASE_URL` зелений. Розширюйте таблиці, якщо ваш дизайн додає поведінку.

---

### Вимоги до здачі домашнього завдання (Deliverables)

Надайте посилання на репозиторій, у якому є:

1. **Інтерактивність UI:** каталог із живим пошуком і видаленням рядків без перезавантаження сторінки.
2. **Перевірка єдиного бінарника:** бінарник проходить тест ізоляції без зовнішніх дискових залежностей (ні `web/`, ні `migrations/`).
3. **Стандарти безпеки:** JWT для вебінтерфейсу — лише в HttpOnly-cookie з `SameSite`; контекстне автоекранування не порушено; API і далі працює на Bearer-токенах, усі тести Тем 7–9 зелені.
4. **Покриття тестами:** інтеграційні тести перевіряють рендеринг HTML-фрагментів і RBAC на HTMX-ендпоінтах; CI зелений.
5. **Журнал промптів `prompts.txt`:** промпти завдань 2.1 і 2.2 (а також інструкції агентові для 3.1 і 3.2, як у Темі 9).
6. **`README.md`** з розділом «Lesson 10 Report»: архітектура cookie, аудит шаблонів, тест ізоляції, посилання на зелений CI.
7. **Докази під `docs/web/`:** `tests-before.txt`, `tests-after.txt`, `isolation.txt`, `template-audit.txt`, `cookie-flow.txt` (значення cookie скорочено, як токени в Темі 9), `smoke.txt`; скриншоти в `docs/screenshots/`.
8. **CI та smoke:** workflow збирає статичний бінарник, запускає його з порожньої теки і перевіряє веб-маршрути; `scripts/smoke.sh` робить те саме локально.

---

### Що перевіряють тести (`internal/app/web_test.go`, `internal/handler/web/catalog_test.go`)

- **`GET /catalog`** → `200`, `Content-Type: text/html; charset=utf-8`; тіло містить `<!doctype html>`, `<table`, `id="album-list"`, `<input name="q"` з трьома HTMX-атрибутами, `<script src="/static/htmx.min.js"` та `<link href="/static/app.css"`; назви альбомів екрануються (назва `<b>x</b>` виводиться як `&lt;b&gt;`); `?q=` фільтрує на сервері і повертається в поле вводу.
- **`GET /web/albums?q=<текст>`** із `HX-Request: true` → `200` і лише елементи `<tr>` (без `<html`, `<table`); збіг за назвою або ім'ям виконавця без урахування регістру; нічого не знайдено → один `<tr class="empty">`; без заголовка → `303` на `/catalog?q=<текст>`.
- **Кнопка видалення** є лише на сторінках, відрендерених для `staff`/`admin`; сторінки анонімного користувача і `member` не містять `hx-delete`.
- **`DELETE /web/albums/{id}`:** лише cookie (заголовок `Authorization: Bearer` ігнорується → `401`); анонімний → `401` `unauthenticated`; `member` → `403` `forbidden`; `staff`/`admin` → `200` з порожнім тілом, альбом і його примірники зникли; невідомий id → `404`; нечисловий id → `400`; `Sec-Fetch-Site: cross-site` → `403`. Помилки — у звичному конверті `{"error":{"code":"...","message":"..."}}`.
- **`GET /login`** → `200` із формою (`method="post" action="/login"`, поля `email`, `password`). **`POST /login`** (form-encoded): успіх → `303` на `/catalog` і `Set-Cookie: albums_token=<jwt>; Path=/; Max-Age=86400; HttpOnly; SameSite=Lax` (`Secure`, коли `COOKIE_SECURE=true`); невідомий e-mail і неправильний пароль → `401` з однаковим тілом і повідомленням `invalid credentials`. **`POST /logout`** → `303`, cookie прострочена.
- **Статика:** `GET /static/app.css` → `200` `text/css`; `GET /static/htmx.min.js` → `200` з JavaScript-типом; `GET /static/missing.css` → `404`.
- **`GET /`** лишається JSON `404`; жоден тест `/api/v1` Тем 7–9 не змінюється.
- **Бінарник:** зібраний із `CGO_ENABLED=0`, стартує з порожньої теки лише з `DATABASE_URL` і `JWT_SECRET`; `MIGRATIONS_PATH` — необов'язкове перевизначення.
- **Шаблони:** імпортується лише `html/template`; `template.HTML` не використовується.

---

# Homework Assignment: Lesson 10 — Full-Stack Go with HTMX and go:embed

## Capstone Project: Albums API (music album catalogue, the `albums` variant)

> Adapted from the LibraTrack assignment (the `books` variant). `Book` → `Album`, so the book
> catalogue becomes the album catalogue: `book_row.html` → `album_row.html`, `#book-list` →
> `#album-list`, `/web/books` → `/web/albums`, "Delete this book?" → "Delete this album?"; the roles
> are the ones the `users` table already has (`member` = reader, `staff` = librarian, `admin`); the
> `libratrack` binary → `bin/api` (what CI and `.gitignore` already use); `design_v10.txt` → the
> "Lesson 10 Report" section of `README.md` (like `security_design` in Lesson 9); the router stays
> `net/http`. The web UI authenticates with an HttpOnly cookie set by `POST /login` (the handout has
> no login page; it is added here), while the REST API keeps accepting `Authorization: Bearer` only.
> The catalogue page is public, like `GET /api/v1/albums`; only `staff` and `admin` see the delete
> buttons. Error codes follow Lesson 9: an anonymous request → `401`, an insufficient role → `403`
> (the handout says `403` for both). The local copy of the handout read "Delete Row (Artist)" where
> the original says "Delete Row (Librarian)"; `artist` is a catalogue entity, not a role, so rows are
> deleted by `staff` and `admin`, exactly like `DELETE /api/v1/albums/{id}`. No tests were shipped
> with the assignment this time: the acceptance tests `internal/app/web_test.go` are written first
> on the existing `internal/app` harness and stay red until the work is done;
> `internal/handler/web/catalog_test.go` is generated by the agent in Task 3.2. The contract the
> tests pin down is in the last section.

**Time Budget:** Up to 2 hours.

**Objective:** Transform the secured **Albums API** into a fully operational web application using HTML templates, introduce dynamic HTMX interactivity without frontend JavaScript frameworks, and package the entire application into a single portable binary using `go:embed`, verifying its independence in an isolated environment.

---

### PART 1: Manual Tasks (Architecture, Security & Isolation Verification) [~35 mins]

*Tasks requiring critical human design choices, autoescaping security verification, and binary deployment checks.*

* **Task 1.1: Web Layer Architecture & Autoescaping Audit (Human-only):**
  1. In the "Lesson 10 Report" section of `README.md` (the assignment's `design_v10.txt`), detail the architectural move to HttpOnly cookies for JWT storage. Explain why storing JWTs in browser `localStorage` introduces XSS risks for server-rendered apps (a script injected into the page reads `localStorage`, but cannot read an HttpOnly cookie). Describe how the cookie is protected against CSRF: the `SameSite=Lax` attribute plus `http.CrossOriginProtection` from the Go 1.25 standard library for unsafe methods; the `Secure` flag is switched on with `COOKIE_SECURE` (behind a TLS proxy). Record that the API stays on bearer tokens and accepts no cookie, and that the web routes accept no `Authorization` header.
  2. Perform a manual audit of all template files: confirm `html/template` is used exclusively over `text/template` (`grep -rn '"text/template"' --include='*.go' .` must find nothing).
  3. Verify that `template.HTML` is not used inappropriately on untrusted user or database inputs (`grep -rn 'template.HTML' internal web`). Document your audit findings in the report and in `docs/web/template-audit.txt`.

* **Task 1.2: Single Binary Isolation Verification Test:**
  1. Compile the application into a statically linked binary: `CGO_ENABLED=0 go build -o bin/api ./cmd/api` (the assignment says `-o libratrack`; `make build` does the same with `-trimpath -ldflags='-s -w'`).
  2. Create a clean temporary folder (e.g. `/tmp/verify_albums`) and copy **only** the `api` executable into it.
  3. Execute the binary from this directory (where no `./web`, `./templates` or `./migrations` folders exist) with nothing but `DATABASE_URL` and `JWT_SECRET` in the environment, without `MIGRATIONS_PATH`: the migrations must be embedded too, because in this project they are the binary's only disk dependency. Verify via browser that `/catalog`, `/static/app.css` and `/static/htmx.min.js` load. Save the output to `docs/web/isolation.txt` and the screenshots to `docs/screenshots/`.

---

### PART 2: Prompt Engineering (Templates, Partials & HTMX Wiring) [~40 mins]

*Crafting precise, detailed prompts to generate UI layouts, partials, and HTMX attributes.*

* **Task 2.1: Prompting Layout and Catalog Page (`layout.html` + `catalog.html` + `album_row.html`):**
  * Write a prompt to generate the layout `web/templates/layout.html` using `{{block "content" .}}` (and `{{block "title" .}}`) and loading `/static/app.css` and `/static/htmx.min.js`; the page `web/templates/catalog.html`; the partials `web/templates/partials/album_row.html` (one `<tr>`) and `partials/album_rows.html` (the rows, or one empty-state row `<tr class="empty">` when nothing matches); and the login page `web/templates/login.html`. Each page is its own template set (a clone of the layout plus the page file), otherwise the last parsed `content` wins.
  * *Requirements:* the catalogue page must display a table of albums (title, artists, genre, year, label) with an availability indicator "available/total copies" and a search form with an input `name="q"`; the `q` value is echoed back into the input after server-side filtering (`GET /catalog?q=`), so the page works without JavaScript too. The header shows the e-mail and role of the logged-in user with a logout button, or a link to `/login`.
  * *Deliverable:* save the prompt to `prompts.txt` and place the generated templates in `web/templates/`. HTMX (`web/static/htmx.min.js`) is a pinned 2.x release kept as a static file, not a Go dependency (CI requires `go mod tidy` to change nothing); record the version and sha256 in `docs/web/htmx.sha256`.

* **Task 2.2: Prompting HTMX Interactivity (Live Search & Item Deletion):**
  * Construct a prompt to generate HTMX attributes and corresponding handlers in `internal/handler/web/`:
    1. **Live Search:** input element with `hx-get="/web/albums"`, `hx-trigger="keyup changed delay:300ms"`, `hx-target="#album-list"`. The `GET /web/albums?q=` handler answers with the rows fragment only (`album_rows`) when the `HX-Request: true` header is present, and with a `303` to `/catalog?q=` otherwise. The search matches the album title or an artist name case-insensitively (`ILIKE`) through a new `Query` field of `repository.ListOptions`; the availability of every row on the page comes from one query (`AvailabilityForAlbums`), not one per album (remember the N+1 of Lesson 8).
    2. **Delete Row (`staff`, `admin`):** delete button in `album_row.html` with `hx-delete="/web/albums/{{.ID}}"`, `hx-target="closest tr"`, `hx-swap="outerHTML"`, `hx-confirm="Delete this album?"`; rendered only for `staff` and `admin`.
    3. **Cookie authentication:** a `CookieAuth` middleware reads the `albums_token` cookie, verifies the JWT with the existing `auth.ParseToken` and stores the claims in the context with `auth.WithUser`; a missing or invalid cookie means an anonymous request, and the existing `RequireRole` produces the `401`/`403`. Handlers `GET /login` (form), `POST /login` (form-encoded `email`, `password`; the credential check is shared with the API, with the same response for an unknown e-mail and a wrong password; `303` to `/catalog` and `Set-Cookie: albums_token=<jwt>; Path=/; Max-Age=86400; HttpOnly; SameSite=Lax`) and `POST /logout` (expires the cookie). The web route group is wrapped in `http.CrossOriginProtection`.
  * *Deliverable:* the delete handler must respond with an empty `200 OK` (not `204`: htmx does not swap on `204`) to remove the targeted `<tr>`. Save the prompt in `prompts.txt`.

---

### PART 3: Agentic AI Use (go:embed Integration & End-to-End Testing) [~45 mins]

*Leveraging an AI coding agent to integrate virtual filesystems and build HTML integration tests.*

* **Task 3.1: Implementing `go:embed` for Templates, Static Assets and Migrations:**
  * Instruct your AI agent to apply the `//go:embed` directive in `web/templates.go` and `web/static.go`:
    * utilise `embed.FS` and `template.ParseFS` instead of disk-based `ParseGlob`;
    * use `fs.Sub(staticFS, "static")` to strip path prefixes when serving static assets (`http.StripPrefix("/static/", http.FileServer(http.FS(...)))`).
  * Specific to this project: `migrations/embed.go` with `//go:embed *.sql` and `repository.MigrateFS(dsn, fsys)`; `cmd/api/main.go` and `cmd/bootstrap/main.go` use the embedded migrations and `MIGRATIONS_PATH` becomes an optional override; `Dockerfile` and `docker-compose.yml` no longer copy `migrations/`.
  * *Execution:* wire the embedded filesystems in `cmd/api/main.go` through `app.Config` (`Templates`, `Static`, `CookieSecure`), which `main` passes to `app.NewRouter`.

* **Task 3.2: AI-Generated HTML/HTMX Handler Tests:**
  * Direct your AI agent to write table-driven integration tests in `internal/handler/web/catalog_test.go` using `httptest` and fake repositories (no database, modelled on `internal/handler/fakes_test.go`).
  * *Test Cases:*
    1. `GET /catalog` returns `HTTP 200` with HTML containing layout markup and the album table.
    2. HTMX request `GET /web/albums?q=blue` with the `HX-Request: true` header returns only rendered `album_row` HTML fragments.
    3. An `hx-delete` attempt without a cookie receives `HTTP 401 Unauthenticated`; with a `member` cookie it receives `HTTP 403 Forbidden`.
  * *Execution:* the acceptance tests `internal/app/web_test.go` (real router and PostgreSQL) run before the work starts (`go test ./internal/app/ -run Web -v`) and fail on `404`; after the implementation `go test ./... -race -count=1` with `TEST_DATABASE_URL` set is green. Extend the tables if your design adds behaviour.

---

### Evaluation Criteria & Deliverables (Self-Check)

Submit a link to the repository containing:

1. **UI Interactivity:** a browsable catalogue with live search and row deletion working without full page reloads.
2. **Single Binary Verification:** the binary passes the isolation test without external disk dependencies (no `web/`, no `migrations/`).
3. **Security Standards:** the web UI's JWT lives only in an HttpOnly cookie with `SameSite`; contextual autoescaping remains uncompromised; the API keeps working on bearer tokens and every test of Lessons 7-9 stays green.
4. **Test Coverage:** integration tests validate HTML fragment rendering and RBAC enforcement on the HTMX endpoints; CI is green.
5. **Prompt Log `prompts.txt`:** the prompts of Tasks 2.1 and 2.2 (plus the agent instructions of 3.1 and 3.2, as in Lesson 9).
6. **`README.md`** with a "Lesson 10 Report": the cookie architecture, the template audit, the isolation test, a link to the green CI run.
7. **Evidence under `docs/web/`:** `tests-before.txt`, `tests-after.txt`, `isolation.txt`, `template-audit.txt`, `cookie-flow.txt` (cookie values cut short, like the tokens in Lesson 9), `smoke.txt`; screenshots in `docs/screenshots/`.
8. **CI and smoke:** the workflow builds the static binary, starts it from an empty directory and checks the web routes; `scripts/smoke.sh` does the same locally.

---

### What the tests check (`internal/app/web_test.go`, `internal/handler/web/catalog_test.go`)

- **`GET /catalog`** → `200`, `Content-Type: text/html; charset=utf-8`; the body contains `<!doctype html>`, `<table`, `id="album-list"`, `<input name="q"` with the three HTMX attributes, `<script src="/static/htmx.min.js"` and `<link href="/static/app.css"`; album titles are escaped (a title `<b>x</b>` renders as `&lt;b&gt;`); `?q=` filters server-side and is echoed into the input.
- **`GET /web/albums?q=<text>`** with `HX-Request: true` → `200` and only `<tr>` elements (no `<html`, no `<table`); matches on the title or an artist name, case-insensitively; no match → one `<tr class="empty">`; without the header → `303` to `/catalog?q=<text>`.
- **The delete button** exists only in pages rendered for `staff`/`admin`; the pages of an anonymous user and of a `member` contain no `hx-delete`.
- **`DELETE /web/albums/{id}`:** cookie only (an `Authorization: Bearer` header is ignored → `401`); anonymous → `401` `unauthenticated`; `member` → `403` `forbidden`; `staff`/`admin` → `200` with an empty body, the album and its copies gone; unknown id → `404`; non-numeric id → `400`; `Sec-Fetch-Site: cross-site` → `403`. Errors use the usual envelope `{"error":{"code":"...","message":"..."}}`.
- **`GET /login`** → `200` with the form (`method="post" action="/login"`, fields `email`, `password`). **`POST /login`** (form-encoded): success → `303` to `/catalog` and `Set-Cookie: albums_token=<jwt>; Path=/; Max-Age=86400; HttpOnly; SameSite=Lax` (`Secure` when `COOKIE_SECURE=true`); unknown e-mail and wrong password → `401` with an identical body and the message `invalid credentials`. **`POST /logout`** → `303`, the cookie expired.
- **Static files:** `GET /static/app.css` → `200` `text/css`; `GET /static/htmx.min.js` → `200` with a JavaScript type; `GET /static/missing.css` → `404`.
- **`GET /`** stays the JSON `404`; no `/api/v1` test of Lessons 7-9 changes.
- **Binary:** built with `CGO_ENABLED=0`, starts from an empty directory with only `DATABASE_URL` and `JWT_SECRET`; `MIGRATIONS_PATH` is an optional override.
- **Templates:** only `html/template` is imported; `template.HTML` is not used.
