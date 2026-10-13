<!--
ШАБЛОН README ДЛЯ ТЕМИ 9. Як ним користуватися:
1. Усе без позначки TODO вже правдиве для коду Теми 8 або зафіксоване умовою
   albums_lesson9_homework.md і приймальними тестами internal/app/auth_test.go та rbac_test.go.
2. Кожну позначку TODO (HTML-коментар у тексті, "# TODO" у прикладах команд) заповніть або
   видаліть після реалізації. Перевірка перед здачею: grep -n TODO README.md (має бути порожньо).
3. Після реалізації звірте назви файлів у розділі «Структура проєкту», кількості в smoke-тесті
   та статуси в таблиці звіту. Потім видаліть цей коментар.
-->

# Albums API

REST-сервіс обліку музичних альбомів на Go з PostgreSQL. Проєкт виконано в межах курсу
«Go with GenAI», варіант `albums`. Тема 7 дала REST API зі сховищем у пам'яті; Тема 8 перевела
його на PostgreSQL через GORM і golang-migrate, нормалізувала схему та виправила N+1; Тема 9
захищає API: реєстрація і логін з `bcrypt` та JWT, рольова авторизація (`member`, `staff`, `admin`)
у вигляді middleware і AI-асистований аудит безпеки.

Умова Теми 9, адаптована під варіант `albums`: [albums_lesson9_homework.md](albums_lesson9_homework.md).
Звіт про виконання: розділ [Lesson 9 Report](#lesson-9-report-аутентифікація-jwt-та-rbac).

## Швидкий запуск (Docker Compose)

Потрібен Docker з Compose v2. PostgreSQL 16 і API піднімаються разом; міграції (схема та демо-дані:
3 жанри, 6 виконавців, 10 альбомів, 24 примірники, 3 акаунти з ролями `admin`, `staff` і `member`)
застосовуються автоматично на старті.

```sh
# 1. Клонувати репозиторій
git clone https://github.com/tatarchuk/golang-course-lesson-9.git && cd golang-course-lesson-9

# 2. Конфігурація (значення збігаються зі стандартними в docker-compose.yml; JWT_SECRET для
#    локального запуску вже заданий у .env.example, для будь-чого іншого його треба змінити)
cp .env.example .env

# 3. Зібрати образ і запустити PostgreSQL + API (http://localhost:8080)
docker compose up -d --build

# 4. Перевірити
curl -s localhost:8080/health                      # {"status":"ok"}
curl -s 'localhost:8080/api/v1/albums?limit=2'     # каталог читається без токена
curl -s -X POST localhost:8080/api/v1/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"admin@albums.local","password":"<dev-пароль>"}'   # {"token":"<jwt>"}; TODO: пароль з розділу «Стартовий адміністратор»
docker compose logs -f api                         # кожен SQL-запит потрапляє в лог
docker compose down                                # зупинити; дані лишаються у volume pgdata
```

<!-- TODO: після завдання 2.1 переконайтеся, що JWT_SECRET є в .env.example і в середовищі сервісу api
в docker-compose.yml; інакше крок 2 має містити окремий рядок про JWT_SECRET. -->

Те саме через `make up`, `make logs`, `make down` (усі цілі: `make` без аргументів показує Makefile).

Скриншоти працюючого стека: [Swagger UI з виконаним `GET /api/v1/albums` і відповіддю 200](docs/screenshots/swagger-get-albums-200.jpg)
та [огляд API у Swagger UI](docs/screenshots/swagger-overview.jpg).
<!-- TODO (за бажанням): скриншот Swagger UI з кнопкою Authorize і захищеним запитом (401 без токена, 201 зі staff-токеном). -->

### Запуск на хості

Потрібен Go 1.25.11 або новіший (версія з `go.mod`). База береться з compose, сервер запускається
локально і так само застосовує міграції на старті.

```sh
docker compose up -d postgres        # або make db
set -a; . ./.env; set +a             # DATABASE_URL, JWT_SECRET, MIGRATIONS_PATH, PORT
go run ./cmd/api                     # або make run
```

| Змінна | Призначення | Типово |
|---|---|---|
| `DATABASE_URL` | рядок підключення PostgreSQL, обов'язкова | — |
| `JWT_SECRET` | секрет підпису JWT (HS256), обов'язкова: з порожнім значенням сервер не стартує | — |
| `MIGRATIONS_PATH` | тека з міграціями golang-migrate | `migrations` |
| `PORT` | порт HTTP-сервера | `8080` |
| `DB_LOG_LEVEL` | логер GORM: `info` (кожен запит), `warn`, `error`, `silent` | `info` |

### Тести та перевірки

```sh
docker compose up -d postgres                 # базу albums_test створює scripts/initdb на першому старті
go test ./internal/app/ -run 'Auth|RBAC' -v   # приймальні тести Теми 9 (червоні, поки маршрутів немає)
make test                                     # go test ./... -race -count=1 з TEST_DATABASE_URL із .env
go test ./internal/model ./internal/handler ./internal/middleware ./internal/auth   # без бази даних
gofmt -l . && go vet ./...
scripts/smoke.sh http://localhost:8080        # повтор API smoke-тесту CI проти запущеного сервера
```

Тести з базою (`internal/repository`, `internal/app`) читають `TEST_DATABASE_URL`; без неї вони
пропускаються, а в CI змінна задана, тож пропусків немає. `JWT_SECRET` тести задають самі
(`t.Setenv` перед побудовою роутера), тому для `make test` він у `.env` не потрібен. Пакети, що
працюють із базою, серіалізуються advisory-lock'ом (`internal/testdb`), кожен тест починає з
порожніх таблиць.

- `internal/model`: валідація вхідних структур, `null` для необов'язкових полів, хибні JSON-типи;
  <!-- TODO: + RegisterInput, LoginInput, RoleInput (порожні поля, роль поза списком). -->
- `internal/auth`: <!-- TODO: хешування та перевірка пароля; видача токена; відхилення токена з іншим секретом, іншим alg, простроченого. -->
- `internal/repository`: інтеграційні тести з реальною БД: створення зі зв'язками, невідомий жанр або
  виконавець (`invalid_reference`), дублікати (`conflict`), список із фільтром і пагінацією, повна
  заміна у `PUT`, каскад при видаленні, примірники та доступність, ідемпотентність міграцій,
  guard-тест `TestListQueryCountIsConstant` на N+1.
  <!-- TODO: + UserRepository: створення, пошук за e-mail та id, дубль e-mail, список, зміна ролі. -->
- `internal/handler`: обробники з фейковими репозиторіями: порядок перевірок, коди помилок, парсинг.
  <!-- TODO: + auth_handler (422 для порожніх полів і дубля, однаковий 401), user_handler (422/404/400). -->
- `internal/middleware`: паніка → 500, CORS (з `Authorization`), логування.
  <!-- TODO: + Auth (401 для кожного виду хибного токена, користувач у контексті) і RequireRole (401/403, обробник не викликається). -->
- `internal/app`: e2e через `NewRouter()` і реальну БД: сценарій smoke-тесту CI, граничні випадки
  контракту Теми 7 і приймальні тести Теми 9: `auth_test.go` (`TestAuthRegisterAndLogin`,
  `TestAuthLoginFailuresAreIndistinguishable`, `TestAuthTokenValidation`,
  `TestAuthCORSAllowsAuthorizationHeader`) та `rbac_test.go` (`TestRBACMatrix`, `TestRBACRoleChange`).

CI ([.github/workflows/theme8-check.yml](.github/workflows/theme8-check.yml)) перевіряє структуру
файлів, статичні правила, міграції на PostgreSQL, тести з реальною БД і smoke-тест API.
<!-- TODO: smoke-тест workflow і scripts/smoke.sh пишуть у каталог без токена; після захисту записів
вони мають логінитися як staff. Вкажіть, що змінено, результат локального smoke-тесту (N/N) і посилання
на зелений запуск CI. -->

### Документація API

- Swagger UI із запуском запитів з браузера: <http://localhost:8080/swagger/index.html>; для захищених
  маршрутів кнопка **Authorize** приймає `Bearer <token>` (`@securityDefinitions.apikey BearerAuth`).
- Специфікація OpenAPI: [docs/swagger.yaml](docs/swagger.yaml), [docs/swagger.json](docs/swagger.json).
- Runbook із `curl`-запитами на всі випадки контракту: [docs/CURL.md](docs/CURL.md).
  <!-- TODO: додати до CURL.md розділ про register/login/me/users і випадки 401/403 або послатися на розділ «Як отримати токен» нижче. -->

Специфікацію згенеровано утилітою [swag](https://github.com/swaggo/swag) з анотацій у `cmd/api/main.go`
та обробниках `internal/handler/*_handler.go`; після їх зміни: `make swagger`.
<!-- TODO: після додавання @Security BearerAuth на захищені обробники перегенерувати docs/. -->

## API

Усі відповіді, крім `204`, мають `Content-Type: application/json`; ключі в `snake_case`.
Необов'язкові поля завжди присутні у відповіді (`null`, якщо не задані); час у RFC 3339 (UTC, `Z`).
Захищені маршрути приймають заголовок `Authorization: Bearer <token>`; перевірка доступу виконується
до перевірок запиту: без токена або з хибним токеном → `401` `unauthenticated`, з недозволеною
роллю → `403` `forbidden`, і відхилений запит нічого не змінює.

### Ресурси

```json
{
  "id": 1,
  "title": "Kind of Blue",
  "label": "Columbia",
  "genre_id": 1,
  "genre": {"id": 1, "name": "jazz", "created_at": "2026-10-09T10:00:00Z", "updated_at": "2026-10-09T10:00:00Z"},
  "artists": [
    {"id": 1, "name": "Miles Davis", "bio": "Trumpeter and bandleader.", "created_at": "...", "updated_at": "..."},
    {"id": 2, "name": "John Coltrane", "bio": null, "created_at": "...", "updated_at": "..."}
  ],
  "release_year": 1959,
  "notes": null,
  "created_at": "2026-10-09T10:00:00Z",
  "updated_at": "2026-10-09T10:00:00Z"
}
```

Тіло `POST`/`PUT /albums` посилається на жанр і виконавців за `id`:

| Поле | Тип | Обов'язкове | Примітка |
|---|---|---|---|
| `title`, `label` | рядок | так | не порожні й не лише з пробілів; пара `title` + `label` унікальна |
| `genre_id` | ціле | так | має існувати, інакше `422` `invalid_reference` |
| `artist_ids` | масив цілих | так | непорожній, без повторів; невідомий id → `422` `invalid_reference` |
| `release_year` | ціле | ні | від 0 до 9999; `null`, якщо не задано |
| `notes` | рядок | ні | `null`, якщо не задано |

Інші ресурси каталогу: жанр `{"id","name",...}` (вхід `{"name"}`, ім'я унікальне), виконавець
`{"id","name","bio",...}` (вхід `{"name","bio"}`), примірник `{"id","album_id","inventory_no","status",...}`
(вхід `{"inventory_no","status"}`, статус `available` | `borrowed` | `lost`, типово `available`,
`inventory_no` унікальний), доступність `{"album_id","total_copies","available_copies"}`.

Акаунт: `{"id","email","role","created_at","updated_at"}`; хеш пароля не потрапляє в жодну відповідь
(`PasswordHash` має тег `json:"-"`). Токен: `{"token":"<jwt>"}`, HS256 із секретом `JWT_SECRET`,
claims `user_id`, `role`, `exp` (24 години). Вхідні структури навмисно вузькі, щоб mass assignment
був неможливий:

| Запит | Тіло | Примітка |
|---|---|---|
| `POST /auth/register`, `POST /auth/login` | `{"email","password"}` | обидва обов'язкові й не порожні; поля `role` у вході немає: новий акаунт завжди `member` |
| `PUT /users/{id}/role` | `{"role"}` | `member` \| `staff` \| `admin`, інакше `422` `validation_error` |

### Ендпоінти

| Запит | Доступ | Відповідь |
|---|---|---|
| `GET /health` | публічний | `200` `{"status":"ok"}` |
| `POST /api/v1/auth/register` | публічний | `201` акаунт із `role: "member"`; порожні або відсутні `email`/`password`, тіло не JSON → `422` `validation_error`; зайнятий e-mail → `422` `validation_error` з повідомленням `email already registered` |
| `POST /api/v1/auth/login` | публічний | `200` `{"token":"<jwt>"}`; невідомий e-mail і хибний пароль → той самий `401` `unauthenticated` `invalid credentials`; відсутній пароль → `422` |
| `GET /api/v1/me` | будь-який аутентифікований | `200` акаунт власника токена; без токена → `401` |
| `POST /api/v1/genres` | `staff`, `admin` | `201`; дубль імені → `409` `conflict`; порожнє ім'я → `422` |
| `GET /api/v1/genres`, `GET /api/v1/genres/{id}` | публічний | `200`; невідомий id → `404` |
| `POST /api/v1/artists` | `staff`, `admin` | `201`; порожнє ім'я → `422` |
| `GET /api/v1/artists`, `GET /api/v1/artists/{id}` | публічний | `200`; невідомий id → `404` |
| `POST /api/v1/albums` | `staff`, `admin` | `201` з вкладеними `genre` та `artists`; дубль `title`+`label` → `409`; невідомі посилання → `422` `invalid_reference`; інша валідація → `422` `validation_error` |
| `GET /api/v1/albums?page=&limit=&genre_id=` | публічний | `200` масив (порожній як `[]`); `400` `invalid_pagination` або `validation_error` |
| `GET /api/v1/albums/{id}` | публічний | `200`; `400` `invalid_id`; `404` `not_found` |
| `PUT /api/v1/albums/{id}` | `staff`, `admin` | `200`, повна заміна, виконавці замінюються списком; `400`, `422`, `404` |
| `DELETE /api/v1/albums/{id}` | `staff`, `admin` | `204` без тіла; каскадом зникають примірники та зв'язки; `400`, `404` |
| `POST /api/v1/albums/{id}/copies` | `staff`, `admin` | `201` (`status` типово `available`); дубль `inventory_no` → `409`; хибний статус → `422`; невідомий альбом → `404` |
| `GET /api/v1/albums/{id}/copies`, `GET /api/v1/albums/{id}/availability` | публічний | `200`; невідомий альбом → `404` |
| `GET /api/v1/users` | `admin` | `200` масив акаунтів без хешів |
| `PUT /api/v1/users/{id}/role` | `admin` | `200` оновлений акаунт; роль поза списком або відсутня → `422` `validation_error`; невідомий id → `404`; нечисловий id → `400` `invalid_id` |
| `GET /swagger/` | публічний | Swagger UI |

- **Список:** порядок за `id`; `page` і `limit` типово 1 і 10, мають бути цілими `>= 1`, `limit` понад
  100 обрізається до 100; `genre_id` — точний збіг, порожнє значення означає «без фільтра».
- **`PUT` — повна заміна:** необов'язкові поля, яких немає в тілі, стають `null`; `artist_ids` замінюють
  попередній список. Порядок перевірок: доступ (401/403) → `id` (400) → тіло (422) → існування (404) →
  посилання (422); відхилений запит нічого не змінює (транзакція).
- **`{id}`** — ціле від 0 до 4294967295 (`uint32`), інакше `400` `invalid_id`.
- **Помилки** мають єдиний формат `{"error":{"code":"…","message":"…"}}` без внутрішніх деталей. Коди:
  `invalid_id`, `invalid_pagination`, `validation_error`, `invalid_reference`, `conflict`, `not_found`,
  `unauthenticated`, `forbidden`, `internal_server_error`. Невідомий маршрут і непідтримуваний метод
  дають JSON `404`.
- **Токен без стану:** зміна ролі через `PUT /users/{id}/role` діє з наступного логіну; виданий раніше
  токен зберігає стару роль до `exp`.
- **CORS:** кожна відповідь має `Access-Control-Allow-Origin: *`; `OPTIONS` → `204` зі списком методів
  і заголовків, серед яких `Authorization`.

## Структура проєкту та прийняті рішення

<!-- TODO: звірити назви нових файлів з тим, що реально з'явилося в репозиторії. -->

```
cmd/api/main.go                      конфігурація з env (JWT_SECRET читається один раз, порожній → fail fast), міграції, HTTP-сервер, graceful shutdown
internal/app/app.go                  NewRouter(repos): маршрути за групами public / protected / staff / admin → ServeMux → Logging(Recover(CORS))
internal/auth/                       bcrypt-хешування, видача та перевірка JWT, користувач у контексті запиту (нове)
internal/model/                      GORM-моделі Album, Artist, Genre, Copy, User (PasswordHash json:"-"), AlbumArtist + вхідні структури з Validate()
internal/repository/                 інтерфейси та GORM-реалізації (+ user_repository.go), errors.go, db.go (Open), migrate.go (Migrate)
internal/handler/                    HTTP-обробники (*_handler.go, + auth_handler.go, user_handler.go), errors.go (коди, відображення помилок), парсинг
internal/middleware/                 Logging, Recover, CORS (+ Authorization), Auth (нове), RequireRole (нове)
internal/testdb/                     спільне підключення для тестів із БД (advisory lock, міграції, TRUNCATE)
migrations/                          000001_init_schema, 000002_seed_demo_data, 000003_add_auth_fields (up/down)
docs/                                OpenAPI (swag, BearerAuth), runbook з curl, ER-діаграма, докази N+1 Теми 8
scripts/                             initdb/ (бази для тестів), smoke.sh (smoke-тест CI, логін як staff), measure_n1.sh (N+1), render_er.sh
Dockerfile, docker-compose.yml       образ API (multi-stage) і стек postgres + api (JWT_SECRET у середовищі api)
Makefile, .env.example               щоденні команди та приклад конфігурації (з JWT_SECRET)
albums_lesson9_homework.md           умова Теми 9, адаптована під варіант albums
prompts.txt                          промпти завдань 2.1 і 2.2
security_audit.md                    AI-аудит безпеки: знахідки, серйозність, тріаж
.github/workflows/theme8-check.yml   CI Теми 8 (структура, міграції, тести, smoke), адаптований під варіант albums
```

Рішення Теми 8, які лишаються в силі:

- **Шари.** Обробники залежать лише від інтерфейсів репозиторіїв; `NewRouter` отримує їх структурою
  `repository.Repositories`, тож тести обробників працюють на фейках, а `main` передає GORM-реалізації.
- **Запис альбому** виконується в транзакції: спочатку перевіряються жанр і виконавці (невідомий id →
  `ErrInvalidReference`), потім вставляється рядок `albums` з `Omit(clause.Associations)`, а рядки
  `album_artists` пишуться явно. `PUT` оновлює колонки через `Select(...)`, тому `nil`-вказівники
  пишуться як `NULL`, а зв'язки перезаписуються в тій самій транзакції.
- **Помилки драйвера** перекладаються один раз (`TranslateError: true` → `translate()` →
  `ErrConflict`, `ErrInvalidReference`, `ErrInvalidInput`), обробники мапують їх на 409/422.
- **Схема лише через міграції:** `AutoMigrate` не використовується; `Migrate()` запускає golang-migrate
  як бібліотеку (джерело `iofs`, драйвер `pgx/v5`) на старті застосунку. Каскади оголошені в БД;
  жанри й виконавці захищені `ON DELETE RESTRICT`.
- **N+1:** `List` довантажує зв'язки через `Preload`, 4 `SELECT` незалежно від кількості альбомів;
  логер GORM на рівні `info` пише кожен SQL-запит у stdout.

Рішення Теми 9:

- **Секрет JWT.** `main.go` читає `JWT_SECRET` один раз на старті й завершується з помилкою, якщо він
  порожній; роутер отримує секрет під час побудови, а не в package-level змінній, тому тести задають
  його через `t.Setenv`. <!-- TODO: як саме секрет потрапляє в роутер/auth (аргумент NewRouter, структура конфігурації, читання env у NewRouter). -->
- **Паролі.** `bcrypt.GenerateFromPassword` з `bcrypt.DefaultCost` (10). Логін викликає
  `bcrypt.CompareHashAndPassword` і для невідомого e-mail (з фіктивним хешем), тому тіло `401` і час
  відповіді однакові для обох невдач, перебір акаунтів неможливий.
- **Токен.** Лише HS256: будь-який інший `alg` (зокрема `none`), хибний підпис, змінений payload
  і прострочений `exp` → `401`. Claims `user_id`, `role`, `exp` = 24 години. Токен без стану: роль
  береться з claim, тому нова роль діє з наступного логіну.
- **Контекст.** `user_id` і `role` кладуться в контекст запиту через `context.WithValue` з
  неекспортованим типом ключа і читаються типізованими акцесорами (`auth.UserFromContext(ctx)`).
- **Ланцюжок middleware.** Групи маршрутів у `net/http` — хелпери в `app.go`:
  `protected := middleware.Auth`, `staff := Auth(RequireRole("staff","admin")(h))`,
  `admin := Auth(RequireRole("admin")(h))`, застосовані до кожного маршруту. `Auth` завжди виконується
  перед `RequireRole`; невдача пише відповідь через `handler.WriteError` і робить `return` без
  `next.ServeHTTP`, тож обгорнутий обробник не виконується.
- **DTO.** Вхідні структури `RegisterInput`, `LoginInput`, `RoleInput` не містять `role` (крім
  `RoleInput`) чи `password_hash`; обробники декодують запити лише в них, ніколи в `model.User`.
- **Логування** без змін: метод, шлях, статус, тривалість; жодних тіл запитів і заголовка
  `Authorization`.
- **Стартовий адміністратор:** <!-- TODO: одне речення про обране рішення; деталі в розділі «Стартовий адміністратор». -->

## Lesson 9 Report: аутентифікація, JWT та RBAC

Умова: [albums_lesson9_homework.md](albums_lesson9_homework.md). Стан виконання:

| Частина | Що потрібно | Стан |
|---|---|---|
| 1. Вручну | ролі й матриця доступу, DTO без mass assignment; аудит логіну, логування та переривання middleware | <!-- TODO: готово / частково --> |
| 2. Промпт-інжиніринг | промпти для auth-модуля (міграція `000003`, модель `User`, `Register`/`Login`, JWT) і для middleware (`Auth`, `RequireRole`, групи маршрутів, CORS); `prompts.txt` | <!-- TODO --> |
| 3. AI-агент | security-рев'ю → `security_audit.md` (щонайменше 5 знахідок із тріажем); `auth_test.go` і `rbac_test.go` зелені | <!-- TODO --> |

### Частина 1. Ролі, матриця доступу та DTO

<!-- TODO: цей розділ закриває завдання 1.1 замість окремого security_design.txt; якщо хочете окремий файл, перенесіть розділ туди й лишіть посилання. -->

Ролі ті, що вже є в схемі (`CHECK (role IN ('member','staff','admin'))` у
[migrations/000001_init_schema.up.sql](migrations/000001_init_schema.up.sql)):

| Роль | В умові | Хто це | Що може |
|---|---|---|---|
| `member` | reader | типова роль нового акаунта | читати каталог, бачити свій акаунт (`GET /me`) |
| `staff` | librarian | персонал каталогу | усе, що `member`, плюс створення, зміна й видалення жанрів, виконавців, альбомів і примірників |
| `admin` | admin | адміністратор | усе, що `staff`, плюс список акаунтів і зміна ролей |

Публічні маршрути (токен не перевіряється): `POST /api/v1/auth/register`, `POST /api/v1/auth/login`,
усі `GET` каталогу (`/genres`, `/genres/{id}`, `/artists`, `/artists/{id}`, `/albums`, `/albums/{id}`,
`/albums/{id}/copies`, `/albums/{id}/availability`), `/health` і `/swagger/`.

Матриця доступу (статус для кожної ролі; anonymous = без токена; саме її перевіряє `TestRBACMatrix`):

| Маршрут | anonymous | member | staff | admin |
|---|---|---|---|---|
| `GET /api/v1/albums`, `GET /api/v1/genres` та інші `GET` каталогу | 200 | 200 | 200 | 200 |
| `GET /api/v1/me` | 401 | 200 | 200 | 200 |
| `POST /api/v1/genres`, `POST /api/v1/artists`, `POST /api/v1/albums`, `POST /api/v1/albums/{id}/copies` | 401 | 403 | 201 | 201 |
| `PUT /api/v1/albums/{id}` | 401 | 403 | 200 | 200 |
| `DELETE /api/v1/albums/{id}` | 401 | 403 | 204 | 204 |
| `GET /api/v1/users`, `PUT /api/v1/users/{id}/role` | 401 | 403 | 403 | 200 |

Відхилений запис нічого не створює.

DTO проти mass assignment (`internal/model/user.go`): <!-- TODO: звірити назви структур і полів з кодом. -->

| Структура | Поля | Де використовується |
|---|---|---|
| `RegisterInput` | `email`, `password` | `POST /auth/register`; роль задає сервер (`member`) |
| `LoginInput` | `email`, `password` | `POST /auth/login` |
| `RoleInput` | `role` | `PUT /users/{id}/role` |
| `User` (модель) | `id`, `email`, `role`, `password_hash` (`json:"-"`), `created_at`, `updated_at` | відповіді; хеш ніколи не серіалізується |

Ручний аудит (завдання 1.2):

- **Логін без перебору акаунтів.** Невідомий e-mail і хибний пароль дають побайтово однакову відповідь
  `401` `{"error":{"code":"unauthenticated","message":"invalid credentials"}}`; `bcrypt.CompareHashAndPassword`
  виконується в обох випадках. <!-- TODO: де саме (internal/handler/auth_handler.go:NN) і що це покриває TestAuthLoginFailuresAreIndistinguishable. -->
- **Логування.** `internal/middleware/logging.go` пише лише метод, шлях, статус і тривалість; тіла
  запитів і заголовок `Authorization` у лог не потрапляють. <!-- TODO: підтвердити після змін. -->
- **Переривання ланцюжка.** `Auth` і `RequireRole` після помилки викликають `handler.WriteError` і
  повертаються без `next.ServeHTTP`. <!-- TODO: internal/middleware/auth.go:NN, rbac.go:NN; який тест це доводить (обробник не викликається). -->

### Як отримати токен

```sh
BASE=localhost:8080/api/v1

# Реєстрація: новий акаунт завжди member (ids у вас можуть відрізнятися)
curl -s -X POST $BASE/auth/register -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"Correct-Horse-Battery-1"}'
# 201 {"id":4,"email":"alice@example.com","role":"member","created_at":"...","updated_at":"..."}

# Логін: токен на 24 години
TOKEN=$(curl -s -X POST $BASE/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"alice@example.com","password":"Correct-Horse-Battery-1"}' | jq -r .token)

# Захищений маршрут
curl -s $BASE/me -H "Authorization: Bearer $TOKEN"
# 200 {"id":4,"email":"alice@example.com","role":"member",...}

# member не може писати в каталог (403 forbidden); без токена взагалі 401 unauthenticated
curl -s -X POST $BASE/genres -H "Authorization: Bearer $TOKEN" -H 'Content-Type: application/json' -d '{"name":"ambient"}'
curl -s -X POST $BASE/genres -H 'Content-Type: application/json' -d '{"name":"ambient"}'

# staff або admin: 201
ADMIN=$(curl -s -X POST $BASE/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"admin@albums.local","password":"<dev-пароль>"}' | jq -r .token)   # TODO: dev-пароль
curl -s -X POST $BASE/genres -H "Authorization: Bearer $ADMIN" -H 'Content-Type: application/json' -d '{"name":"ambient"}'

# Адміністратор підвищує alice до staff: новий логін alice видає токен із новою роллю,
# старий токен лишається member до exp
curl -s -X PUT $BASE/users/4/role -H "Authorization: Bearer $ADMIN" -H 'Content-Type: application/json' -d '{"role":"staff"}'
curl -s $BASE/users -H "Authorization: Bearer $ADMIN"
```

### Стартовий адміністратор

<!-- TODO: задокументуйте обране рішення завдання 2.1 (один із варіантів):
A. Seed-акаунтам admin@albums.local, staff@albums.local і member@albums.local міграція
   000003_add_auth_fields задає bcrypt-хеш задокументованого dev-пароля (UPDATE після ALTER TABLE;
   у 000002 колонки ще немає), тож admin@albums.local і є стартовим адміністратором. Вкажіть пароль,
   де лежить хеш і що для не-локального запуску пароль треба змінити.
B. Колонка password_hash додається з DEFAULT '' (seed-акаунти увійти не можуть), а перший
   адміністратор створюється інакше (SQL після старту, окрема команда тощо). Опишіть кроки.
-->

| Акаунт | Роль | Пароль |
|---|---|---|
| `admin@albums.local` | `admin` | <!-- TODO --> |
| `staff@albums.local` | `staff` | <!-- TODO --> |
| `member@albums.local` | `member` | <!-- TODO --> |

### Частина 2. Промпти для генерації коду

Промпти збережено у [prompts.txt](prompts.txt). <!-- TODO: створити файл. -->

**2.1. Модуль аутентифікації.** На вхід: `internal/model/user.go`, міграції `000001` і `000002`,
`internal/handler/errors.go`, `respond.go`, `request.go`, інтерфейси `internal/repository`. Результат:
`migrations/000003_add_auth_fields.{up,down}.sql`, оновлений `internal/model/user.go`, `internal/auth/`,
`internal/repository/user_repository.go`, `internal/handler/auth_handler.go` і `user_handler.go`.
Ключові вимоги, закладені в промпт:

- міграція додає лише `password_hash` (колонка `role` уже є); down-міграція прибирає колонку;
- реєстрація: `bcrypt.GenerateFromPassword` з `bcrypt.DefaultCost`, роль `member`, `201` без хеша;
  зайнятий e-mail → `422` `validation_error` `email already registered`;
- логін: `bcrypt.CompareHashAndPassword` (і для невідомого e-mail), `200 {"token"}`, HS256 через
  `github.com/golang-jwt/jwt/v5`, однакова `401`-відповідь для обох невдач;
- claims `user_id`, `role`, `exp` (24 год); секрет лише з `JWT_SECRET`, зчитаний один раз у `main.go`
  (fail fast), без package-level змінної;
- `JWT_SECRET` у `.env.example` і `docker-compose.yml`; анотації Swagger
  (`@securityDefinitions.apikey BearerAuth`, `@Security BearerAuth`);
- конверт помилок проєкту та наявні хелпери (`handler.WriteError`, `respond.go`, `request.go`).

<details>
<summary>Повний текст промпту 2.1</summary>

```text
TODO: вставити текст промпту з prompts.txt
```

</details>

Аналіз результату: <!-- TODO: що асистент згенерував, що довелося виправити вручну (напр. місце читання секрету, код для дубля e-mail, тег json:"-"), чим перевірено (go test ./internal/app/ -run Auth). -->

**2.2. Middleware `Auth` і `RequireRole`.** На вхід: `internal/middleware/*.go`, `internal/app/app.go`,
`internal/auth/` з 2.1. Результат: `internal/middleware/auth.go`, `internal/middleware/rbac.go`,
оновлені `cors.go` і `app.go`. Ключові вимоги:

- `Auth`: токен із `Authorization: Bearer <token>`, лише HS256 (інший `alg` відхиляється), перевірка
  `exp`, `user_id` і `role` в контексті з неекспортованим типом ключа і типізованими акцесорами;
  помилка → `401` `unauthenticated`;
- `RequireRole(allowed ...string)`: `401` без аутентифікованого користувача, `403` `forbidden` для
  недозволеної ролі; завжди після `Auth`;
- реєстрація маршрутів в `app.go` хелперами `protected`, `staff`, `admin`
  (`mux.Handle("POST /api/v1/albums", staff(http.HandlerFunc(albums.Create)))`);
- переривання: `handler.WriteError` + `return` без `next.ServeHTTP`;
- `CORS` перелічує `Authorization` в `Access-Control-Allow-Headers`.

<details>
<summary>Повний текст промпту 2.2</summary>

```text
TODO: вставити текст промпту з prompts.txt
```

</details>

Аналіз результату: <!-- TODO: що згенеровано, що виправлено, чим перевірено (go test ./internal/app/ -run RBAC, go test ./internal/middleware). -->

### Частина 3. AI-аудит безпеки та тести

**3.1. Security-рев'ю.** Агенту передано `internal/auth`, `internal/middleware`,
`internal/handler/auth_handler.go`, `user_handler.go`, `album_handler.go`, `copy_handler.go` та
`internal/repository` з інструкцією:

> Review this Go net/http API code for security vulnerabilities. Check specifically for: 1) SQL injection
> (raw queries, string concatenation); 2) Mass assignment (binding request JSON directly to DB models);
> 3) Missing authentication or RBAC checks; 4) Sensitive data exposure (leaking password hashes in JSON
> responses, secrets or tokens in logs). Output a structured list with line numbers, severity
> (Low/Medium/High), and proposed fixes. Do not rewrite files completely.

