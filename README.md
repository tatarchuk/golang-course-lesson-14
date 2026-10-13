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
застосовуються автоматично на старті. Демо-акаунти не мають пароля: першого адміністратора створює
команда `bootstrap` (крок 4).

```sh
# 1. Клонувати репозиторій
git clone https://github.com/tatarchuk/golang-course-lesson-9.git && cd golang-course-lesson-9

# 2. Конфігурація: значення збігаються зі стандартними в docker-compose.yml. JWT_SECRET у
#    .env.example - лише заготовка, сервер її відхилить: згенеруйте свій (Linux: sed -i без '')
cp .env.example .env
sed -i '' "s|^JWT_SECRET=.*|JWT_SECRET=$(openssl rand -base64 48)|" .env

# 3. Зібрати образ і запустити PostgreSQL + API (http://localhost:8080)
docker compose up -d --build

# 4. Дати адміністраторові пароль (ADMIN_PASSWORD з .env; те саме з хоста: make bootstrap)
docker compose exec -e ADMIN_PASSWORD=Albums-Dev-2026 api /app/bootstrap -email admin@albums.local

# 5. Перевірити
curl -s localhost:8080/health                      # {"status":"ok"}
curl -s 'localhost:8080/api/v1/albums?limit=2'     # каталог читається без токена
curl -s -X POST localhost:8080/api/v1/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"admin@albums.local","password":"Albums-Dev-2026"}'   # {"token":"<jwt>"}
docker compose logs -f api                         # кожен SQL-запит потрапляє в лог (без значень)
docker compose down                                # зупинити; дані лишаються у volume pgdata
```

Те саме через `make up`, `make bootstrap`, `make logs`, `make down` (усі цілі: `make` без аргументів
показує Makefile). Без `JWT_SECRET` стек не стартує: `docker-compose.yml` навмисно не має для нього
типового значення, а сервер відмовляється від секрету, коротшого за 32 байти, і від заготовки
`change-me…` з `.env.example` (`validateSecret` у `cmd/api/main.go`). PostgreSQL опубліковано лише
на `127.0.0.1`.

Скриншоти працюючого стека з Теми 8: [Swagger UI з виконаним `GET /api/v1/albums` і відповіддю 200](docs/screenshots/swagger-get-albums-200.jpg)
та [огляд API у Swagger UI](docs/screenshots/swagger-overview.jpg). Для Теми 9 замість скриншотів
збережено текстові докази: [docs/auth/token-flow.txt](docs/auth/token-flow.txt) (повний сценарій з
токенами) і [docs/auth/smoke.txt](docs/auth/smoke.txt) (78 перевірок проти стека).

### Запуск на хості

Потрібен Go 1.25.11 або новіший (версія з `go.mod`). База береться з compose, сервер запускається
локально і так само застосовує міграції на старті.

```sh
docker compose up -d postgres        # або make db
set -a; . ./.env; set +a             # DATABASE_URL, JWT_SECRET, MIGRATIONS_PATH, PORT, ADMIN_*
make bootstrap                       # go run ./cmd/bootstrap -email $ADMIN_EMAIL (пароль з ADMIN_PASSWORD)
go run ./cmd/api                     # або make run
```

| Змінна | Призначення | Типово |
|---|---|---|
| `DATABASE_URL` | рядок підключення PostgreSQL, обов'язкова | — |
| `JWT_SECRET` | секрет підпису JWT (HS256), обов'язкова: щонайменше 32 байти, заготовка `change-me…` з `.env.example` відхиляється (`openssl rand -base64 48`) | — |
| `MIGRATIONS_PATH` | тека з міграціями golang-migrate | `migrations` |
| `PORT` | порт HTTP-сервера | `8080` |
| `DB_LOG_LEVEL` | логер GORM: `info` (кожен запит), `warn`, `error`, `silent` | `info` |
| `ADMIN_EMAIL`, `ADMIN_PASSWORD` | акаунт, який створює або скидає `make bootstrap` (лише для `cmd/bootstrap`) | `admin@albums.local`, — |

### Тести та перевірки

```sh
docker compose up -d postgres                 # базу albums_test створює scripts/initdb на першому старті
go test ./internal/app/ -run 'Auth|RBAC' -v   # приймальні тести Теми 9
make test                                     # go test ./... -race -count=1 з TEST_DATABASE_URL із .env
go test ./internal/model ./internal/handler ./internal/middleware ./internal/auth   # без бази даних
gofmt -l . && go vet ./...
scripts/smoke.sh http://localhost:8080        # повтор API smoke-тесту CI проти запущеного стека (після bootstrap)
```

Тести з базою (`internal/repository`, `internal/app`, `cmd/bootstrap`) читають `TEST_DATABASE_URL`;
без неї вони пропускаються, а в CI змінна задана, тож пропусків немає. `JWT_SECRET` тести задають самі
(`t.Setenv` перед побудовою роутера), тому для `make test` він у `.env` не потрібен. Пакети, що
працюють із базою, серіалізуються advisory-lock'ом (`internal/testdb`), кожен тест починає з порожніх
таблиць (включно з `users`, тому seed-акаунти в тестах відсутні).

- `internal/model`: валідація вхідних структур, `null` для необов'язкових полів, хибні JSON-типи;
  Тема 9: `RegisterInput` (порожні поля, пароль 8..72 байти), `LoginInput`, `RoleInput` (роль поза
  списком), `NormalizeEmail`, `TestUserJSONHidesPasswordHash` (хеш не потрапляє в JSON).
- `internal/auth`: хешування та перевірка пароля (порожній і фіктивний хеш ніколи не збігаються),
  видача токена, відхилення токена без `exp`, простроченого, з іншим секретом, зі зміненим payload,
  з `alg: none`, з `user_id` 0; токен у формі, яку чеканять приймальні тести; користувач у контексті.
- `internal/repository`: інтеграційні тести з реальною БД: створення зі зв'язками, невідомий жанр або
  виконавець (`invalid_reference`), дублікати (`conflict`), список із фільтром і пагінацією, повна
  заміна у `PUT`, каскад при видаленні, примірники та доступність, ідемпотентність міграцій,
  guard-тест `TestListQueryCountIsConstant` на N+1; Тема 9: `UserRepository` (дубль e-mail →
  `ErrConflict`, невідомий id → `ErrNotFound`, роль поза `CHECK` → `ErrInvalidInput`, upsert
  адміністратора двічі) і `TestMigrationAddsPasswordHashColumn` (колонка `NOT NULL DEFAULT ''`).
