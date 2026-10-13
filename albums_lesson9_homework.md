# Домашнє завдання: Тема 9 — Безпека API (аутентифікація, JWT та RBAC з AI)

## Наскрізний проєкт: Albums API (система обліку музичних альбомів, варіант `albums`)

> Умову адаптовано з курсового проєкту LibraTrack (варіант `books`). `Book` → `Album`, тому замість
> ендпоінтів книг і видач (`books`, `loans`) захищаються операції запису каталогу (`genres`, `artists`,
> `albums`, `copies`); ролі — ті, що вже є в таблиці `users` (`member` = reader, `staff` = librarian,
> `admin`); маршрутизатор лишається `net/http` (`http.ServeMux` з method-патернами та обгортання
> middleware замість груп маршрутів Gin). Контракт, який фіксують адаптовані тести, описано в останньому
> розділі; самі тести вже лежать у репозиторії (`internal/app/auth_test.go`, `internal/app/rbac_test.go`)
> і будуть червоними, доки завдання не виконано.

**Часовий ліміт:** до 2 годин.

**Мета роботи:** захистити REST API **Albums API** відповідно до production-стандартів безпеки. Реалізувати повноцінну аутентифікацію з хешуванням паролів (`bcrypt`) і видачею токенів (`JWT`), додати рольову авторизацію у вигляді middleware (`RBAC`) та провести структурований AI-асистований аудит безпеки кодової бази, щоб знайти й усунути вразливості.

---

### ЧАСТИНА 1: Ручні завдання (архітектура безпеки, моделювання та рев'ю) [~35 хв]

*Цей етап потребує людських проєктних рішень, аналізу ризиків і контролю якості коду.*

* **Завдання 1.1: Ролі, матриця доступу та DTO (проєктування безпеки і контракту):**
  1. Створіть файл `security_design.txt` (або розділ у новому `README.md`).
  2. Визначте ролі. У схемі їх уже три (`CHECK (role IN ('member','staff','admin'))` у `migrations/000001_init_schema.up.sql`): `member` (типова роль нового акаунта, у завданні — `reader`), `staff` (у завданні — `librarian`) та `admin`.
  3. Складіть матрицю доступу для Albums API:
     * **Публічні (без аутентифікації):** `POST /api/v1/auth/register`, `POST /api/v1/auth/login`, усі `GET` каталогу (`/genres`, `/genres/{id}`, `/artists`, `/artists/{id}`, `/albums`, `/albums/{id}`, `/albums/{id}/copies`, `/albums/{id}/availability`), `/health` і `/swagger/`.
     * **Захищені (будь-який аутентифікований користувач):** `GET /api/v1/me` (акаунт, якому належить токен; у варіанті `albums` немає видач, тож цей маршрут заміняє `GET /me/loans`).
     * **Персонал (`staff`, `admin`):** `POST /api/v1/genres`, `POST /api/v1/artists`, `POST /api/v1/albums`, `PUT /api/v1/albums/{id}`, `DELETE /api/v1/albums/{id}`, `POST /api/v1/albums/{id}/copies` (операції запису каталогу заміняють `POST /books`, `PUT /books/:id`, `POST /loans`, `POST /loans/:id/return`).
     * **Адміністратор (`admin`):** `GET /api/v1/users`, `PUT /api/v1/users/{id}/role`.
  4. Спроєктуйте вхідні DTO так, щоб Mass Assignment був неможливий: `RegisterInput` (у завданні — `RegisterRequest`) містить лише `email` і `password` (ніколи — `role`), `RoleInput` — лише `role`, а в моделі `User` поле `PasswordHash` має тег `json:"-"`, щоб хеш ніколи не потрапив у відповідь. Обробники декодують запити у ці вхідні структури, а не в `model.User`. Якщо додасте оновлення профілю, його вхідна структура теж не повинна містити `role` чи `password_hash`.