Повний звіт із тріажем: [security_audit.md](security_audit.md). Підсумок:

| № | Знахідка | Файл | Серйозність | Тріаж |
|---|---|---|---|---|
| 1 | <!-- TODO --> | <!-- TODO: шлях:рядок --> | <!-- TODO: High / Medium / Low --> | <!-- TODO: Fixed Now / Tracked for Later / False Positive --> |
| 2 | <!-- TODO --> | | | |
| 3 | <!-- TODO --> | | | |
| 4 | <!-- TODO --> | | | |
| 5 | <!-- TODO --> | | | |

<!-- TODO: щонайменше 5 знахідок. Для Fixed Now вкажіть, де виправлено; для False Positive — чому це не вразливість; для Tracked for Later — що і коли. -->

**3.2. Табличні тести безпеки.** Тести вже були в репозиторії й до реалізації падали на `404`
для відсутніх маршрутів:

```text
TODO: 3-4 рядки з go test ./internal/app/ -run 'Auth|RBAC' -v до реалізації (--- FAIL ... expected 201, got 404 ...)
```

| Тест | Що перевіряє |
|---|---|
| `TestAuthRegisterAndLogin` | `201` з акаунтом без хеша; дубль e-mail, порожні поля, тіло не JSON → `422`; логін видає HS256-токен з `user_id`, `role` і `exp` ≈ 24 год; хибний пароль і невідомий e-mail → узагальнений `401`; відсутній пароль → `422` |
| `TestAuthLoginFailuresAreIndistinguishable` | тіла `401` для невідомого e-mail і хибного пароля побайтово однакові |
| `TestAuthTokenValidation` | без заголовка, не Bearer, порожній, пошкоджений, прострочений, з іншим секретом, зі зміненим payload, з `alg: none` → `401`; валідний → маршрут виконується |
| `TestAuthCORSAllowsAuthorizationHeader` | preflight перелічує `Authorization` в `Access-Control-Allow-Headers` |
| `TestRBACMatrix` | матриця доступу вище для anonymous, `member`, `staff`, `admin`; відхилений запис нічого не створює |
| `TestRBACRoleChange` | після `PUT /users/{id}/role` новий логін дає токен із новою роллю, старий токен зберігає стару |

Після реалізації:

```text
TODO: підсумок go test ./... -race -count=1 з TEST_DATABASE_URL (рядки ok для кожного пакета, кількість тестів)
```

<!-- TODO: що змінено в helpers_test.go (fixtures/call логіняться як staff), у scripts/smoke.sh і workflow;
результат scripts/smoke.sh (N/N); посилання на зелений запуск CI. Якщо таблиці тестів розширено під
власні рішення, перелічіть додані випадки. -->