- `internal/handler`: обробники з фейковими репозиторіями: порядок перевірок, коди помилок, парсинг;
  Тема 9: реєстрація (нормалізація e-mail, bcrypt-хеш у сховищі, `422` для дубля з повідомленням
  `email already registered`, жодного хеша у відповіді), логін (однакове тіло `401` для хибного
  пароля, невідомого e-mail і акаунта без пароля, причому останній теж коштує одне bcrypt-порівняння),
  `GET /me` (`401` для видаленого акаунта), список акаунтів і зміна ролі (`400` → `422` → `404`).
- `internal/middleware`: паніка → 500, CORS (з `Authorization`), логування; Тема 9: `Auth` (сім видів
  хибних облікових даних → `401`, обробник не викликається; схема `bearer` будь-яким регістром),
  `RequireRole` (`401` без користувача, `403` для `member`, пропуск `staff`/`admin`).
- `internal/app`: e2e через `NewRouter()` і реальну БД: сценарій smoke-тесту CI, граничні випадки
  контракту Теми 7 і приймальні тести Теми 9: `auth_test.go` (`TestAuthRegisterAndLogin`,
  `TestAuthLoginFailuresAreIndistinguishable`, `TestAuthTokenValidation`,
  `TestAuthCORSAllowsAuthorizationHeader`) та `rbac_test.go` (`TestRBACMatrix`, `TestRBACRoleChange`);
  `TestNewRouterPanicsWithoutSecret` фіксує fail fast без секрету.
- `cmd/bootstrap`: команда створює адміністратора, повторний запуск скидає пароль того самого рядка;
  без пароля або `DATABASE_URL` завершується помилкою.

CI ([.github/workflows/theme8-check.yml](.github/workflows/theme8-check.yml), тепер «Theme 8 + Theme 9
check») перевіряє структуру файлів, статичні правила (додано: `json:"-"` на `PasswordHash`,
`RegisterInput` без поля `role`, залежності `jwt/v5` і `x/crypto`, `JWT_SECRET` у `.env.example` і
`docker-compose.yml`, `RequireRole` всередині `Auth` в `app.go`), міграції на PostgreSQL, тести з
реальною БД і smoke-тест API. Smoke-тест CI, як і `scripts/smoke.sh`, спершу створює адміністратора
командою `cmd/bootstrap`, проходить сценарій Теми 9 (401 → реєстрація → 403 для `member` → логін
адміністратора → підвищення ролі) і далі виконує каталожні перевірки Теми 8 з токеном адміністратора;
окрема перевірка стежить, щоб у лозі сервера не було bcrypt-хешів. Локальний повтор:
`scripts/smoke.sh` дає 78/78 ([docs/auth/smoke.txt](docs/auth/smoke.txt)). Workflow запускається на кожен push і pull request; результат — у вкладці Actions репозиторію.

### Документація API

- Swagger UI із запуском запитів з браузера: <http://localhost:8080/swagger/index.html>; для захищених
  маршрутів кнопка **Authorize** приймає рядок `Bearer <token>` (схема `BearerAuth`,
  `@securityDefinitions.apikey`), після чого Swagger додає заголовок `Authorization` до запитів.
- Специфікація OpenAPI: [docs/swagger.yaml](docs/swagger.yaml), [docs/swagger.json](docs/swagger.json).
- Runbook із `curl`-запитами на всі випадки контракту, включно з отриманням токена та випадками
  `401`/`403`: [docs/CURL.md](docs/CURL.md).