* **Завдання 1.2: Ручний аудит роботи з паролями та логування:**
  1. Перевірте обробник `/auth/login`: невідомий e-mail і неправильний пароль мають давати однаковий `401` з однаковим тілом (`"invalid credentials"`), щоб ендпоінт не можна було використати для перебору акаунтів (User Enumeration). Викликайте `bcrypt.CompareHashAndPassword` навіть для невідомого e-mail (з фіктивним хешем), щоб і час відповіді нічого не видавав.
  2. Перегляньте `internal/middleware/logging.go` (Тема 7): він логує лише метод, шлях, статус і тривалість. Так і лишіть: жодних тіл запитів і заголовка `Authorization` у лозі; якщо колись додасте логування тіл — маскуйте `password`.
  3. Перевірте переривання ланцюжка middleware: у `net/http` немає `c.AbortWithStatusJSON()`. Помилка аутентифікації чи авторизації має записати відповідь (`handler.WriteError`) і зробити `return` **без** виклику `next.ServeHTTP`, щоб обгорнутий обробник не виконувався.

---

### ЧАСТИНА 2: Промпт-інжиніринг (аутентифікація, JWT та RBAC middleware) [~40 хв]

*Складіть точні, повністю специфіковані промпти для AI-асистента, щоб згенерувати код аутентифікації та авторизації.*

* **Завдання 2.1: Промпт для модуля аутентифікації (bcrypt + видача JWT):**
  * Складіть і виконайте промпт, який генерує пару міграцій `000003_add_auth_fields.up.sql` / `.down.sql` і оновлює модель `User` (`internal/model/user.go`). Колонка `role` уже є, тож міграція додає лише `password_hash`. Seed (`000002_seed_demo_data.up.sql`) уже містить `admin@albums.local`, `staff@albums.local` і `member@albums.local`: задайте їм bcrypt-хеш задокументованого dev-пароля (тоді `admin@albums.local` стане стартовим адміністратором) або додайте колонку з `DEFAULT ''` і створіть першого адміністратора інакше. В обох випадках задокументуйте рішення.
  * *Вимоги до промпту для `Register` і `Login`:*
    1. Реєстрація хешує пароль через `bcrypt.GenerateFromPassword` з `bcrypt.DefaultCost` (10), зберігає роль `member` і відповідає `201` з користувачем (без хеша).
    2. Логін перевіряє пароль через `bcrypt.CompareHashAndPassword` і відповідає `200 {"token": "<jwt>"}` — токеном, підписаним `jwt.SigningMethodHS256` (`github.com/golang-jwt/jwt/v5`).
    3. JWT містить claims `user_id`, `role` та `exp` (24 години). Секрет підпису береться виключно зі змінної середовища `JWT_SECRET` (`os.Getenv`), зчитується один раз на старті в `main.go` (fail fast, якщо порожній) і передається в роутер; не зчитуйте його в package-level змінну під час ініціалізації пакета — тести задають його через `t.Setenv` перед побудовою роутера.
    4. Додайте `JWT_SECRET` до `.env.example` і `docker-compose.yml` та оновіть анотації Swagger (`@securityDefinitions.apikey BearerAuth`, `@Security BearerAuth` на захищених обробниках).
  * *Результат:* збережіть текст промпту в `prompts.txt`, а згенеровані файли інтегруйте в `internal/auth/` (хешування і токени) та `internal/handler/` (`auth_handler.go`, `user_handler.go`), з `UserRepository` в `internal/repository/`.

