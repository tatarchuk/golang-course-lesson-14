<!--
Чернетка README Теми 10 (крок 1 плану). Правила:
1. Усе без позначки TODO вже правдиве для коду Теми 9 або зафіксоване умовою albums_lesson10_homework.md.
2. Кожну позначку TODO (HTML-коментар у тексті, "# TODO" у прикладах команд) заповніть або видаліть після
   реалізації відповідного кроку. Перевірка перед здачею: grep -n TODO README.md (має бути порожньо).
-->
# Albums API

REST-сервіс обліку музичних альбомів на Go з PostgreSQL і вебінтерфейсом на `html/template` та HTMX.
Проєкт виконано в межах курсу «Go with GenAI», варіант `albums`. Тема 7 дала REST API зі сховищем у
пам'яті; Тема 8 перевела його на PostgreSQL через GORM і golang-migrate, нормалізувала схему та виправила
N+1; Тема 9 захистила API: реєстрація і логін з `bcrypt` та JWT, рольова авторизація (`member`, `staff`,
`admin`) у вигляді middleware і AI-асистований аудит безпеки; Тема 10 додає серверний рендеринг:
сторінку каталогу з живим пошуком і видаленням рядків через HTMX, вхід через HttpOnly-cookie, а шаблони,
статику й міграції вбудовує в один бінарник через `go:embed`.