Специфікацію згенеровано утилітою [swag](https://github.com/swaggo/swag) з анотацій у `cmd/api/main.go`
та обробниках `internal/handler/*_handler.go`; після їх зміни: `make swagger`. Усі анотації лишаються
ASCII, бо CI перевіряє і згенерований `docs/docs.go`.

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
claims `user_id`, `role`, `exp` (24 години) та `iat`. Вхідні структури навмисно вузькі, щоб mass
assignment був неможливий:

| Запит | Тіло | Примітка |
|---|---|---|
| `POST /auth/register`, `POST /auth/login` | `{"email","password"}` | обидва обов'язкові й не порожні; e-mail нормалізується (пробіли, регістр), має вигляд `name@domain` і не довший за 254 байти; пароль при реєстрації 8..72 байти; поля `role` у вході немає: новий акаунт завжди `member` |
| `PUT /users/{id}/role` | `{"role"}` | `member` \| `staff` \| `admin`, інакше `422` `validation_error` |

### Ендпоінти

| Запит | Доступ | Відповідь |
|---|---|---|
| `GET /health` | публічний | `200` `{"status":"ok"}` |
| `POST /api/v1/auth/register` | публічний | `201` акаунт із `role: "member"`; порожні або відсутні `email`/`password`, e-mail не у формі `name@domain`, закороткий пароль, тіло не JSON → `422` `validation_error`; зайнятий e-mail → `422` `validation_error` з повідомленням `email already registered` |
| `POST /api/v1/auth/login` | публічний | `200` `{"token":"<jwt>"}`; невідомий e-mail, хибний пароль і акаунт без пароля → той самий `401` `unauthenticated` `invalid credentials`; відсутній пароль → `422` |
| `GET /api/v1/me` | будь-який аутентифікований | `200` акаунт власника токена; без токена або якщо акаунт видалено → `401` |
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
  і заголовків `Content-Type, Authorization`; preflight обробляється до перевірки токена.

## Структура проєкту та прийняті рішення

```
cmd/api/main.go                      конфігурація з env (JWT_SECRET читається один раз, порожній -> fail fast), міграції, HTTP-сервер, graceful shutdown
cmd/bootstrap/main.go                створення або скидання адміністратора: bcrypt-хеш + upsert з роллю admin (єдиний спосіб дати перший пароль)
internal/app/app.go                  NewRouter(repos, cfg): маршрути за групами public / protected / staff / admin -> ServeMux -> Logging(Recover(CORS))
internal/auth/                       bcrypt-хешування (DummyHash для невідомих акаунтів), видача та перевірка HS256 JWT, користувач у контексті запиту
internal/model/                      GORM-моделі Album, Artist, Genre, Copy, User (PasswordHash json:"-"), AlbumArtist + вхідні структури з Validate()
internal/repository/                 інтерфейси та GORM-реалізації (+ user_repository.go), errors.go, db.go (Open), migrate.go (Migrate)
internal/handler/                    HTTP-обробники (*_handler.go, + auth_handler.go, user_handler.go), errors.go (коди, відображення помилок), парсинг
internal/middleware/                 Logging, Recover, CORS (+ Authorization), Auth, RequireRole
internal/testdb/                     спільне підключення для тестів із БД (advisory lock, міграції, TRUNCATE)
migrations/                          000001_init_schema, 000002_seed_demo_data, 000003_add_auth_fields (up/down)
docs/                                OpenAPI (swag, BearerAuth), runbook з curl, ER-діаграма, докази N+1 Теми 8, docs/auth/ з доказами Теми 9
scripts/                             initdb/ (бази для тестів), smoke.sh (smoke-тест CI з логіном), measure_n1.sh (N+1), render_er.sh
Dockerfile, docker-compose.yml       образ API з обома бінарниками (/app/api, /app/bootstrap) і стек postgres + api (JWT_SECRET обов'язковий)
Makefile, .env.example               щоденні команди (+ bootstrap) та приклад конфігурації (з JWT_SECRET, ADMIN_EMAIL, ADMIN_PASSWORD)
albums_lesson9_homework.md           умова Теми 9, адаптована під варіант albums
prompts.txt                          промпти завдань 2.1, 2.2 і 3.1 з нотатками про результат
security_audit.md                    AI-аудит безпеки: знахідки, серйозність, тріаж
.github/workflows/theme8-check.yml   CI «Theme 8 + Theme 9 check»: структура, міграції, тести, smoke-тест з аутентифікацією
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
  логер GORM на рівні `info` пише кожен SQL-запит у stdout. Після аудиту Теми 9 запити логуються з
  плейсхолдерами (`ParameterizedQueries`), тож кількість `SELECT` рахується так само, а значення
  (e-mail, хеші) у лог не потрапляють.

Рішення Теми 9:

- **Секрет JWT.** `cmd/api/main.go:41-44` читає `JWT_SECRET` один раз на старті й завершується з
  помилкою, якщо він порожній, коротший за 32 байти або є заготовкою з `.env.example`
  (`validateSecret`, `TestValidateSecret`); роутер отримує його явно через `app.Config{JWTSecret}`
  (`internal/app/app.go:17-19`) і додатково панікує на порожнє значення (`app.go:28`,
  `TestNewRouterPanicsWithoutSecret`). Package-level змінних немає, тому тести задають секрет через
  `t.Setenv` і будують роутер після цього.
- **Паролі.** `bcrypt.GenerateFromPassword` з `bcrypt.DefaultCost` (10). Логін
  (`internal/handler/auth_handler.go:84-98`) викликає `bcrypt.CompareHashAndPassword` завжди: для
  невідомого e-mail і для акаунта з порожнім хешем (seed-акаунти до bootstrap) — проти `auth.DummyHash`,
  справжнього bcrypt-хеша випадкового рядка. Інакше bcrypt миттєво відкидав би порожній хеш і час
  відповіді видавав би такі акаунти. Тіло `401` в усіх випадках побайтово однакове.
- **Токен.** Лише HS256 (`jwt.WithValidMethods`): будь-який інший `alg` (зокрема `none`), хибний
  підпис, змінений payload, прострочений або відсутній `exp` (`jwt.WithExpirationRequired`),
  `user_id <= 0` чи порожня роль → `401`. Claims `user_id`, `role`, `exp` = 24 години, `iat`. Токен
  без стану: роль береться з claim, тому нова роль діє з наступного логіну.
- **Контекст.** `user_id` і `role` кладуться в контекст запиту через `context.WithValue` з
  неекспортованим типом ключа (`internal/auth/context.go`) і читаються типізованими акцесорами
  (`auth.UserFromContext(ctx)`); пакет `auth` не імпортує ні `handler`, ні `middleware`.
- **Ланцюжок middleware.** Групи маршрутів у `net/http` — хелпери в `app.go:37-43`:
  `protected := middleware.Auth(secret)`, `staff := protected(RequireRole("staff","admin")(h))`,
  `admin := protected(RequireRole("admin")(h))`, застосовані до кожного маршруту (`app.go:61-73`).
  `Auth` завжди виконується перед `RequireRole`; невдача пише відповідь через `handler.WriteError` і
  робить `return` без `next.ServeHTTP` (`internal/middleware/auth.go:20-28`, `rbac.go:20-27`), тож
  обгорнутий обробник не виконується, а відхилений запис нічого не створює.
- **DTO.** `RegisterInput` і `LoginInput` містять лише `email` та `password`, `RoleInput` — лише
  `role` (`internal/model/user.go:68-111`); обробники декодують запити лише в них, ніколи в
  `model.User`; `User.PasswordHash` має тег `json:"-"` (`user.go:49`), тож хеш не серіалізується
  жодним шляхом.
- **Логування** без змін: метод, шлях, статус, тривалість (`internal/middleware/logging.go`); жодних
  тіл запитів і заголовка `Authorization`.
- **Стартовий адміністратор:** міграція не зберігає хешів; перший пароль задає `cmd/bootstrap`
  (розділ [Стартовий адміністратор](#стартовий-адміністратор)).

## Lesson 9 Report: аутентифікація, JWT та RBAC

Умова: [albums_lesson9_homework.md](albums_lesson9_homework.md). Стан виконання:

| Частина | Що потрібно | Стан |
|---|---|---|
| 1. Вручну | ролі й матриця доступу, DTO без mass assignment; аудит логіну, логування та переривання middleware | готово |
| 2. Промпт-інжиніринг | промпти для auth-модуля (міграція `000003`, модель `User`, `Register`/`Login`, JWT) і для middleware (`Auth`, `RequireRole`, групи маршрутів, CORS); [prompts.txt](prompts.txt) | готово |
| 3. AI-агент | security-рев'ю → [security_audit.md](security_audit.md) (знахідки з тріажем); `auth_test.go` і `rbac_test.go` зелені, `go test ./... -race` зелений | готово |

### Частина 1. Ролі, матриця доступу та DTO

Цей розділ закриває завдання 1.1 (замість окремого `security_design.txt`). Ролі ті, що вже є в схемі
(`CHECK (role IN ('member','staff','admin'))` у
[migrations/000001_init_schema.up.sql](migrations/000001_init_schema.up.sql)):

| Роль | В умові | Хто це | Що може |
|---|---|---|---|
| `member` | reader | типова роль нового акаунта | читати каталог, бачити свій акаунт (`GET /me`) |
| `staff` | librarian | персонал каталогу | усе, що `member`, плюс створення, зміна й видалення жанрів, виконавців, альбомів і примірників |
| `admin` | admin | адміністратор | усе, що `staff`, плюс список акаунтів і зміна ролей |

Публічні маршрути (токен не перевіряється): `POST /api/v1/auth/register`, `POST /api/v1/auth/login`,
усі `GET` каталогу (`/genres`, `/genres/{id}`, `/artists`, `/artists/{id}`, `/albums`, `/albums/{id}`,
`/albums/{id}/copies`, `/albums/{id}/availability`), `/health` і `/swagger/`.

Матриця доступу (статус для кожної ролі; anonymous = без токена; саме її перевіряє `TestRBACMatrix`,
44 підтести):

| Маршрут | anonymous | member | staff | admin |
|---|---|---|---|---|
| `GET /api/v1/albums`, `GET /api/v1/genres` та інші `GET` каталогу | 200 | 200 | 200 | 200 |
| `GET /api/v1/me` | 401 | 200 | 200 | 200 |
| `POST /api/v1/genres`, `POST /api/v1/artists`, `POST /api/v1/albums`, `POST /api/v1/albums/{id}/copies` | 401 | 403 | 201 | 201 |
| `PUT /api/v1/albums/{id}` | 401 | 403 | 200 | 200 |
| `DELETE /api/v1/albums/{id}` | 401 | 403 | 204 | 204 |
| `GET /api/v1/users`, `PUT /api/v1/users/{id}/role` | 401 | 403 | 403 | 200 |

Відхилений запис нічого не створює (тест рахує рядки `genres` і `copies` після матриці).

DTO проти mass assignment ([internal/model/user.go](internal/model/user.go)):

| Структура | Поля | Де використовується |
|---|---|---|
| `RegisterInput` | `email`, `password` | `POST /auth/register`; роль задає сервер (`member`), e-mail `name@domain` до 254 байтів, пароль 8..72 байти |
| `LoginInput` | `email`, `password` | `POST /auth/login` |
| `RoleInput` | `role` | `PUT /users/{id}/role`; лише `member`, `staff`, `admin` |
| `User` (модель) | `id`, `email`, `role`, `password_hash` (`json:"-"`), `created_at`, `updated_at` | відповіді; хеш ніколи не серіалізується (`TestUserJSONHidesPasswordHash`) |

Ручний аудит (завдання 1.2), виконаний після генерації коду:

- **Логін без перебору акаунтів.** Невідомий e-mail, хибний пароль і акаунт без пароля дають
  побайтово однакову відповідь `401` `{"error":{"code":"unauthenticated","message":"invalid credentials"}}`;
  `bcrypt.CompareHashAndPassword` виконується в усіх трьох випадках
  (`internal/handler/auth_handler.go:90-98`). Під час рев'ю згенерованого коду знайдено й виправлено
  ваду: для акаунта з порожнім хешем bcrypt повертає помилку миттєво, тож seed-акаунти можна було б
  вирізнити за часом відповіді; тепер такий акаунт порівнюється з `auth.DummyHash`. Доводять
  `TestAuthLoginFailuresAreIndistinguishable` (e2e) і `TestAuthHandlerLogin` (однакові тіла та
  нижня межа часу для акаунта без пароля).
- **Логування.** `internal/middleware/logging.go` не змінювався: пише лише метод, шлях, статус і
  тривалість; тіла запитів і заголовок `Authorization` у лог не потрапляють (перевірено грепом по
  лозі працюючого стека після реєстрації та логіну: 0 збігів для `Authorization` і для початку
  токена `eyJhbGci`).
- **Переривання ланцюжка.** `Auth` і `RequireRole` після помилки викликають `handler.WriteError` і
  повертаються без `next.ServeHTTP` (`internal/middleware/auth.go:20-26`, `rbac.go:20-25`);
  `TestAuthRejectsBadCredentials` і `TestRequireRole` перевіряють прапорець `called`, який лишається
  `false` для кожної відхиленої спроби.

### Як отримати токен

Повний запис цього сценарію з реальними відповідями: [docs/auth/token-flow.txt](docs/auth/token-flow.txt).

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
# 403 {"error":{"code":"forbidden","message":"this action requires role staff or admin"}}
curl -s -X POST $BASE/genres -H 'Content-Type: application/json' -d '{"name":"ambient"}'
# 401 {"error":{"code":"unauthenticated","message":"a bearer token is required"}}

# staff або admin: 201 (адміністратор з розділу «Стартовий адміністратор»)
ADMIN=$(curl -s -X POST $BASE/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"admin@albums.local","password":"Albums-Dev-2026"}' | jq -r .token)
curl -s -X POST $BASE/genres -H "Authorization: Bearer $ADMIN" -H 'Content-Type: application/json' -d '{"name":"ambient"}'
# 201 {"id":4,"name":"ambient",...}

# Адміністратор підвищує alice до staff: новий логін alice видає токен із новою роллю,
# старий токен лишається member до exp
curl -s -X PUT $BASE/users/4/role -H "Authorization: Bearer $ADMIN" -H 'Content-Type: application/json' -d '{"role":"staff"}'
# 200 {"id":4,"email":"alice@example.com","role":"staff",...}
curl -s $BASE/users -H "Authorization: Bearer $ADMIN"
# 200 [{"id":1,"email":"admin@albums.local","role":"admin",...}, ...]
```

### Стартовий адміністратор

Обрано варіант без хешів у міграціях. Міграція
[000003_add_auth_fields](migrations/000003_add_auth_fields.up.sql) додає колонку
`password_hash TEXT NOT NULL DEFAULT ''`; seed-акаунти `admin@albums.local`, `staff@albums.local` і
`member@albums.local` лишаються без пароля і не можуть увійти (порожній хеш ніколи не збігається, а
логін порівнює його з фіктивним хешем, щоб і час відповіді нічого не видавав). Першого адміністратора
створює або скидає команда [cmd/bootstrap](cmd/bootstrap/main.go): вона перевіряє вхід тими самими
правилами, що й реєстрація, застосовує міграції (якщо API ще не застосував), хешує пароль bcrypt і
робить upsert рядка з роллю `admin` (`INSERT ... ON CONFLICT (email) DO UPDATE SET password_hash,
role, updated_at`). Повторний запуск скидає пароль того самого рядка (`TestRunCreatesAndResetsTheAdmin`).

```sh
make bootstrap                                   # на хості: ADMIN_EMAIL і ADMIN_PASSWORD з .env
docker compose exec -e ADMIN_PASSWORD=Albums-Dev-2026 api /app/bootstrap -email admin@albums.local   # у контейнері
```

Пароль передається через змінну середовища `ADMIN_PASSWORD` (прапорець `-password` теж є, але він
лишає пароль в історії оболонки та списку процесів). Dev-пароль `Albums-Dev-2026` задокументовано тут
і в `.env.example`; для будь-якого іншого середовища його треба змінити (`.env` не комітиться).
Команда пише лише `administrator <email> is ready (id N)`, логер GORM у ній на рівні `warn`, тож хеш у
вивід не потрапляє.

| Акаунт | Роль | Пароль |
|---|---|---|
| `admin@albums.local` | `admin` | після `make bootstrap`: `Albums-Dev-2026` (або інший з `ADMIN_PASSWORD`) |
| `staff@albums.local` | `staff` | немає, увійти не може; за потреби `cmd/bootstrap -email staff@albums.local` (стане `admin`) або реєстрація нового акаунта і `PUT /users/{id}/role` |
| `member@albums.local` | `member` | немає, увійти не може |

### Частина 2. Промпти для генерації коду

Промпти складено до генерації і збережено у [prompts.txt](prompts.txt) разом із нотатками про
результат; виконано в цьому репозиторії через Claude Code (Claude Fable 5.1). Увесь код писався
test-first: спершу тести, які падали, потім генерація за промптом, потім ручне рев'ю.

**2.1. Модуль аутентифікації.** На вхід: `internal/model/user.go`, міграції `000001` і `000002`,
`internal/handler/errors.go`, `respond.go`, `request.go`, інтерфейси `internal/repository`. Результат:
`migrations/000003_add_auth_fields.{up,down}.sql`, оновлений `internal/model/user.go`, `internal/auth/`,
`internal/repository/user_repository.go` (+ `Users` у `Repositories`), `internal/handler/auth_handler.go`
і `user_handler.go`, два нові коди помилок, `cmd/api/main.go`, `.env.example`, `docker-compose.yml`.
Ключові вимоги, закладені в промпт:

- міграція додає лише `password_hash` (колонка `role` уже є) і не зберігає хешів; down-міграція
  прибирає колонку;
- реєстрація: `bcrypt.GenerateFromPassword` з `bcrypt.DefaultCost`, роль `member`, `201` без хеша;
  зайнятий e-mail → `422` `validation_error` `email already registered`;
- логін: `bcrypt.CompareHashAndPassword` і для невідомого e-mail чи порожнього хеша (проти
  `DummyHash`), `200 {"token"}`, HS256 через `github.com/golang-jwt/jwt/v5`, однакова `401`-відповідь
  для всіх невдач;
- claims `user_id`, `role`, `exp` (24 год), `iat`; секрет лише з `JWT_SECRET`, зчитаний один раз у
  `main.go` (fail fast) і переданий роутеру через `app.Config`, без package-level змінної;
- `JWT_SECRET` у `.env.example` і `docker-compose.yml`; анотації Swagger
  (`@securityDefinitions.apikey BearerAuth` останнім блоком над `main`, `@Security BearerAuth`);
- конверт помилок проєкту та наявні хелпери (`readInput`, `pathID`, `WriteError`, `writeRepoError`).

<details>
<summary>Повний текст промпту 2.1</summary>

```text
You are a senior Go engineer working in the repository of "Albums API" (Go module `homework`, Go 1.25,
net/http ServeMux with method patterns, GORM v1.31 + pgx on PostgreSQL 16, golang-migrate reading SQL
files from `migrations/`). Generate the authentication module of Lesson 9. Follow the existing code
conventions exactly: every type and function has a full-sentence comment starting with its name;
ASCII-only source; imports in three groups (stdlib, third-party, `homework/...`); request structs are
`XInput` with a value-receiver `Validate() error` built from `requiredProblem(field{...})` and
`validationError(problems)` in `internal/model/validate.go`; repositories are `gormXRepository{db}`
with a compile-time interface check and `translate(err)` from `internal/repository/errors.go`
(unique violation -> ErrConflict, CHECK violation -> ErrInvalidInput, missing row -> ErrNotFound);
handlers use `readInput(w, r, &in)` (writes 422 on bad JSON or Validate error), `pathID(w, r)`
(writes 400 invalid_id), `WriteJSON`, `WriteError(w, status, code, message)` and
`writeRepoError(w, err, "User")`; error envelope `{"error":{"code":"...","message":"..."}}`.

Produce these files and nothing else:

1. `migrations/000003_add_auth_fields.up.sql` and `.down.sql`. The `users` table already has
   id, email (UNIQUE uq_users_email), role (CHECK chk_users_role IN member/staff/admin), timestamps.
   The up migration adds ONLY `password_hash TEXT NOT NULL DEFAULT ''` (IF NOT EXISTS); the down
   migration drops it (IF EXISTS). Do not store any hash in a migration: seeded accounts keep '' and
   cannot log in until `cmd/bootstrap` sets a password. Start each file with a `-- ` comment saying so.

2. `internal/model/user.go`: add `PasswordHash string` with tags `json:"-" gorm:"column:password_hash"`
   (never serialised); constants MinPasswordLength = 8, MaxPasswordLength = 72 (bcrypt limit);
   `ValidRole(role string) bool`; `NormalizeEmail(s string) string` (TrimSpace + ToLower);
   `RegisterInput{Email, Password}` (both required; password 8..72 bytes; NO role field);
   `LoginInput{Email, Password}` (both required; password <= 72 bytes); `RoleInput{Role}` (required,
   must satisfy ValidRole). Tags: `json:"email" validate:"required" example:"reader@albums.local"`,
   `json:"password" validate:"required" example:"Correct-Horse-Battery-1"`, `json:"role" validate:"required" example:"staff"`.

3. `internal/auth/` (new package; it must import neither `handler` nor `middleware`):
   - `doc.go`: package comment.
   - `password.go`: `HashPassword(password string) (string, error)` using
     `bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)`; `CheckPassword(hash, password string) bool`
     (`bcrypt.CompareHashAndPassword == nil`; an empty or malformed hash never matches); an exported
     constant `DummyHash` holding a real cost-10 bcrypt hash of a random string, documented as the hash
     login compares against when the e-mail is unknown so response time does not reveal accounts.
   - `token.go`: `const TokenTTL = 24 * time.Hour`; `type Claims struct{ UserID int64; Role string }`;
     `var ErrInvalidToken`; `IssueToken(secret string, userID int64, role string, ttl time.Duration) (string, error)`
     signing `jwt.SigningMethodHS256` (github.com/golang-jwt/jwt/v5) with claims `user_id`, `role`,
     `exp`, `iat` (use a struct embedding jwt.RegisteredClaims so exp/iat are numeric dates);
     `ParseToken(secret, raw string) (Claims, error)` using `jwt.ParseWithClaims` with
     `jwt.WithValidMethods([]string{"HS256"})` and `jwt.WithExpirationRequired()`; any failure
     (malformed, expired, bad signature, alg none, user_id <= 0, empty role) returns ErrInvalidToken.
   - `context.go`: unexported key type; `WithUser(ctx, Claims) context.Context`;
     `UserFromContext(ctx) (Claims, bool)`.

4. `internal/repository/user_repository.go`: interface `UserRepository` with
   `Create(ctx, email, passwordHash string) (model.User, error)` (role member; duplicate -> ErrConflict),
   `Get(ctx, id int64)`, `GetByEmail(ctx, email string)`, `List(ctx) ([]model.User, error)` (ordered by id,
   empty slice not nil), `UpdateRole(ctx, id int64, role string) (model.User, error)` (missing -> ErrNotFound;
   re-read the row after the update), `UpsertAdmin(ctx, email, passwordHash string) (model.User, error)`
   (INSERT ... ON CONFLICT (email) DO UPDATE SET password_hash, role = 'admin', updated_at = now(), via
   gorm clause.OnConflict). Add `Users UserRepository` to `Repositories` and wire `&gormUserRepository{db: db}`
   in `NewGorm` (internal/repository/repository.go).

5. `internal/handler/errors.go`: add `CodeUnauthenticated = "unauthenticated"` and `CodeForbidden = "forbidden"`.

6. `internal/handler/auth_handler.go`: `AuthHandler{users repository.UserRepository; secret string}`,
   `NewAuthHandler(users, secret)`, `TokenResponse{Token string json:"token"}`.
   - `Register` (POST /api/v1/auth/register): readInput RegisterInput -> HashPassword -> users.Create with
     NormalizeEmail(in.Email); repository.ErrConflict -> `WriteError(w, 422, CodeValidation, "email already registered")`;
     other errors -> writeRepoError; success 201 with the user (the hash is hidden by json:"-").
   - `Login` (POST /api/v1/auth/login): readInput LoginInput -> users.GetByEmail(NormalizeEmail); when the
     account is unknown OR its stored hash is empty (an account that never got a password) compare the
     password against auth.DummyHash anyway, so every failure costs one bcrypt comparison; unknown e-mail,
     empty hash and wrong password all answer `WriteError(w, 401, CodeUnauthenticated, "invalid credentials")`
     with identical bodies; a repository error other than ErrNotFound -> writeRepoError; success 200
     TokenResponse with IssueToken(secret, user.ID, user.Role, auth.TokenTTL).
   - `Me` (GET /api/v1/me): auth.UserFromContext -> users.Get(claims.UserID) -> 200 user; no claims -> 401;
     ErrNotFound (the account was deleted after the token was issued) -> 401 unauthenticated, not 404.
   Swag annotations in the repository style (`@Summary`, `@Tags auth`, `@Accept`, `@Produce`, `@Param`,
   `@Success`, `@Failure`, `@Router /auth/register [post]`); `Me` gets `@Security BearerAuth`.

7. `internal/handler/user_handler.go`: `UserHandler{users}`, `NewUserHandler(users)`;
   `List` (GET /api/v1/users) -> 200 array; `UpdateRole` (PUT /api/v1/users/{id}/role): pathID (400) ->
   readInput RoleInput (422) -> users.UpdateRole (404 "User not found") -> 200 updated user.
   Both annotated with `@Security BearerAuth` and `@Tags users`.

8. `cmd/api/main.go`: read `JWT_SECRET` once with os.Getenv at the top of run(); return
   `errors.New("JWT_SECRET is required: a long random string that signs login tokens")` when empty;
   pass it to the router as `app.Config{JWTSecret: secret}` (the router constructor becomes
   `app.NewRouter(repos repository.Repositories, cfg app.Config)` and panics when cfg.JWTSecret is empty).
   Add the swag general annotations as the LAST lines of the comment block above main(), after @BasePath
   (swag's security parser consumes every line that follows it):
     // @securityDefinitions.apikey  BearerAuth
     // @in                          header
     // @name                        Authorization
     // @description                 Type "Bearer " followed by the token from POST /auth/login.
   All annotations, descriptions and example tags stay ASCII (CI scans the generated docs/docs.go).

9. `.env.example`: add JWT_SECRET (a dev-only value with a comment), ADMIN_EMAIL=admin@albums.local and
   ADMIN_PASSWORD=Albums-Dev-2026 (used by `make bootstrap`). `docker-compose.yml`: pass
   `JWT_SECRET: ${JWT_SECRET:?set JWT_SECRET in .env}` to the api service.

Do not touch internal/app/app.go or internal/middleware (a second prompt covers them). Do not add
tests (they are written separately). Output each file completely, no placeholders.
```