* **Завдання 2.2: Промпт для ланцюжка middleware (`Auth` та `RequireRole`):**
  * Напишіть промпт, який генерує `Auth` (`internal/middleware/auth.go`) і `RequireRole` (`internal/middleware/rbac.go`) як middleware виду `func(http.Handler) http.Handler` — так само, як наявні `Logging`, `Recover` і `CORS`.
  * *Вимоги до промпту:*
    1. `Auth` дістає токен із заголовка `Authorization: Bearer <token>`, перевіряє підпис (лише HS256; будь-який інший `alg` відхиляється) та claim `exp` і кладе `user_id` і `role` в контекст запиту (`context.WithValue` з неекспортованим типом ключа і типізованими акцесорами на кшталт `auth.UserFromContext(ctx)` — аналог `c.Set` у `net/http`). Помилка: `401` `unauthenticated`.
    2. `RequireRole` — варіативна (`allowed ...string`), читає роль із контексту і відповідає `401` `unauthenticated`, якщо аутентифікованого користувача немає, та `403` `forbidden`, якщо роль не дозволена.
    3. Промпт має показати реєстрацію маршрутів у `internal/app/app.go` — аналог вкладених груп у `net/http`: хелпери на кшталт `protected := middleware.Auth`, `staff := func(h http.Handler) http.Handler { return middleware.Auth(middleware.RequireRole("staff", "admin")(h)) }` та `admin := ...RequireRole("admin")`, застосовані до кожного маршруту (`mux.Handle("POST /api/v1/albums", staff(http.HandlerFunc(albums.Create)))`). `Auth` завжди виконується перед `RequireRole`.
    4. `CORS` має перелічити `Authorization` в `Access-Control-Allow-Headers`, інакше браузери не зможуть надіслати токен.
  * *Результат:* збережіть промпт у `prompts.txt`.

---

### ЧАСТИНА 3: Агентне використання AI (автоматизований аудит безпеки та тестування) [~45 хв]

* **Завдання 3.1: AI-асистований security-рев'ю коду:**
  * Передайте AI-агенту код створених або змінених модулів (`internal/auth`, `internal/middleware`, `internal/handler/auth_handler.go`, `user_handler.go`, `album_handler.go`, `copy_handler.go`, `internal/repository`).
  * *Інструкція для агента:*
    > "Review this Go net/http API code for security vulnerabilities. Check specifically for: 1) SQL injection (raw queries, string concatenation); 2) Mass assignment (binding request JSON directly to DB models); 3) Missing authentication or RBAC checks; 4) Sensitive data exposure (leaking password hashes in JSON responses, secrets or tokens in logs). Output a structured list with line numbers, severity (Low/Medium/High), and proposed fixes. Do not rewrite files completely."
  * *Тріаж і звіт:* критично оцініть результат. Складіть `security_audit.md` щонайменше з **5 знахідками**, кожна — із рівнем серйозності та статусом тріажу: *Fixed Now*, *Tracked for Later* або *False Positive*.

* **Завдання 3.2: Табличні тести безпеки:**
  * Тести з оригінального завдання були написані для Gin із замоканими обробниками. Їх переписано як справжні інтеграційні тести проти роутера цього проєкту, і вони вже в репозиторії: `internal/app/auth_test.go` (реєстрація, логін, валідація токена, CORS) та `internal/app/rbac_test.go` (матриця доступу для анонімного користувача, `member`, `staff` і `admin` та сценарій зміни ролі). Вони використовують наявний тестовий каркас `internal/app` (справжній PostgreSQL, `TEST_DATABASE_URL`) і покривають кейси завдання: дубль e-mail (`422`), успішний логін із JWT (`200`), неправильний пароль (`401`, узагальнене повідомлення), захищений маршрут без токена або з простроченим токеном (`401`), `member` на маршруті персоналу (`403`), `staff`/`admin` на ньому ж (`201`).
  * *Виконання:* запустіть їх до початку роботи (`go test ./internal/app/ -run 'Auth|RBAC' -v`) і переконайтеся, що вони падають на `404` для відсутніх маршрутів; реалізуйте; запустіть `go test ./... -race -count=1` із заданим `TEST_DATABASE_URL` і зробіть зеленим увесь набір. Розширюйте таблиці, якщо ваш дизайн додає поведінку. Наявні тести, `scripts/smoke.sh` і workflow Теми 8 пишуть у каталог без токена, тож після захисту записів вони мають аутентифікуватися як `staff` (оновіть `fixtures`/`call` в `internal/app/helpers_test.go` і скрипти).

---

### Вимоги до здачі домашнього завдання (Deliverables)

Надайте посилання на репозиторій, у якому є:

1. **Міграція БД:** `000003_add_auth_fields.up.sql` і `.down.sql` (додає `password_hash`; down-міграція прибирає колонку).
2. **Безпечна робота з паролями:** паролі хешуються `bcrypt` під час реєстрації; `PasswordHash` має тег `json:"-"`; хеш не потрапляє в жодну відповідь.
3. **Дисципліна помилок і токенів:** на `/auth/login` неможливий перебір акаунтів (однакове тіло `401` для обох невдач); JWT містить `exp` і підписаний `JWT_SECRET`.
4. **Порядок middleware:** `internal/app/app.go` чітко розділяє публічні, захищені, staff- і admin-маршрути та виконує `RequireRole` після `Auth`.
5. **Журнал промптів `prompts.txt`:** промпти завдань 2.1 і 2.2.
6. **Аудит безпеки `security_audit.md`:** щонайменше 5 знахідок із рівнями серйозності та статусами тріажу.
7. **Тести:** `go test ./... -race` зелений із `TEST_DATABASE_URL`, включно з `auth_test.go` і `rbac_test.go` (`401` і `403` перевірено для `member`, `staff` і `admin`).
8. **`README.md`** з розділом "Lesson 9 Report": матриця доступу (або `security_design.txt`), як отримати токен (`curl` для register і login) і стартовий адміністратор.

---

### Що перевіряють тести (`internal/app/auth_test.go`, `internal/app/rbac_test.go`)

- **Конфігурація.** Тести викликають `t.Setenv("JWT_SECRET", ...)` і лише потім будують роутер через `app.NewRouter`, тож секрет має зчитуватися під час побудови роутера (або на кожен запит), а не під час ініціалізації пакета. Токени, які тести створюють самі, — стандартні HS256 JWT (`{"alg":"HS256","typ":"JWT"}`, base64url без доповнення), підписані цим секретом; саме такі створює і приймає `github.com/golang-jwt/jwt/v5`.
- **Конверт помилок.** Кожна помилка — `{"error":{"code":"...","message":"..."}}`. Нові коди: `unauthenticated` (`401`) і `forbidden` (`403`); наявні `validation_error` (`422`), `not_found` (`404`) та `invalid_id` (`400`) зберігають значення.
- **`POST /api/v1/auth/register`** з `{"email","password"}`: `201` і користувач `{id, email, role: "member", created_at, updated_at}`; порожні або відсутні `email`/`password`, як і тіло, що не є JSON, → `422` `validation_error`; уже зареєстрований e-mail → `422` `validation_error` з повідомленням `email already registered` (завдання фіксує тут `422`, на відміну від `409`, який каталог використовує для дублів). Жодна відповідь не містить `password_hash`, bcrypt-хеша чи слова `password`.
- **`POST /api/v1/auth/login`** з `{"email","password"}`: `200 {"token":"<jwt>"}`; токен перевіряється `JWT_SECRET`, його claims — `user_id` (id користувача, число), `role` та `exp` приблизно на 24 год уперед. Неправильний пароль і невідомий e-mail → обидва `401` `unauthenticated`, повідомлення `invalid credentials`, тіла побайтово однакові. Відсутній пароль → `422` `validation_error`.
- **Middleware `Auth`.** Відсутній заголовок `Authorization`, схема не Bearer, `Bearer` без токена, пошкоджений токен, прострочений токен, токен з іншим секретом, токен зі зміненим після підпису payload і токен з `alg: none` → усі `401` `unauthenticated`. Токен, виданий логіном, або токен, підписаний спільним секретом, → захищений маршрут виконується.
- **`GET /api/v1/me`:** `200` з акаунтом за `user_id` токена (`id`, `email`, `role`), без хеша.
- **Матриця доступу** (статус для кожної ролі; anonymous = без токена):

  | Маршрут | anonymous | member | staff | admin |
  |---|---|---|---|---|
  | `GET /api/v1/albums`, `GET /api/v1/genres` | 200 | 200 | 200 | 200 |
  | `GET /api/v1/me` | 401 | 200 | 200 | 200 |
  | `POST /api/v1/genres`, `POST /api/v1/artists`, `POST /api/v1/albums`, `POST /api/v1/albums/{id}/copies` | 401 | 403 | 201 | 201 |
  | `PUT /api/v1/albums/{id}` | 401 | 403 | 200 | 200 |
  | `DELETE /api/v1/albums/{id}` | 401 | 403 | 204 | 204 |
  | `GET /api/v1/users`, `PUT /api/v1/users/{id}/role` | 401 | 403 | 403 | 200 |

  Відхилений запис нічого не створює. Токени `staff` і `admin` у матриці тести підписують спільним секретом, тож middleware має брати роль із claim `role` токена.