Умова Теми 10, адаптована під варіант `albums`: [albums_lesson10_homework.md](albums_lesson10_homework.md).
Звіт про виконання: розділ [Lesson 10 Report](#lesson-10-report-full-stack-go-з-htmx-та-goembed).
Матеріали Теми 9 (умова, промпти, звіт аудиту) лишилися в репозиторії попередньої теми
(<https://github.com/tatarchuk/golang-course-lesson-9>); її докази збережено тут у [docs/auth/](docs/auth/).

## Швидкий запуск (Docker Compose)

Потрібен Docker з Compose v2. PostgreSQL 16 і API піднімаються разом; міграції (схема та демо-дані:
3 жанри, 6 виконавців, 10 альбомів, 24 примірники, 3 акаунти з ролями `admin`, `staff` і `member`)
застосовуються автоматично на старті. Демо-акаунти не мають пароля: першого адміністратора створює
команда `bootstrap` (крок 4).

```sh
# 1. Клонувати репозиторій
git clone https://github.com/tatarchuk/golang-course-lesson-10.git && cd golang-course-lesson-10

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
open http://localhost:8080/catalog                 # TODO: вебкаталог з'являється після кроку 4 плану; вхід: /login
docker compose logs -f api                         # кожен SQL-запит потрапляє в лог (без значень)
docker compose down                                # зупинити; дані лишаються у volume pgdata
```

Те саме через `make up`, `make bootstrap`, `make logs`, `make down` (усі цілі: `make` без аргументів
показує Makefile). Без `JWT_SECRET` стек не стартує: `docker-compose.yml` навмисно не має для нього
типового значення, а сервер відмовляється від секрету, коротшого за 32 байти, і від заготовки
`change-me…` з `.env.example` (`validateSecret` у `cmd/api/main.go`). PostgreSQL опубліковано лише
на `127.0.0.1`.

<!-- TODO (крок 7): скриншоти docs/screenshots/catalog.jpg, catalog-search.jpg, login.jpg і посилання на них;
     одне речення про те, що образ більше не містить теки migrations/ (вбудовані міграції). -->
Скриншоти Теми 8: [Swagger UI з виконаним `GET /api/v1/albums`](docs/screenshots/swagger-get-albums-200.jpg)
та [огляд API](docs/screenshots/swagger-overview.jpg). Текстові докази Теми 9: [docs/auth/token-flow.txt](docs/auth/token-flow.txt)
і [docs/auth/smoke.txt](docs/auth/smoke.txt). Докази Теми 10 збираються в [docs/web/](docs/web/).

### Запуск на хості

Потрібен Go 1.25.11 або новіший (версія з `go.mod`). База береться з compose, сервер запускається
локально і так само застосовує міграції на старті.

```sh
docker compose up -d postgres        # або make db
set -a; . ./.env; set +a             # DATABASE_URL, JWT_SECRET, PORT, ADMIN_*; TODO: + COOKIE_SECURE, MIGRATIONS_PATH необов'язковий
make bootstrap                       # go run ./cmd/bootstrap -email $ADMIN_EMAIL (пароль з ADMIN_PASSWORD)
go run ./cmd/api                     # або make run
# TODO (крок 3): make build -> bin/api, статично злінкований бінарник (CGO_ENABLED=0), що працює з порожньої теки
```

| Змінна | Призначення | Типово |
|---|---|---|
| `DATABASE_URL` | рядок підключення PostgreSQL, обов'язкова | — |
| `JWT_SECRET` | секрет підпису JWT (HS256), обов'язкова: щонайменше 32 байти, заготовка `change-me…` з `.env.example` відхиляється (`openssl rand -base64 48`) | — |
| `MIGRATIONS_PATH` | тека з міграціями golang-migrate <!-- TODO (крок 3): стає необов'язковим перевизначенням, типово — міграції, вбудовані в бінарник --> | `migrations` |
| `COOKIE_SECURE` | <!-- TODO (крок 3): --> прапорець `Secure` на cookie сесії вебінтерфейсу; `true` за TLS-проксі | `false` |
| `PORT` | порт HTTP-сервера | `8080` |
| `DB_LOG_LEVEL` | логер GORM: `info` (кожен запит), `warn`, `error`, `silent` | `info` |
| `ADMIN_EMAIL`, `ADMIN_PASSWORD` | акаунт, який створює або скидає `make bootstrap` (лише для `cmd/bootstrap`) | `admin@albums.local`, — |

### Тести та перевірки

```sh
docker compose up -d postgres                 # базу albums_test створює scripts/initdb на першому старті
go test ./internal/app/ -run Web -v           # приймальні тести Теми 10 (червоні до реалізації: docs/web/tests-before.txt)
go test ./internal/app/ -run 'Auth|RBAC' -v   # приймальні тести Теми 9
make test                                     # go test ./... -race -count=1 з TEST_DATABASE_URL із .env
go test ./internal/model ./internal/handler ./internal/middleware ./internal/auth   # без бази даних
# TODO (крок 5): go test ./internal/handler/web   # тести вебобробників на фейках, без бази
gofmt -l . && go vet ./...
scripts/smoke.sh http://localhost:8080        # повтор API smoke-тесту CI проти запущеного стека (після bootstrap)
```

Тести з базою (`internal/repository`, `internal/app`, `cmd/bootstrap`) читають `TEST_DATABASE_URL`;
без неї вони пропускаються, а в CI змінна задана, тож пропусків немає. `JWT_SECRET` тести задають самі
(`t.Setenv` перед побудовою роутера), тому для `make test` він у `.env` не потрібен. Пакети, що
працюють із базою, серіалізуються advisory-lock'ом (`internal/testdb`), кожен тест починає з порожніх
таблиць (включно з `users`, тому seed-акаунти в тестах відсутні).

- `internal/model`: валідація вхідних структур, `null` для необов'язкових полів, хибні JSON-типи;
  `RegisterInput`, `LoginInput`, `RoleInput`, `NormalizeEmail`, `TestUserJSONHidesPasswordHash`.
- `internal/auth`: хешування та перевірка пароля, видача токена, відхилення токена без `exp`,
  простроченого, з іншим секретом, зі зміненим payload, з `alg: none`, з `user_id` 0; користувач у контексті.
- `internal/repository`: інтеграційні тести з реальною БД: створення зі зв'язками, невідомі посилання
  (`invalid_reference`), дублікати (`conflict`), список із фільтром і пагінацією, повна заміна у `PUT`,
  каскад при видаленні, примірники та доступність, ідемпотентність міграцій, guard-тест
  `TestListQueryCountIsConstant` на N+1, `UserRepository`, `TestMigrationAddsPasswordHashColumn`.
  Тема 10: пошук за назвою або виконавцем без урахування регістру (`ListOptions.Query`: `ILIKE`,
  екранування `%`, `_` і `\`, поєднання з фільтром за жанром і пагінацією, та сама кількість `SELECT`,
  що й без пошуку) і `AvailabilityForAlbums` одним запитом для цілої сторінки (альбоми без примірників
  відсутні в результаті, порожній список id не торкається бази).
- `internal/handler`: обробники з фейковими репозиторіями: порядок перевірок, коди помилок, парсинг;
  реєстрація, логін (однакове тіло `401` для всіх невдач, одне bcrypt-порівняння завжди), `GET /me`,
  список акаунтів і зміна ролі.
  <!-- TODO (крок 5): + internal/handler/web: сторінка каталогу, фрагмент рядків, матриця доступу на видаленні, логін/логаут з cookie. -->
- `internal/middleware`: паніка → 500, CORS (з `Authorization`), логування, `Auth` (сім видів хибних
  облікових даних → `401`), `RequireRole` (`401`/`403`).
  <!-- TODO (крок 4): + CookieAuth: валідна cookie кладе claims у контекст, відсутня чи хибна - анонімний запит; заголовок Authorization ігнорується. -->
- `internal/app`: e2e через `NewRouter()` і реальну БД: сценарій smoke-тесту CI, граничні випадки
  контракту Теми 7, приймальні тести Теми 9 (`auth_test.go`, `rbac_test.go`) і Теми 10 (`web_test.go`:
  `TestWebCatalogPage`, `TestWebAlbumRowsFragment`, `TestWebDeleteRow`, `TestWebLoginAndLogout`,
  `TestWebStaticAssets`; таблиця в розділі [Частина 3](#частина-3-goembed-і-тести));
  `TestNewRouterPanicsWithoutSecret` фіксує fail fast без секрету.
- `cmd/bootstrap`: команда створює адміністратора, повторний запуск скидає пароль того самого рядка;
  без пароля або `DATABASE_URL` завершується помилкою.

CI ([.github/workflows/theme8-check.yml](.github/workflows/theme8-check.yml), «Theme 8 + Theme 9
check») перевіряє структуру файлів, статичні правила, міграції на PostgreSQL, тести з реальною БД і
smoke-тест API з аутентифікацією. <!-- TODO (крок 6): перейменувати на "Theme 8 + 9 + 10 check"; перелічити нові перевірки
(файли web/, лише html/template, без template.HTML, go:embed/ParseFS/fs.Sub, статичний бінарник з порожньої теки, вебмаршрути
в smoke-тесті, cookie-сценарій, N+1 на /catalog) і дати посилання на зелений запуск. -->
Приймальні тести Теми 10 після цього кроку червоні, тож до реалізації workflow очікувано падає на джобі
з тестами.

### Документація API

- Swagger UI із запуском запитів з браузера: <http://localhost:8080/swagger/index.html>; для захищених
  маршрутів кнопка **Authorize** приймає рядок `Bearer <token>` (схема `BearerAuth`), після чого Swagger
  додає заголовок `Authorization` до запитів.
- Специфікація OpenAPI: [docs/swagger.yaml](docs/swagger.yaml), [docs/swagger.json](docs/swagger.json).
- Runbook із `curl`-запитами на всі випадки контракту, включно з отриманням токена та випадками
  `401`/`403`: [docs/CURL.md](docs/CURL.md). <!-- TODO (крок 7): + розділ про вебмаршрути з cookie jar і HX-Request. -->

Специфікацію згенеровано утилітою [swag](https://github.com/swaggo/swag) з анотацій у `cmd/api/main.go`
та обробниках `internal/handler/*_handler.go`; після їх зміни: `make swagger`. Усі анотації лишаються
ASCII, бо CI перевіряє і згенерований `docs/docs.go`. Вебобробники анотацій не мають: вони не є частиною
API `/api/v1`.

## Вебінтерфейс (Тема 10)

<!-- TODO (крок 4): звірити з реалізацією; до того цей розділ описує контракт із albums_lesson10_homework.md. -->
Сторінки рендеряться на сервері з шаблонів `html/template` (`web/templates/`), інтерактивність дає
HTMX (`web/static/htmx.min.js`, зафіксована версія 2.x) без власного JavaScript. Каталог публічний, як і
`GET /api/v1/albums`; кнопки видалення бачать лише `staff` та `admin` після входу через `/login`.

| Запит | Доступ | Відповідь |
|---|---|---|
| `GET /catalog?q=` | публічний | `200` HTML: макет, форма пошуку (`name="q"`), таблиця альбомів (назва, виконавці, жанр, рік, лейбл, примірники `доступно/усього`); `q` фільтрує на сервері за назвою або виконавцем без урахування регістру і повертається в поле |
| `GET /web/albums?q=` | публічний | із заголовком `HX-Request: true` → `200` лише рядки `<tr>` (порожній результат → один `<tr class="empty">`); без заголовка → `303` на `/catalog?q=` |
| `DELETE /web/albums/{id}` | `staff`, `admin` (лише cookie) | `200` з порожнім тілом, альбом і примірники видалено; анонімний → `401`, `member` → `403`, заголовок `Authorization` ігнорується (`401`), невідомий id → `404`, нечисловий → `400`; крос-сайтовий запит → `403` |
| `GET /login` | публічний | `200` форма входу |
| `POST /login` | публічний | form-encoded `email`, `password`; успіх → `303` на `/catalog` і cookie `albums_token`; невідомий e-mail і хибний пароль → однаковий `401` з формою і текстом `invalid credentials` |
| `POST /logout` | будь-хто | `303` на `/catalog`, cookie прострочена |
| `GET /static/app.css`, `GET /static/htmx.min.js` | публічний | `200` із вбудованої файлової системи; невідомий файл → `404`, лістинг теки не віддається |

HTMX-атрибути з умови: поле пошуку `hx-get="/web/albums" hx-trigger="keyup changed delay:300ms"
hx-target="#album-list"`; кнопка видалення `hx-delete="/web/albums/{id}" hx-target="closest tr"
hx-swap="outerHTML" hx-confirm="Delete this album?"`. Обробник видалення відповідає порожнім `200`, а не
`204`: HTMX не виконує swap на `204`, тоді як порожній `200` із `outerHTML` прибирає рядок.

Cookie сесії `albums_token` містить той самий HS256 JWT, що й API, з атрибутами `Path=/`, `Max-Age=86400`,
`HttpOnly`, `SameSite=Lax` і `Secure`, коли `COOKIE_SECURE=true`. API її не приймає (лише
`Authorization: Bearer`), а вебмаршрути не приймають заголовок: дві поверхні аутентифікації не
перетинаються. Докладніше в розділі [Частина 1](#частина-1-архітектура-вебшару-та-аудит-автоекранування).

## API

Усі відповіді, крім `204`, мають `Content-Type: application/json`; ключі в `snake_case`.
Необов'язкові поля завжди присутні у відповіді (`null`, якщо не задані); час у RFC 3339 (UTC, `Z`).
Захищені маршрути приймають заголовок `Authorization: Bearer <token>`; перевірка доступу виконується
до перевірок запиту: без токена або з хибним токеном → `401` `unauthenticated`, з недозволеною
роллю → `403` `forbidden`, і відхилений запит нічого не змінює.

### Ролі та доступ

Ролі ті, що є в схемі (`CHECK (role IN ('member','staff','admin'))` у
[migrations/000001_init_schema.up.sql](migrations/000001_init_schema.up.sql)):

| Роль | Хто це | Що може |
|---|---|---|
| `member` | типова роль нового акаунта | читати каталог, бачити свій акаунт (`GET /me`), користуватися вебкаталогом |
| `staff` | персонал каталогу | усе, що `member`, плюс створення, зміна й видалення жанрів, виконавців, альбомів і примірників, зокрема видалення рядків у вебкаталозі |
| `admin` | адміністратор | усе, що `staff`, плюс список акаунтів і зміна ролей |

Матриця доступу API (статус для кожної ролі; anonymous = без токена; її перевіряє `TestRBACMatrix`):

| Маршрут | anonymous | member | staff | admin |
|---|---|---|---|---|
| `GET /api/v1/albums`, `GET /api/v1/genres` та інші `GET` каталогу | 200 | 200 | 200 | 200 |
| `GET /api/v1/me` | 401 | 200 | 200 | 200 |
| `POST /api/v1/genres`, `POST /api/v1/artists`, `POST /api/v1/albums`, `POST /api/v1/albums/{id}/copies` | 401 | 403 | 201 | 201 |
| `PUT /api/v1/albums/{id}` | 401 | 403 | 200 | 200 |
| `DELETE /api/v1/albums/{id}` | 401 | 403 | 204 | 204 |
| `GET /api/v1/users`, `PUT /api/v1/users/{id}/role` | 401 | 403 | 403 | 200 |
| `DELETE /web/albums/{id}` (cookie) | 401 | 403 | 200 | 200 |

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
  дають JSON `404` (зокрема `GET /`: домашньої сторінки немає, вхід у вебінтерфейс — `/catalog`).
- **Токен без стану:** зміна ролі через `PUT /users/{id}/role` діє з наступного логіну; виданий раніше
  токен зберігає стару роль до `exp`.
- **CORS:** кожна відповідь має `Access-Control-Allow-Origin: *`; `OPTIONS` → `204` зі списком методів
  і заголовків `Content-Type, Authorization`; preflight обробляється до перевірки токена. Облікові дані
  (cookie) у крос-доменних запитах браузери не надсилають, бо `Access-Control-Allow-Credentials` не задано.

### Як отримати токен

Повний запис цього сценарію з реальними відповідями (Тема 9): [docs/auth/token-flow.txt](docs/auth/token-flow.txt).

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

# staff або admin: 201 (адміністратор з розділу «Стартовий адміністратор»)
ADMIN=$(curl -s -X POST $BASE/auth/login -H 'Content-Type: application/json' \
  -d '{"email":"admin@albums.local","password":"Albums-Dev-2026"}' | jq -r .token)
curl -s -X POST $BASE/genres -H "Authorization: Bearer $ADMIN" -H 'Content-Type: application/json' -d '{"name":"ambient"}'
# 201 {"id":4,"name":"ambient",...}

# Адміністратор підвищує alice до staff: новий логін alice видає токен із новою роллю,
# старий токен лишається member до exp
curl -s -X PUT $BASE/users/4/role -H "Authorization: Bearer $ADMIN" -H 'Content-Type: application/json' -d '{"role":"staff"}'
# 200 {"id":4,"email":"alice@example.com","role":"staff",...}
```

### Стартовий адміністратор

Міграції не зберігають хешів. Міграція [000003_add_auth_fields](migrations/000003_add_auth_fields.up.sql)
додає колонку `password_hash TEXT NOT NULL DEFAULT ''`; seed-акаунти `admin@albums.local`,
`staff@albums.local` і `member@albums.local` лишаються без пароля і не можуть увійти (порожній хеш ніколи
не збігається, а логін порівнює його з фіктивним хешем, щоб і час відповіді нічого не видавав). Першого
адміністратора створює або скидає команда [cmd/bootstrap](cmd/bootstrap/main.go): вона перевіряє вхід
тими самими правилами, що й реєстрація, застосовує міграції (якщо API ще не застосував), хешує пароль
bcrypt і робить upsert рядка з роллю `admin`. Повторний запуск скидає пароль того самого рядка.

```sh
make bootstrap                                   # на хості: ADMIN_EMAIL і ADMIN_PASSWORD з .env
docker compose exec -e ADMIN_PASSWORD=Albums-Dev-2026 api /app/bootstrap -email admin@albums.local   # у контейнері
```

Dev-пароль `Albums-Dev-2026` задокументовано тут і в `.env.example`; для будь-якого іншого середовища
його треба змінити (`.env` не комітиться). Тим самим акаунтом можна увійти і у вебінтерфейс (`/login`).

| Акаунт | Роль | Пароль |
|---|---|---|
| `admin@albums.local` | `admin` | після `make bootstrap`: `Albums-Dev-2026` (або інший з `ADMIN_PASSWORD`) |
| `staff@albums.local` | `staff` | немає, увійти не може; за потреби `cmd/bootstrap -email staff@albums.local` (стане `admin`) або реєстрація нового акаунта і `PUT /users/{id}/role` |
| `member@albums.local` | `member` | немає, увійти не може |

## Структура проєкту та прийняті рішення

```
cmd/api/main.go                      конфігурація з env (JWT_SECRET читається один раз, порожній -> fail fast), міграції, HTTP-сервер, graceful shutdown
cmd/bootstrap/main.go                створення або скидання адміністратора: bcrypt-хеш + upsert з роллю admin
internal/app/app.go                  NewRouter(repos, cfg): маршрути за групами public / protected / staff / admin -> ServeMux -> Logging(Recover(CORS))
internal/auth/                       bcrypt-хешування (DummyHash для невідомих акаунтів), видача та перевірка HS256 JWT, користувач у контексті запиту
internal/model/                      GORM-моделі Album, Artist, Genre, Copy, User (PasswordHash json:"-"), AlbumArtist + вхідні структури з Validate()
internal/repository/                 інтерфейси та GORM-реалізації, errors.go, db.go (Open), migrate.go (Migrate)
internal/handler/                    HTTP-обробники API (*_handler.go), errors.go (коди, відображення помилок), парсинг
internal/middleware/                 Logging, Recover, CORS (+ Authorization), Auth, RequireRole
internal/testdb/                     спільне підключення для тестів із БД (advisory lock, міграції, TRUNCATE)
migrations/                          000001_init_schema, 000002_seed_demo_data, 000003_add_auth_fields (up/down)
docs/                                OpenAPI (swag, BearerAuth), runbook з curl, ER-діаграма, docs/n1/ (Тема 8), docs/auth/ (Тема 9), docs/web/ (Тема 10)
scripts/                             initdb/ (бази для тестів), smoke.sh (smoke-тест CI з логіном), measure_n1.sh (N+1), render_er.sh
Dockerfile, docker-compose.yml       образ API з обома бінарниками (/app/api, /app/bootstrap) і стек postgres + api (JWT_SECRET обов'язковий)
Makefile, .env.example               щоденні команди (+ bootstrap) та приклад конфігурації
albums_lesson10_homework.md          умова Теми 10, адаптована під варіант albums
prompts.txt                          промпти завдань 2.1, 2.2, 3.1, 3.2 з нотатками про результат
.github/workflows/theme8-check.yml   CI: структура, міграції, тести, smoke-тест з аутентифікацією
```

<!-- TODO (кроки 3-4): додати до переліку web/ (templates.go, static.go, templates/, static/), internal/handler/web/,
     internal/middleware/cookie.go, internal/handler/authenticate.go, migrations/embed.go; оновити рядки cmd/api, Dockerfile. -->

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
  логер GORM на рівні `info` пише кожен SQL-запит у stdout з плейсхолдерами (`ParameterizedQueries`).

Рішення Теми 9, які лишаються в силі:

- **Секрет JWT.** `cmd/api/main.go` читає `JWT_SECRET` один раз на старті й завершується з помилкою,
  якщо він порожній, коротший за 32 байти або є заготовкою з `.env.example` (`validateSecret`); роутер
  отримує його явно через `app.Config{JWTSecret}` і панікує на порожнє значення. Тести задають секрет
  через `t.Setenv` і будують роутер після цього.
- **Паролі.** `bcrypt.GenerateFromPassword` з `bcrypt.DefaultCost` (10). Логін виконує
  `bcrypt.CompareHashAndPassword` завжди: для невідомого e-mail і для акаунта з порожнім хешем — проти
  `auth.DummyHash`, тож час відповіді нічого не видає, а тіло `401` в усіх випадках побайтово однакове.
- **Токен.** Лише HS256 (`jwt.WithValidMethods`): інший `alg`, хибний підпис, змінений payload,
  прострочений або відсутній `exp`, `user_id <= 0` чи порожня роль → `401`. Claims `user_id`, `role`,
  `exp` = 24 години, `iat`. Токен без стану: роль береться з claim.
- **Ланцюжок middleware.** Групи маршрутів у `net/http` — хелпери `protected`, `staff`, `admin` в
  `app.go`; `Auth` завжди перед `RequireRole`; невдача пише відповідь через `handler.WriteError` і робить
  `return` без `next.ServeHTTP`.
- **DTO.** `RegisterInput` і `LoginInput` містять лише `email` та `password`, `RoleInput` — лише `role`;
  обробники ніколи не декодують запити в `model.User`; `User.PasswordHash` має тег `json:"-"`.
- **Стартовий адміністратор:** міграція не зберігає хешів; перший пароль задає `cmd/bootstrap`.

Рішення Теми 10:

<!-- TODO (кроки 2-6): після реалізації підтвердити кожен пункт посиланнями file:line. -->
- **Дві поверхні аутентифікації.** API приймає лише `Authorization: Bearer`; вебмаршрути — лише cookie
  `albums_token` (`CookieAuth`). Cookie без заголовка не дає доступу до API, заголовок без cookie — до
  вебмаршрутів, тож CSRF-поверхня обмежена вебмаршрутами, які захищені `SameSite=Lax` і
  `http.CrossOriginProtection`.
- **Анонімний запит замість помилки.** `CookieAuth` ніколи не відхиляє запит: відсутня чи хибна cookie
  означає анонімного відвідувача (публічний каталог рендериться без кнопок), а `401`/`403` дає наявний
  `RequireRole` на маршруті видалення — ті самі коди й конверт помилок, що й в API.
- **Порожній `200` замість `204`** на `DELETE /web/albums/{id}`: HTMX не виконує swap на `204`.
  API-маршрут `DELETE /api/v1/albums/{id}` і далі відповідає `204`.
- **Пошук і наявність без N+1.** `ListOptions.Query` додає `ILIKE` за назвою й виконавцем до того
  самого запиту з `Preload`; наявність примірників для всієї сторінки читається одним запитом
  (`AvailabilityForAlbums`), тож сторінка каталогу коштує сталу кількість `SELECT`.
- **Один набір шаблонів на сторінку.** `layout.html` + партіали розбираються один раз, кожна сторінка
  — клон із власним `content`; відповідь спершу рендериться в буфер, тож помилка шаблону дає `500`, а не
  пів сторінки. Лише `html/template`, без `template.HTML`.
- **Усе в бінарнику.** Шаблони, статика (включно з HTMX) і міграції вбудовані через `go:embed`;
  `MIGRATIONS_PATH` — лише перевизначення для розробки; образ Docker більше не копіює `migrations/`.

## Lesson 10 Report: Full-Stack Go з HTMX та go:embed

Умова: [albums_lesson10_homework.md](albums_lesson10_homework.md). Стан виконання:

| Частина | Що потрібно | Стан |
|---|---|---|
| 1. Вручну | архітектура cookie замість `localStorage`, аудит шаблонів (лише `html/template`, без `template.HTML`); тест ізоляції статичного бінарника з порожньої теки | <!-- TODO --> у роботі |
| 2. Промпт-інжиніринг | промпти для шаблонів (`layout`, `catalog`, партіали, `login`) і для HTMX з обробниками (`internal/handler/web`, cookie-сесія); [prompts.txt](prompts.txt) | промпти складено (крок 1), виконання <!-- TODO --> |
| 3. AI-агент | `go:embed` для шаблонів, статики й міграцій; тести `internal/handler/web/catalog_test.go`; приймальні тести `internal/app/web_test.go` зелені | тести написано й червоні (крок 1), реалізація <!-- TODO --> |

### Частина 1. Архітектура вебшару та аудит автоекранування

<!-- TODO (крок 7): розгорнути після реалізації; нижче - проєктне рішення, прийняте до початку роботи. -->
**Де живе JWT у браузері.** Вебінтерфейс отримує той самий HS256 JWT, що й API, але не у відповіді JSON,
а в cookie `albums_token` з прапорцями `HttpOnly`, `SameSite=Lax`, `Path=/`, `Max-Age=86400` (= TTL
токена) і `Secure` за TLS (`COOKIE_SECURE=true`). `localStorage` для серверного застосунку гірший з двох
причин: будь-який скрипт на сторінці (XSS через необроблений ввід, скомпрометована CDN-бібліотека) читає
`localStorage` одним викликом і виносить токен, тоді як HttpOnly-cookie недоступна з JavaScript узагалі;
і токен із `localStorage` довелося б вручну додавати до кожного запиту HTMX, що означає власний
JavaScript, якого Тема 10 уникає. Ціна cookie — CSRF: браузер додає її автоматично. Тому `SameSite=Lax`
(cookie не надсилається з крос-сайтових `POST`/`DELETE`), `http.CrossOriginProtection` з Go 1.25
(перевіряє `Sec-Fetch-Site`/`Origin` для небезпечних методів) і жорстке розділення: API не читає cookie,
вебмаршрути не читають `Authorization`.

**Аудит шаблонів.** <!-- TODO (крок 7): результати grep -rn '"text/template"' і grep -rn 'template.HTML' (обидва порожні),
посилання на docs/web/template-audit.txt, пояснення, де саме autoescaping рятує (назва альбому з <b>, параметр q у value="",
id у hx-delete). -->

### Тест ізоляції єдиного бінарника

<!-- TODO (крок 7): виконати і вставити вивід docs/web/isolation.txt (file/otool, запуск з порожньої теки, curl /catalog, /static/*),
скриншоти. -->
```sh
make build                                        # CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/api ./cmd/api
DIR=$(mktemp -d /tmp/verify_albums.XXXXXX) && cp bin/api "$DIR"/ && cd "$DIR"
set -a; . ~/repo/golang-course-lesson-10/.env; set +a; unset MIGRATIONS_PATH
./api                                             # лише DATABASE_URL і JWT_SECRET; ні web/, ні migrations/ поруч немає
curl -s -o /dev/null -w '%{http_code} %{content_type}\n' localhost:8080/catalog
curl -s -o /dev/null -w '%{http_code} %{content_type}\n' localhost:8080/static/app.css
curl -s -o /dev/null -w '%{http_code} %{content_type}\n' localhost:8080/static/htmx.min.js
```

### Частина 2. Промпти для генерації коду

Промпти складено до генерації і збережено у [prompts.txt](prompts.txt); виконуються в цьому репозиторії
через Claude Code (Claude Fable 5.1) у порядку 2.1 → 3.1 → 2.2 → 3.2 (обробники 2.2 споживають вбудовані
файлові системи з 3.1). Код пишеться test-first: приймальні тести `internal/app/web_test.go` написано
першими і вони червоні.

**2.1. Макет, сторінка каталогу, партіали, сторінка входу.** Промпт фіксує контракт даних шаблонів
(`.Title`, `.User`, `.CanDelete`, `.Query`, `.Albums`, `.Error`; рядок з `.ID`, `.Title`, `.Artists`,
`.Genre`, `.ReleaseYear`, `.Label`, `.Available`, `.Total`, `.CanDelete`), `{{block "content" .}}` і
`{{block "title" .}}` у `layout.html`, форму пошуку з трьома HTMX-атрибутами, що працює і без JavaScript,
партіал `album_rows` з порожнім рядком, кнопку видалення лише для `.CanDelete`, заборону `template.HTML`
і сторінку входу, яка не повертає введений e-mail (однакові тіла для двох видів невдалого входу).
<!-- TODO (крок 2/4): аналіз результату. -->

**2.2. HTMX-інтерактивність, обробники, cookie-сесія.** Промпт описує `ListOptions.Query` з `ILIKE` і
екрануванням, `AvailabilityForAlbums` одним запитом, винесення перевірки облікових даних у
`handler.Authenticate`, `CookieAuth`, пакет `internal/handler/web` (сторінка, фрагмент за `HX-Request`,
видалення з порожнім `200`, логін/логаут з атрибутами cookie) і реєстрацію вебгрупи в `app.go` під
`http.CrossOriginProtection`. <!-- TODO (крок 4): аналіз результату. -->

### Частина 3. go:embed і тести

**3.1. `go:embed`.** <!-- TODO (крок 3): що згенеровано (web/templates.go, web/static.go, migrations/embed.go, MigrateFS,
app.Config, Dockerfile/compose без migrations/), ручні правки, перевірка. -->

**3.2. Тести.** Приймальні тести `internal/app/web_test.go` написано до реалізації; до неї кожен маршрут
відповідає JSON `404` (повний вивід: [docs/web/tests-before.txt](docs/web/tests-before.txt)):

```text
--- FAIL: TestWebCatalogPage (0.25s)
    web_test.go:152: expected 200, got 404 {"error":{"code":"not_found","message":"route not found"}}
--- FAIL: TestWebAlbumRowsFragment (0.23s)
--- FAIL: TestWebDeleteRow (0.23s)
--- FAIL: TestWebLoginAndLogout (0.10s)
--- FAIL: TestWebStaticAssets (0.01s)
FAIL	homework/internal/app	1.568s
```

| Тест | Що перевіряє |
|---|---|
| `TestWebCatalogPage` | `200` HTML з макетом, формою пошуку (`hx-get`, `hx-trigger`, `hx-target`), статикою, усіма альбомами, виконавцями й `2/3` для альбому з трьома примірниками; екранування `<b>` у назві; `?q=` фільтрує і повертається у `value`; кнопки видалення лише для `staff`/`admin` |
| `TestWebAlbumRowsFragment` | `HX-Request` → лише `<tr>` (без `<html`, `<table`), збіг за назвою і за виконавцем без регістру, екранування у фрагменті, один `<tr class="empty">` без збігів, кнопка видалення лише для `staff`; без заголовка → `303` на `/catalog?q=` |
| `TestWebDeleteRow` | анонімний `401`, `member` `403`, Bearer-заголовок без cookie `401`, прострочена cookie `401`, `Sec-Fetch-Site: cross-site` `403`; відхилені запити нічого не змінюють; `staff`/`admin` → порожній `200`, альбом і примірники зникли; `404` і `400` для невідомого й нечислового id |
| `TestWebLoginAndLogout` | форма входу; успішний вхід → `303` і cookie з `HttpOnly`, `SameSite=Lax`, `Path=/`, `Max-Age=86400`, без `Secure`; cookie показує e-mail і роль у шапці; невідомий e-mail і хибний пароль → побайтово однакові `401` без `Set-Cookie`; логаут прострочує cookie |
| `TestWebStaticAssets` | `app.css` і `htmx.min.js` з правильними типами, `404` для невідомого файлу, лістинг `/static/` не віддається |

<!-- TODO (крок 5): catalog_test.go на фейках і зелений прогін go test ./... -race (docs/web/tests-after.txt). -->