</details>

Аналіз результату: згенеровано всі перелічені файли в конвенціях проєкту; під час рев'ю (завдання
1.2) уточнено дві речі, і вони зафіксовані тестами: акаунт із порожнім хешем теж порівнюється з
`DummyHash` (інакше seed-акаунти вирізнялися б за часом), а `GET /me` для видаленого акаунта відповідає
`401`, а не `404`. Залежності закріплено на `golang.org/x/crypto v0.53.0` і `jwt/v5 v5.3.1`, бо
`@latest` піднімав директиву `go` до 1.26. Перевірено: `go test ./internal/model ./internal/auth
./internal/repository ./internal/handler -race` і після промпту 2.2 — `go test ./internal/app/ -run 'Auth|RBAC' -v`.

**2.2. Middleware `Auth` і `RequireRole`.** На вхід: `internal/middleware/*.go`, `internal/app/app.go`,
`internal/auth/` з 2.1. Результат: `internal/middleware/auth.go`, `internal/middleware/rbac.go`,
оновлені `cors.go`, `doc.go` і `app.go`, `@Security BearerAuth` на шести обробниках запису. Ключові вимоги:

- `Auth`: токен із `Authorization: Bearer <token>`, лише HS256 (інший `alg` відхиляється), перевірка
  `exp`, `user_id` і `role` в контексті з неекспортованим типом ключа і типізованими акцесорами;
  помилка → `401` `unauthenticated`;