- **Адмінські маршрути.** `GET /api/v1/users` → `200` і JSON-масив користувачів без хешів. `PUT /api/v1/users/{id}/role` з `{"role":"staff"}` → `200` і оновлений користувач; роль поза `member`/`staff`/`admin` або відсутня → `422` `validation_error`; невідомий id → `404` `not_found`; нечисловий id → `400` `invalid_id`. Після зміни новий логін видає токен із новою роллю, і staff-маршрути відкриваються; старий токен зберігає стару роль (stateless JWT).
- **CORS.** Відповідь на preflight (`OPTIONS`) містить `Authorization` в `Access-Control-Allow-Headers`.

---

# Homework Assignment: Lesson 9 — API Security (Authentication, JWT and RBAC with AI)

## Capstone Project: Albums API (music album catalogue, the `albums` variant)

> Adapted from the LibraTrack assignment (the `books` variant). `Book` → `Album`, so the catalogue
> write endpoints (`genres`, `artists`, `albums`, `copies`) take the place of the book and loan
> endpoints; the roles are the ones the `users` table already has (`member` = reader,
> `staff` = librarian, `admin`); the router stays `net/http` (`http.ServeMux` with method patterns
> and middleware wrapping instead of Gin route groups). The contract the adapted tests pin down is
> in the last section; the tests themselves are already in the repository
> (`internal/app/auth_test.go`, `internal/app/rbac_test.go`) and stay red until the work is done.

**Time Budget:** Up to 2 hours.

**Objective:** Secure the **Albums API** following production-grade security standards. Implement full authentication with password hashing (`bcrypt`) and token issuance (`JWT`), add role-based authorization as middleware (`RBAC`), and run a structured AI-assisted security review of the codebase to find and fix vulnerabilities.

---

### PART 1: Manual Tasks (Security Architecture, Modelling & Review) [~35 mins]

*This part needs human design decisions, risk analysis and code-quality gates.*

* **Task 1.1: Roles, Access Matrix and DTO Design (Security & Contract Design):**
  1. Create a `security_design.txt` file (or a section in the new `README.md`).
  2. Define the roles. The schema already has three (`CHECK (role IN ('member','staff','admin'))` in `migrations/000001_init_schema.up.sql`): `member` (the default for new accounts, the assignment's `reader`), `staff` (the assignment's `librarian`) and `admin`.
  3. Write down the access-control matrix for the Albums API:
     * **Public (unauthenticated):** `POST /api/v1/auth/register`, `POST /api/v1/auth/login`, every `GET` of the catalogue (`/genres`, `/genres/{id}`, `/artists`, `/artists/{id}`, `/albums`, `/albums/{id}`, `/albums/{id}/copies`, `/albums/{id}/availability`), `/health` and `/swagger/`.
     * **Protected (any authenticated user):** `GET /api/v1/me` (the account behind the token; the albums variant has no loans, so this takes the place of `GET /me/loans`).
     * **Staff (`staff`, `admin`):** `POST /api/v1/genres`, `POST /api/v1/artists`, `POST /api/v1/albums`, `PUT /api/v1/albums/{id}`, `DELETE /api/v1/albums/{id}`, `POST /api/v1/albums/{id}/copies` (the catalogue writes replace `POST /books`, `PUT /books/:id`, `POST /loans`, `POST /loans/:id/return`).
     * **Admin (`admin`):** `GET /api/v1/users`, `PUT /api/v1/users/{id}/role`.
  4. Design the request DTOs so that mass assignment is impossible: `RegisterInput` (the assignment's `RegisterRequest`) carries only `email` and `password` (never `role`), `RoleInput` carries only `role`, and the `User` model gets `PasswordHash` with the `json:"-"` tag so a hash can never be serialised into a response. Handlers decode requests into these input structs, never into `model.User`. If you add a profile update, its input must not carry `role` or `password_hash` either.

* **Task 1.2: Manual Security Audit of Password Handling & Logging:**
  1. Audit the `/auth/login` handler: an unknown e-mail and a wrong password must produce the same `401` with the same body (`"invalid credentials"`), so the endpoint cannot be used to enumerate accounts. Run `bcrypt.CompareHashAndPassword` even when the e-mail is unknown (against a dummy hash) so the response time does not give the answer away either.
  2. Inspect `internal/middleware/logging.go` (Lesson 7): it logs only the method, path, status and duration. Keep it that way: no request bodies and no `Authorization` header in the log; if you ever add body logging, redact `password`.
  3. Verify middleware abort behaviour: `net/http` has no `c.AbortWithStatusJSON()`. An authentication or authorization failure must write the error response (`handler.WriteError`) and `return` **without** calling `next.ServeHTTP`, so the wrapped handler never runs.

---

### PART 2: Prompt Engineering (Authentication, JWT & RBAC Middleware) [~40 mins]

*Write precise, fully specified prompts for your AI assistant to generate the authentication and authorization code.*

* **Task 2.1: Prompting the Authentication Module (bcrypt + JWT issuance):**
  * Construct and execute a prompt that generates the migration pair `000003_add_auth_fields.up.sql` / `.down.sql` and updates the `User` model (`internal/model/user.go`). `role` already exists, so the migration adds `password_hash` only. The seed (`000002_seed_demo_data.up.sql`) already contains `admin@albums.local`, `staff@albums.local` and `member@albums.local`: give them a bcrypt hash of a documented development password (so `admin@albums.local` is the bootstrap admin), or add the column with `DEFAULT ''` and bootstrap the first admin another way. Either way, document it.
  * *Prompt requirements for `Register` and `Login`:*
    1. Registration hashes the password with `bcrypt.GenerateFromPassword` and `bcrypt.DefaultCost` (10), stores the role `member` and answers `201` with the user (without the hash).
    2. Login verifies the password with `bcrypt.CompareHashAndPassword` and answers `200 {"token": "<jwt>"}` with a token signed with `jwt.SigningMethodHS256` (`github.com/golang-jwt/jwt/v5`).
    3. The JWT carries the claims `user_id`, `role` and `exp` (24 hours). The signing secret comes strictly from `JWT_SECRET` (`os.Getenv`), read once at start-up in `main.go` (fail fast when empty) and passed to the router; do not read it into a package-level variable at import time, the tests set it with `t.Setenv` before building the router.
    4. Add `JWT_SECRET` to `.env.example` and `docker-compose.yml`, and update the Swagger annotations (`@securityDefinitions.apikey BearerAuth`, `@Security BearerAuth` on protected handlers).
  * *Deliverable:* save the prompt text to `prompts.txt` and integrate the generated files into `internal/auth/` (hashing and tokens) and `internal/handler/` (`auth_handler.go`, `user_handler.go`), with a `UserRepository` in `internal/repository/`.

* **Task 2.2: Prompting the Middleware Chain (`Auth` & `RequireRole`):**
  * Write a prompt that generates `Auth` (`internal/middleware/auth.go`) and `RequireRole` (`internal/middleware/rbac.go`) as `func(http.Handler) http.Handler` middleware, like the existing `Logging`, `Recover` and `CORS`.
  * *Prompt requirements:*
    1. `Auth` takes the token from `Authorization: Bearer <token>`, verifies the signature (HS256 only, reject any other `alg`) and the `exp` claim, and stores `user_id` and `role` in the request context (`context.WithValue` with an unexported key type and typed accessors such as `auth.UserFromContext(ctx)`, the `net/http` equivalent of `c.Set`). Failure: `401` `unauthenticated`.
    2. `RequireRole` is variadic (`allowed ...string`), reads the role from the context and answers `401` `unauthenticated` when there is no authenticated user and `403` `forbidden` when the role is not allowed.
    3. The prompt must show the route registration in `internal/app/app.go` with the `net/http` equivalent of nested groups: helpers such as `protected := middleware.Auth`, `staff := func(h http.Handler) http.Handler { return middleware.Auth(middleware.RequireRole("staff", "admin")(h)) }` and `admin := ...RequireRole("admin")`, applied per route (`mux.Handle("POST /api/v1/albums", staff(http.HandlerFunc(albums.Create)))`). `Auth` always runs before `RequireRole`.
    4. `CORS` must list `Authorization` in `Access-Control-Allow-Headers`, otherwise browsers cannot send the token.
  * *Deliverable:* save the prompt to `prompts.txt`.

---

### PART 3: Agentic AI Use (Automated AI Security Audit & Testing) [~45 mins]

* **Task 3.1: AI-Assisted Security Code Review:**
  * Supply your AI agent with the code of the modules you created or changed (`internal/auth`, `internal/middleware`, `internal/handler/auth_handler.go`, `user_handler.go`, `album_handler.go`, `copy_handler.go`, `internal/repository`).
  * *Agent prompt instruction:*
    > "Review this Go net/http API code for security vulnerabilities. Check specifically for: 1) SQL injection (raw queries, string concatenation); 2) Mass assignment (binding request JSON directly to DB models); 3) Missing authentication or RBAC checks; 4) Sensitive data exposure (leaking password hashes in JSON responses, secrets or tokens in logs). Output a structured list with line numbers, severity (Low/Medium/High), and proposed fixes. Do not rewrite files completely."
  * *Triage & Report:* critically evaluate the output. Compile `security_audit.md` with **at least 5 findings**, each with a severity and a triage status: *Fixed Now*, *Tracked for Later* or *False Positive*.

* **Task 3.2: Table-Driven Security Tests:**
  * The tests of the original assignment were written for Gin with mocked handlers. They have been rewritten as real integration tests against this project's router and are already in the repository: `internal/app/auth_test.go` (registration, login, token validation, CORS) and `internal/app/rbac_test.go` (the access matrix for anonymous, `member`, `staff` and `admin`, and the role-change flow). They use the existing `internal/app` test harness (a real PostgreSQL, `TEST_DATABASE_URL`) and cover the assignment's cases: duplicate e-mail (`422`), valid login returning a JWT (`200`), wrong password (`401`, generic message), protected route with a missing or expired token (`401`), `member` calling a staff route (`403`), `staff`/`admin` calling it (`201`).
  * *Execution:* run them before you start (`go test ./internal/app/ -run 'Auth|RBAC' -v`) and watch them fail on `404` for the missing routes; implement; run `go test ./... -race -count=1` with `TEST_DATABASE_URL` set and make the whole suite green. Extend the tables if your design adds behaviour. The existing tests, `scripts/smoke.sh` and the Theme 8 workflow write to the catalogue without a token, so they must authenticate as `staff` once the writes are protected (update `fixtures`/`call` in `internal/app/helpers_test.go` and the scripts).

---

### Evaluation Criteria & Deliverables (Self-Check)

Submit a link to the repository containing:

1. **DB migration:** `000003_add_auth_fields.up.sql` and `.down.sql` (adds `password_hash`; the down migration removes it).
2. **Secure password handling:** passwords hashed with `bcrypt` on registration; `PasswordHash` annotated with `json:"-"`; no hash in any response.
3. **Error & token discipline:** no user enumeration on `/auth/login` (the same `401` body for both failures); the JWT carries `exp` and is signed with `JWT_SECRET`.
4. **Middleware ordering:** `internal/app/app.go` cleanly separates the public, protected, staff and admin routes and runs `RequireRole` after `Auth`.
5. **Prompt log `prompts.txt`:** the prompts of Tasks 2.1 and 2.2.
6. **Security audit `security_audit.md`:** at least 5 findings with severities and triage statuses.
7. **Tests:** `go test ./... -race` is green with `TEST_DATABASE_URL`, including `auth_test.go` and `rbac_test.go` (`401` and `403` verified for `member`, `staff` and `admin`).
8. **`README.md`** with a "Lesson 9 Report": the access matrix (or `security_design.txt`), how to obtain a token (`curl` for register and login) and the bootstrap admin.

---

### What the tests check (`internal/app/auth_test.go`, `internal/app/rbac_test.go`)

- **Configuration.** The tests call `t.Setenv("JWT_SECRET", ...)` and only then build the router through `app.NewRouter`, so the secret must be read when the router is built (or per request), not at package init. Tokens the tests mint themselves are standard HS256 JWTs (`{"alg":"HS256","typ":"JWT"}`, base64url without padding) signed with that secret, which is what `github.com/golang-jwt/jwt/v5` produces and accepts.
- **Envelope.** Every error is `{"error":{"code":"...","message":"..."}}`. New codes: `unauthenticated` (`401`) and `forbidden` (`403`); the existing `validation_error` (`422`), `not_found` (`404`) and `invalid_id` (`400`) keep their meaning.
- **`POST /api/v1/auth/register`** with `{"email","password"}`: `201` and the user `{id, email, role: "member", created_at, updated_at}`; blank or missing `email`/`password`, like a body that is not JSON, → `422` `validation_error`; an e-mail that is already registered → `422` `validation_error` with the message `email already registered` (the assignment fixes `422` here, unlike the `409` the catalogue uses for duplicates). No response ever contains `password_hash`, a bcrypt hash or the word `password`.
- **`POST /api/v1/auth/login`** with `{"email","password"}`: `200 {"token":"<jwt>"}`; the token verifies with `JWT_SECRET`, its claims are `user_id` (the user's id, a number), `role` and `exp` about 24 h ahead. Wrong password and unknown e-mail both → `401` `unauthenticated`, message `invalid credentials`, byte-identical bodies. A missing password → `422` `validation_error`.
- **`Auth` middleware.** No `Authorization` header, a non-Bearer scheme, `Bearer` without a token, a malformed token, an expired token, a token signed with another secret, a token whose payload was changed after signing and an `alg: none` token all → `401` `unauthenticated`. A token issued by login, or one minted with the shared secret, → the protected route runs.
- **`GET /api/v1/me`:** `200` with the account of the token's `user_id` (`id`, `email`, `role`), no hash.
- **Access matrix** (status per role; anonymous = no token):

  | Route | anonymous | member | staff | admin |
  |---|---|---|---|---|
  | `GET /api/v1/albums`, `GET /api/v1/genres` | 200 | 200 | 200 | 200 |
  | `GET /api/v1/me` | 401 | 200 | 200 | 200 |
  | `POST /api/v1/genres`, `POST /api/v1/artists`, `POST /api/v1/albums`, `POST /api/v1/albums/{id}/copies` | 401 | 403 | 201 | 201 |
  | `PUT /api/v1/albums/{id}` | 401 | 403 | 200 | 200 |
  | `DELETE /api/v1/albums/{id}` | 401 | 403 | 204 | 204 |
  | `GET /api/v1/users`, `PUT /api/v1/users/{id}/role` | 401 | 403 | 403 | 200 |

  A rejected write creates nothing. The staff and admin tokens of the matrix are minted by the tests with the shared secret, so the middleware must take the role from the token's `role` claim.
- **Admin routes.** `GET /api/v1/users` → `200` and a JSON array of users without hashes. `PUT /api/v1/users/{id}/role` with `{"role":"staff"}` → `200` and the updated user; a role outside `member`/`staff`/`admin`, or a missing one, → `422` `validation_error`; unknown id → `404` `not_found`; non-numeric id → `400` `invalid_id`. After the change a fresh login issues a token with the new role and the staff gate opens; the old token keeps its old role (stateless JWT).
- **CORS.** The preflight (`OPTIONS`) answer lists `Authorization` in `Access-Control-Allow-Headers`.