- `RequireRole(allowed ...string)`: `401` без аутентифікованого користувача, `403` `forbidden` для
  недозволеної ролі; завжди після `Auth`;
- реєстрація маршрутів в `app.go` хелперами `protected`, `staff`, `admin`
  (`mux.Handle("POST /api/v1/albums", staff(albums.Create))`); `Logging(Recover(CORS(mux)))` зовні,
  щоб preflight відповідав до перевірки токена;
- переривання: `handler.WriteError` + `return` без `next.ServeHTTP`;
- `CORS` перелічує `Authorization` в `Access-Control-Allow-Headers`.

<details>
<summary>Повний текст промпту 2.2</summary>

```text
Continue in the same repository ("Albums API", Go module `homework`, net/http ServeMux). The package
`internal/auth` now provides `ParseToken(secret, raw string) (Claims, error)`, `Claims{UserID int64; Role string}`,
`WithUser(ctx, Claims) context.Context` and `UserFromContext(ctx) (Claims, bool)`; `internal/handler`
provides `WriteError(w, status, code, message)`, `CodeUnauthenticated` and `CodeForbidden`;
`internal/middleware` already has `Logging`, `Recover` and `CORS`, all of type `func(http.Handler) http.Handler`,
and it already imports `handler` (so `handler` must never import `middleware`). Same conventions as before
(full-sentence comments, ASCII only, three import groups).

Generate:

1. `internal/middleware/auth.go`: `Auth(secret string) func(http.Handler) http.Handler`. It reads the
   `Authorization` header, requires the scheme `Bearer` (case-insensitive) followed by a non-empty token,
   verifies the token with auth.ParseToken (HS256 only; exp required; any other alg, a bad signature, a
   tampered payload or an expired token are rejected) and calls next with
   `r.WithContext(auth.WithUser(r.Context(), claims))`. On any failure it writes
   `handler.WriteError(w, http.StatusUnauthorized, handler.CodeUnauthenticated, <message>)` and returns
   WITHOUT calling next.ServeHTTP (net/http has no AbortWithStatusJSON; the early return is the abort).
   Messages must not echo the token.

2. `internal/middleware/rbac.go`: `RequireRole(allowed ...string) func(http.Handler) http.Handler`. It reads
   the role with auth.UserFromContext: no claims -> 401 unauthenticated; role not in allowed -> 403
   `forbidden`; otherwise next. It assumes Auth ran first and says so in its comment.

3. `internal/middleware/cors.go`: `Access-Control-Allow-Headers` must list `Content-Type, Authorization`,
   otherwise browsers cannot send the token. Update `doc.go`'s package comment.

4. `internal/app/app.go`: change the constructor to `NewRouter(repos repository.Repositories, cfg Config) http.Handler`
   with `type Config struct{ JWTSecret string }`; panic with a clear message when JWTSecret is empty.
   Build the net/http equivalent of nested route groups with helpers:
     protected := middleware.Auth(cfg.JWTSecret)
     staff := func(h http.HandlerFunc) http.Handler { return protected(middleware.RequireRole(model.UserRoleStaff, model.UserRoleAdmin)(h)) }
     admin := func(h http.HandlerFunc) http.Handler { return protected(middleware.RequireRole(model.UserRoleAdmin)(h)) }
   and register, in clearly commented groups:
   - public: GET /health, POST /api/v1/auth/register, POST /api/v1/auth/login, every GET of the catalogue
     (/genres, /genres/{id}, /artists, /artists/{id}, /albums, /albums/{id}, /albums/{id}/copies,
     /albums/{id}/availability), GET /swagger/, and the "/" JSON 404 fallback;
   - protected (any authenticated user): GET /api/v1/me -> protected(http.HandlerFunc(authn.Me));
   - staff and admin: POST /api/v1/genres, POST /api/v1/artists, POST /api/v1/albums, PUT /api/v1/albums/{id},
     DELETE /api/v1/albums/{id}, POST /api/v1/albums/{id}/copies, e.g.
     mux.Handle("POST /api/v1/albums", staff(albums.Create));
   - admin: GET /api/v1/users, PUT /api/v1/users/{id}/role.
   Auth must always run before RequireRole. Keep the outer chain Logging(Recover(CORS(mux))) so a CORS
   preflight (OPTIONS) is answered before any route-level Auth. Construct the handlers with
   handler.NewAuthHandler(repos.Users, cfg.JWTSecret) and handler.NewUserHandler(repos.Users).

5. Add `@Security BearerAuth` (plus `@Failure 401` and `@Failure 403 {object} ErrorResponse`) to the six
   catalogue write handlers: GenreHandler.Create, ArtistHandler.Create, AlbumHandler.Create/Update/Delete,
   CopyHandler.Create.

Output each file completely. No tests (written separately), no other files.
```

</details>

Аналіз результату: згенерований код не потребував правок; змінено лише тестовий каркас Теми 8, як і
вимагає умова: `newRouter` задає `JWT_SECRET` і передає `app.Config`, `staffCall()` надсилає
staff-токен у 35 каталожних записах, а `expect()` приймає JSON-масив для списків, на що спирається
матриця `TestRBACMatrix`. Перевірено: `go test ./internal/middleware -race`,
`go test ./internal/app/ -run 'Auth|RBAC' -v` (усі зелені), `go test ./... -race -count=1`.

### Частина 3. AI-аудит безпеки та тести

**3.1. Security-рев'ю.** Окремому агентові (свіжий контекст, лише читання) передано `internal/auth`,
`internal/middleware`, `internal/handler/auth_handler.go`, `user_handler.go`, `album_handler.go`,
`copy_handler.go`, `errors.go`, `request.go`, `internal/repository`, `internal/model/user.go`,
`internal/app/app.go`, `cmd/api/main.go`, `cmd/bootstrap/main.go`, `docker-compose.yml`,
`.env.example`, `scripts/smoke.sh` з інструкцією з умови:

> Review this Go net/http API code for security vulnerabilities. Check specifically for: 1) SQL injection
> (raw queries, string concatenation); 2) Mass assignment (binding request JSON directly to DB models);
> 3) Missing authentication or RBAC checks; 4) Sensitive data exposure (leaking password hashes in JSON
> responses, secrets or tokens in logs). Output a structured list with line numbers, severity
> (Low/Medium/High), and proposed fixes. Do not rewrite files completely.

Необроблений звіт агента: [docs/auth/audit-raw.md](docs/auth/audit-raw.md). Повний тріаж з
аргументами: [security_audit.md](security_audit.md). Підсумок:

| № | Знахідка | Серйозність | Статус |
|---|---|---|---|
| 1 | Логер GORM друкував значення параметрів: bcrypt-хеш при реєстрації та bootstrap, e-mail при кожному логіні (`internal/repository/db.go`) | Medium | Fixed Now: `ParameterizedQueries: true`, `TestOpenLogsSQLWithoutValues`, докази [до](docs/auth/log-before-fix.txt) і [після](docs/auth/log-after-fix.txt) |
| 2 | Роль у 24-годинному stateless-токені: пониження або видалення діє лише після `exp`, відкликання немає | Medium | Tracked for Later: так вимагає умова (роль із claim) і `TestRBACMatrix`; план — коротший TTL + refresh, `token_version` |
| 3 | Немає rate limiting на `/auth/login` і `/auth/register`; bcrypt як підсилювач CPU; «адреси» до 1 MiB | Medium | Tracked for Later; зараз e-mail обмежено 254 байтами і формою `name@domain` (`TestEmailShapeAndLength`) |
| 4 | Секрет перевірявся лише на порожність; швидкий старт копіював публічний секрет з `.env.example` | Medium | Fixed Now: `validateSecret` (щонайменше 32 байти, заготовка відхиляється, `TestValidateSecret`), генерація `openssl rand -base64 48`, локальний секрет ротовано |
| 5 | `POST /auth/register` як оракул існування акаунта (`email already registered`) | Low | Tracked for Later: саме це повідомлення й `422` вимагає умова; пом'якшення — лімітер з №3 |
| 6 | Повні JWT у закоміченому `docs/auth/token-flow.txt` | Low | Fixed Now: транскрипт перегенеровано з маскуванням, секрет ротовано, перевірка CI на JWT у документації |
| 7 | PostgreSQL на `0.0.0.0:5432` з типовим паролем | Low | Fixed Now: порт прив'язано до `127.0.0.1` |
| 8 | (ручне рев'ю, до запуску агента) акаунт із порожнім хешем вирізнявся за часом відповіді | Medium | Fixed Now: порівняння з `DummyHash`, `TestAuthHandlerLogin` |

Інформаційні зауваження агента (`nosniff`/`no-store`, `DisallowUnknownFields`, `WWW-Authenticate`)
відкладено як дешеве посилення; очікувані хибні спрацювання (CORS `*` з bearer-токенами, `TRUNCATE` у
тестовому каркасі) агент сам перевірив як безпечні.

**3.2. Табличні тести безпеки.** Тести вже були в репозиторії й до реалізації падали на `404` для
відсутніх маршрутів (повний вивід: [docs/auth/tests-before.txt](docs/auth/tests-before.txt)):

```text
--- FAIL: TestAuthRegisterAndLogin (0.02s)
    auth_test.go:266: expected 201, got 404 {"error":{"code":"not_found","message":"route not found"}}
--- FAIL: TestAuthLoginFailuresAreIndistinguishable (0.01s)
--- FAIL: TestAuthTokenValidation (0.01s)
--- FAIL: TestAuthCORSAllowsAuthorizationHeader (0.01s)
--- FAIL: TestRBACMatrix (0.01s)
--- FAIL: TestRBACRoleChange (0.01s)
FAIL	homework/internal/app	0.825s
```

| Тест | Що перевіряє |
|---|---|
| `TestAuthRegisterAndLogin` | `201` з акаунтом без хеша; дубль e-mail, порожні поля, тіло не JSON → `422`; логін видає HS256-токен з `user_id`, `role` і `exp` ≈ 24 год; хибний пароль і невідомий e-mail → узагальнений `401`; відсутній пароль → `422` |
| `TestAuthLoginFailuresAreIndistinguishable` | тіла `401` для невідомого e-mail і хибного пароля побайтово однакові |
| `TestAuthTokenValidation` | без заголовка, не Bearer, порожній, пошкоджений, прострочений, з іншим секретом, зі зміненим payload, з `alg: none` → `401`; валідний → маршрут виконується |
| `TestAuthCORSAllowsAuthorizationHeader` | preflight перелічує `Authorization` в `Access-Control-Allow-Headers` |
| `TestRBACMatrix` | матриця доступу вище для anonymous, `member`, `staff`, `admin`; відхилений запис нічого не створює |
| `TestRBACRoleChange` | після `PUT /users/{id}/role` новий логін дає токен із новою роллю, старий токен зберігає стару; `422`/`404`/`400` для хибної ролі, невідомого й нечислового id |

Після реалізації та виправлень аудиту ([docs/auth/tests-after.txt](docs/auth/tests-after.txt)):

```text
$ go test ./... -race -count=1
ok  	homework/cmd/api	1.818s
ok  	homework/cmd/bootstrap	4.397s
?   	homework/docs	[no test files]
ok  	homework/internal/app	15.630s
ok  	homework/internal/auth	4.532s
ok  	homework/internal/handler	6.383s
ok  	homework/internal/middleware	1.478s
ok  	homework/internal/model	1.414s
ok  	homework/internal/repository	5.017s
?   	homework/internal/testdb	[no test files]
# top-level tests passed (go test ./... -count=1 -v | grep -c "^--- PASS"): 102
```

Зміни в тестовому каркасі та скриптах: `internal/app/helpers_test.go` (`newRouter` з секретом,
`staffCall`, `expect` для списків), `scripts/smoke.sh` (логін адміністратора, сценарій Теми 9, токен у
кожному записі) і workflow CI (крок `cmd/bootstrap`, сценарій Теми 9, перевірка відсутності хешів у
лозі). Локальний smoke-тест: 78/78 ([docs/auth/smoke.txt](docs/auth/smoke.txt)); міграція `000003`
перевірена як у CI: up → колонка `NOT NULL DEFAULT ''` → down -all → up → версія 3
([docs/auth/migrate.txt](docs/auth/migrate.txt)).
