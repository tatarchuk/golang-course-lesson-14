# Albums API

REST-сервіс обліку музичних альбомів на Go з PostgreSQL і вебінтерфейсом на `html/template` та HTMX.
Проєкт виконано в межах курсу «Go with GenAI», варіант `albums`. Тема 7 дала REST API зі сховищем у
пам'яті; Тема 8 перевела його на PostgreSQL через GORM і golang-migrate, нормалізувала схему та виправила
N+1; Тема 9 захистила API: реєстрація і логін з `bcrypt` та JWT, рольова авторизація (`member`, `staff`,
`admin`) у вигляді middleware і AI-асистований аудит безпеки; Тема 10 додає серверний рендеринг:
сторінку каталогу з живим пошуком і видаленням рядків через HTMX, вхід через HttpOnly-cookie, а шаблони,
статику й міграції вбудовує в один бінарник через `go:embed`.

Матеріали Теми 10 і її додаткової частини DevOps Basics (умови, промпти, звіти) лишилися в репозиторії
попередньої теми (<https://github.com/tatarchuk/golang-course-lesson-10>); їхні докази збережено тут у
[docs/web/](docs/web/) і [docs/devops/](docs/devops/). Матеріали Теми 9 (умова, промпти, звіт аудиту) — у
репозиторії <https://github.com/tatarchuk/golang-course-lesson-9>, її докази — у [docs/auth/](docs/auth/).

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
open http://localhost:8080/catalog                 # вебкаталог із живим пошуком; вхід через /login (адміністратор із кроку 4)
docker compose logs -f api                         # кожен SQL-запит потрапляє в лог (без значень)
docker compose down                                # зупинити; дані лишаються у volume pgdata
```

Те саме через `make up`, `make bootstrap`, `make logs`, `make down` (усі цілі: `make` без аргументів
показує Makefile). Без `JWT_SECRET` стек не стартує: `docker-compose.yml` навмисно не має для нього
типового значення, а сервер відмовляється від секрету, коротшого за 32 байти, і від заготовки
`change-me…` з `.env.example` (`validateSecret` у `cmd/api/main.go`). PostgreSQL опубліковано лише
на `127.0.0.1`.

Образ API більше не містить теки `migrations/`: шаблони, статика й SQL-міграції вбудовані в обидва бінарники,
тож `/app` в образі — це лише `api` і `bootstrap`. Вебінтерфейс після старту: <http://localhost:8080/catalog>
(вхід — <http://localhost:8080/login>, адміністратор із кроку 4).

Скриншоти Теми 10 (headless Chrome проти бінарника, запущеного з порожньої теки): [каталог](docs/screenshots/catalog.png),
[пошук `kid`](docs/screenshots/catalog-search.png), [сторінка входу](docs/screenshots/login.png). Скриншоти Теми 8:
[Swagger UI з виконаним `GET /api/v1/albums`](docs/screenshots/swagger-get-albums-200.jpg) та
[огляд API](docs/screenshots/swagger-overview.jpg). Текстові докази Теми 9: [docs/auth/token-flow.txt](docs/auth/token-flow.txt)
і [docs/auth/smoke.txt](docs/auth/smoke.txt); Теми 10 — у [docs/web/](docs/web/) (тести до і після, ізоляція,
аудит шаблонів, cookie-сценарій, SQL-лог сторінки каталогу, smoke).

### Запуск на хості

Потрібен Go 1.25.11 або новіший (версія з `go.mod`). База береться з compose, сервер запускається
локально і так само застосовує міграції на старті.

```sh
docker compose up -d postgres        # або make db
set -a; . ./.env; set +a             # DATABASE_URL, JWT_SECRET, PORT, COOKIE_SECURE, ADMIN_* (MIGRATIONS_PATH - лише перевизначення)
make bootstrap                       # go run ./cmd/bootstrap -email $ADMIN_EMAIL (пароль з ADMIN_PASSWORD)
go run ./cmd/api                     # або make run
make build                           # bin/api: статично злінкований бінарник (CGO_ENABLED=0) із вбудованими шаблонами, статикою й міграціями
```

| Змінна | Призначення | Типово |
|---|---|---|
| `DATABASE_URL` | рядок підключення PostgreSQL, обов'язкова | — |
| `JWT_SECRET` | секрет підпису JWT (HS256), обов'язкова: щонайменше 32 байти, заготовка `change-me…` з `.env.example` відхиляється (`openssl rand -base64 48`) | — |
| `MIGRATIONS_PATH` | необов'язкове перевизначення: тека з SQL-файлами golang-migrate замість набору, вбудованого в бінарник (`migrations/embed.go`) | — (вбудовані міграції) |
| `COOKIE_SECURE` | прапорець `Secure` на cookie сесії вебінтерфейсу (`albums_token`); `true` за TLS-проксі | `false` |
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
go test ./internal/handler/web                # тести вебобробників на фейках (31 підтестів), без бази
gofmt -l . && go vet ./...
scripts/smoke.sh http://localhost:8080        # повтор API + веб smoke-тесту CI (100 перевірок) проти запущеного стека (після bootstrap)
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
  `TestListQueryCountIsConstant` на N+1, `UserRepository`, `TestMigrationAddsPasswordHashColumn`,
  `TestMigrateFSAppliesTheEmbeddedSet` (вбудований набір: 3 пари up/down, повторний запуск — no-op, версія 3);
  `TestOpenLogsSQLWithoutValues` тепер покриває і `Scan`-запит наявності (значення не потрапляють у лог).
  Тема 10: пошук за назвою або виконавцем без урахування регістру (`ListOptions.Query`: `ILIKE`,
  екранування `%`, `_` і `\`, поєднання з фільтром за жанром і пагінацією, та сама кількість `SELECT`,
  що й без пошуку) і `AvailabilityForAlbums` одним запитом для цілої сторінки (альбоми без примірників
  відсутні в результаті, порожній список id не торкається бази).
- `internal/handler`: обробники з фейковими репозиторіями: порядок перевірок, коди помилок, парсинг;
  реєстрація, логін (однакове тіло `401` для всіх невдач, одне bcrypt-порівняння завжди), `GET /me`,
  список акаунтів і зміна ролі; Тема 10: `Authenticate` (нормалізація e-mail, та сама помилка для невідомого
  e-mail, хибного пароля й акаунта без пароля; помилка сховища не маскується).
- `internal/handler/web` (`catalog_test.go`, фейки у `fakes_test.go`, маршрутизація як в `app.go`): сторінка
  каталогу для анонімного, `member`, `staff`, `admin` і видаленого акаунта (екранування назви, `?q=` доходить
  до репозиторію як `Query` і повертається у поле), фрагмент рядків (лише `<tr>`, порожній рядок, кнопка для
  `staff`, `303` без `HX-Request`), матриця на `DELETE /web/albums/{id}` (`401`/`403`/`200`/`404`/`400`,
  Bearer-заголовок ігнорується, крос-сайт `403`, перевіряється, що саме дійшло до репозиторію), логін і логаут
  (атрибути cookie й claims токена, `Secure` за `COOKIE_SECURE`, однакові тіла двох невдач без `Set-Cookie`, `400`
  для нечитабельної форми та для тіла понад 16 КіБ), порожній рядок на 6 або 7 колонок.
- `internal/middleware`: паніка → 500, CORS (з `Authorization`), логування, `Auth` (сім видів хибних
  облікових даних → `401`), `RequireRole` (`401`/`403`).
  Тема 10: `CookieAuth` (валідна cookie кладе claims у контекст; відсутня, прострочена, з іншим секретом
  чи під іншим ім'ям — анонімний запит без відповіді-помилки; заголовок `Authorization` ігнорується) і
  пара `CookieAuth` + `RequireRole` (`401`/`403`/прохід із кодами API); `SecurityHeaders` (CSP, `nosniff`, `DENY`,
  `Referrer-Policy`).
- `internal/app`: e2e через `NewRouter()` і реальну БД: сценарій smoke-тесту CI, граничні випадки
  контракту Теми 7, приймальні тести Теми 9 (`auth_test.go`, `rbac_test.go`) і Теми 10 (`web_test.go`:
  `TestWebCatalogPage`, `TestWebAlbumRowsFragment`, `TestWebDeleteRow`, `TestWebLoginAndLogout`,
  `TestWebStaticAssets`, `TestWebSecurityHeadersAndRootFallback`; таблиця в розділі [Частина 3 звіту Теми 10](https://github.com/tatarchuk/golang-course-lesson-10#частина-3-goembed-і-тести));
  `TestNewRouterPanicsWithoutSecret` фіксує fail fast без секрету.
- `web`: шаблони розбираються з вбудованої файлової системи так само, як в обробниках (макет і партіали
  один раз, клон на сторінку); сторінка каталогу рендерить рядки, наявність `2/3`, кнопки видалення лише
  для `CanDelete` і екранує HTML у назві та лапки в `q`; партіал рядків — лише `<tr>`; сторінка входу не
  повертає введений e-mail; статика містить `app.css` і `htmx.min.js`, а префікс `static/` знято.
- `cmd/bootstrap`: команда створює адміністратора, повторний запуск скидає пароль того самого рядка;
  без пароля або `DATABASE_URL` завершується помилкою; мігрує вбудованим набором, як і API.

CI ([.github/workflows/theme8-check.yml](.github/workflows/theme8-check.yml), тепер «Theme 8 + Theme 9 +
Theme 10 check») перевіряє структуру файлів, статичні правила, міграції на PostgreSQL, тести з реальною БД і
smoke-тест API та вебінтерфейсу. Додано для Теми 10. У статичній джобі: обов'язкові файли `web/`,
`migrations/embed.go`, вебобробники з їхніми тестами; лише `html/template` і жодного з типів
`template.HTML`, `HTMLAttr`, `JS`, `JSStr`, `URL`, `CSS`, `Srcset`; `go:embed` і `fs.Sub` у `web/`, `ParseFS` без `ParseGlob`; вбудовані міграції без `COPY
migrations` у `Dockerfile`; прапорці cookie (`HttpOnly`, `SameSite=Lax`), `CrossOriginProtection` в `app.go`,
`CookieAuth` без `Authorization` і `Auth` без cookie; контрольна сума htmx 2.0.11; `COOKIE_SECURE` у
`.env.example` і `docker-compose.yml`; `make build`. В інтеграційній
джобі: статична збірка `CGO_ENABLED=0` з перевіркою `file ... statically linked`, запуск бінарника самого з
порожньої теки без `MIGRATIONS_PATH` (у лозі має бути `applying the embedded set`) і веб smoke-тест: сторінка
каталогу й статика, фрагмент за `HX-Request` і `303` без нього, cookie-вхід адміністратора з `#HttpOnly_` у
cookie jar, матриця видалення (анонім `401`, member `403`, Bearer `401`, крос-сайт `403`, адміністратор —
порожній `200`, альбом зникає з API і з фрагмента), логаут прострочує cookie, сторінки несуть `Content-Security-Policy`,
`X-Frame-Options` і `nosniff`, статика кешується, анонімний `GET /catalog` коштує не більше 5 `SELECT`, у лозі
сервера немає токенів. Локальний повтор: `scripts/smoke.sh` дає 102/102
([docs/web/smoke.txt](docs/web/smoke.txt)). Workflow запускається на кожен push і pull request; зелений запуск для
цього коду (обидві джоби, Static checks та Migrations, tests, API smoke test):
<https://github.com/tatarchuk/golang-course-lesson-10/actions/runs/38913600126>.

### Документація API

- Swagger UI із запуском запитів з браузера: <http://localhost:8080/swagger/index.html>; для захищених
  маршрутів кнопка **Authorize** приймає рядок `Bearer <token>` (схема `BearerAuth`), після чого Swagger
  додає заголовок `Authorization` до запитів.
- Специфікація OpenAPI: [docs/swagger.yaml](docs/swagger.yaml), [docs/swagger.json](docs/swagger.json).
- Runbook із `curl`-запитами на всі випадки контракту, включно з отриманням токена та випадками
  `401`/`403`, і розділом 8 про вебінтерфейс (cookie jar, `HX-Request`, видалення рядка, логаут): [docs/CURL.md](docs/CURL.md).

Специфікацію згенеровано утилітою [swag](https://github.com/swaggo/swag) з анотацій у `cmd/api/main.go`
та обробниках `internal/handler/*_handler.go`; після їх зміни: `make swagger`. Усі анотації лишаються
ASCII, бо CI перевіряє і згенерований `docs/docs.go`. Вебобробники анотацій не мають: вони не є частиною
API `/api/v1`.

## Вебінтерфейс (Тема 10)

Сторінки рендеряться на сервері з шаблонів `html/template` (`web/templates/`), інтерактивність дає
HTMX (`web/static/htmx.min.js`, зафіксована версія 2.x) без власного JavaScript. Каталог публічний, як і
`GET /api/v1/albums`; кнопки видалення бачать лише `staff` та `admin` після входу через `/login`.

| Запит | Доступ | Відповідь |
|---|---|---|
| `GET /catalog?q=` | публічний | `200` HTML: макет, форма пошуку (`name="q"`), таблиця альбомів (назва, виконавці, жанр, рік, лейбл, примірники `доступно/усього`); `q` фільтрує на сервері за назвою або виконавцем без урахування регістру і повертається в поле; показує перші 50 збігів за `id`, пагінації в UI немає |
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
перетинаються. Крос-сайтовий запит на будь-який вебмаршрут зі зміною стану відхиляє
`http.CrossOriginProtection` (перевірка `Sec-Fetch-Site`/`Origin`, Go 1.25) з тим самим конвертом `403`
`forbidden`. Докладніше в розділі [Частина 1 звіту Теми 10](https://github.com/tatarchuk/golang-course-lesson-10#частина-1-архітектура-вебшару-та-аудит-автоекранування).

Відомі обмеження: токен без стану діє до `exp` і після логауту чи видалення акаунта (сторінка й фрагмент показують
такий акаунт як анонімний, але `DELETE /web/albums/{id}` довіряє ролі з токена, як і staff-маршрути API); на
`401`/`403`/`404` із маршруту видалення HTMX не виконує swap, тож рядок лишається без повідомлення (стан видно після
перезавантаження сторінки); обмеження частоти входу немає ні для форми, ні для `POST /api/v1/auth/login`
(відстежено з Теми 9).

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
cmd/api/main.go                      конфігурація з env (JWT_SECRET читається один раз, порожній -> fail fast), вбудовані міграції або MIGRATIONS_PATH, app.Config з Templates/Static/CookieSecure, HTTP-сервер, graceful shutdown
cmd/bootstrap/main.go                створення або скидання адміністратора: bcrypt-хеш + upsert з роллю admin
internal/app/app.go                  NewRouter(repos, cfg): групи public / protected / staff / admin + вебгрупа (cookie, CrossOriginProtection, /static/) -> ServeMux -> Logging(Recover(CORS))
internal/auth/                       bcrypt-хешування (DummyHash для невідомих акаунтів), видача та перевірка HS256 JWT, користувач у контексті запиту
internal/model/                      GORM-моделі Album, Artist, Genre, Copy, User (PasswordHash json:"-"), AlbumArtist + вхідні структури з Validate()
internal/repository/                 інтерфейси та GORM-реалізації, errors.go, db.go (Open), migrate.go (MigrateFS для вбудованого набору, Migrate для теки)
internal/handler/                    HTTP-обробники API (*_handler.go), authenticate.go (спільна перевірка облікових даних), errors.go, парсинг (+ PathID, WriteRepoError для вебшару)
internal/handler/web/                вебінтерфейс: Catalog, Rows (фрагмент для HX-Request), Delete (порожній 200), LoginForm/Login/Logout (cookie albums_token), StaticFiles; шаблони розбираються раз, клон на сторінку
internal/middleware/                 Logging, Recover, CORS (+ Authorization), Auth, RequireRole, CookieAuth (сесія вебінтерфейсу)
internal/testdb/                     спільне підключення для тестів із БД (advisory lock, вбудовані міграції, TRUNCATE)
migrations/                          000001_init_schema, 000002_seed_demo_data, 000003_add_auth_fields (up/down) + embed.go: SQL-файли вбудовані в бінарник
web/                                 вбудовані файли вебінтерфейсу (go:embed): templates.go, static.go, templates/ (layout, catalog, login, partials/album_row*), static/ (app.css, htmx.min.js 2.0.11)
docs/                                OpenAPI (swag, BearerAuth), runbook з curl, ER-діаграма, docs/n1/ (Тема 8), docs/auth/ (Тема 9), docs/web/ (Тема 10), docs/devops/ (додаткова частина Теми 10)
scripts/                             initdb/ (бази для тестів), smoke.sh (smoke-тест CI: API з логіном + вебінтерфейс із cookie jar), measure_n1.sh (N+1), render_er.sh
Dockerfile, docker-compose.yml       образ API з обома бінарниками (/app/api, /app/bootstrap) без теки migrations/ і стек postgres + api (JWT_SECRET обов'язковий, COOKIE_SECURE)
Makefile, .env.example               щоденні команди (+ bootstrap, build) та приклад конфігурації (COOKIE_SECURE; MIGRATIONS_PATH закоментовано)
Vagrantfile                          VM для додаткової частини: Ubuntu 24.04 arm64 (VirtualBox), порти на 127.0.0.1, репозиторій у /app, провізіонери docker-engine, floci, aws-tools, k3s
deploy/vagrant/                      провізіонери (Docker Engine, AWS CLI + awslocal, k3s), env-status.sh, обхід перевірки портів Vagrant на macOS 27 (port_check_fix.rb + тест)
deploy/floci/                        Floci (compose.floci.yaml), Demo 1 (demo1-ec2.sh), Demo 2 (demo2-lambda.sh; lambda/ — окремий Go-модуль із тестами)
deploy/k8s/                          k3s: PostgreSQL (StatefulSet), API (Deployment, NodePort), Ingress (Traefik), bootstrap Job, create-secrets.sh
.github/workflows/theme8-check.yml   CI «Theme 8 + 9 + 10»: структура й статичні правила (html/template, go:embed, cookie), міграції, тести, статичний бінарник з порожньої теки, API + веб smoke-тест
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

- **Дві поверхні аутентифікації.** API приймає лише `Authorization: Bearer` (`internal/middleware/auth.go:14`);
  вебмаршрути — лише cookie `albums_token` (`internal/middleware/cookie.go:16`; маршрути в
  `internal/app/app.go:92-106`). Cookie без заголовка не дає доступу до API, заголовок без cookie — до
  вебмаршрутів (`TestWebDeleteRow`: Bearer → `401`), тож CSRF-поверхня обмежена вебмаршрутами, які захищені
  `SameSite=Lax` (`internal/handler/web/auth.go:64-74`) і `http.CrossOriginProtection` (`app.go:96-99`, відповідь —
  конверт `403 forbidden`).
- **Анонімний запит замість помилки.** `CookieAuth` ніколи не відхиляє запит: відсутня чи хибна cookie означає
  анонімного відвідувача (публічний каталог рендериться без кнопок), а `401`/`403` дає наявний `RequireRole` на
  маршруті видалення — ті самі коди й конверт помилок, що й в API (`app.go:93-95`, `TestCookieAuthWithRequireRole`).
  Сторінка й фрагмент показують cookie видаленого акаунта як анонімного відвідувача, але маршрут видалення довіряє
  ролі з токена до `exp`, як і staff-маршрути API (stateless JWT, див. «Відомі обмеження» вище).
- **Порожній `200` замість `204`** на `DELETE /web/albums/{id}` (`internal/handler/web/catalog.go:63-73`): HTMX не
  виконує swap на `204`, а порожній `200` з `hx-swap="outerHTML"` прибирає рядок. API-маршрут
  `DELETE /api/v1/albums/{id}` і далі відповідає `204`.
- **Пошук і наявність без N+1.** `ListOptions.Query` додає `ILIKE ... ESCAPE '\'` за назвою й виконавцем до того
  самого запиту з `Preload` (`internal/repository/album_repository.go:92-97`, екранування в `likePattern`, `:239`);
  наявність примірників для всієї сторінки читається одним `GROUP BY` (`internal/repository/copy_repository.go:72`,
  виклик в `internal/handler/web/view.go:76-85`), тож `GET /catalog` коштує 5 `SELECT` для анонімного відвідувача
  (+1 `users.Get` для залогіненого) незалежно від кількості альбомів ([docs/web/queries-catalog.txt](docs/web/queries-catalog.txt)).
- **Один набір шаблонів на сторінку.** `layout.html` + партіали розбираються один раз, кожна сторінка — клон із
  власним `content` (`internal/handler/web/templates.go:28-44`); відповідь спершу рендериться в буфер
  (`templates.go:58-67`), тож помилка шаблону дає `500`, а не пів сторінки. Лише `html/template`, без
  `template.HTML` ([docs/web/template-audit.txt](docs/web/template-audit.txt)).
- **Усе в бінарнику.** Шаблони, статика (включно з HTMX) і міграції вбудовані через `go:embed` (`web/templates.go:8`,
  `web/static.go:8`, `migrations/embed.go:11`); `MIGRATIONS_PATH` — лише перевизначення для розробки
  (`cmd/api/main.go:121`, `internal/repository/migrate.go:22,46`); образ Docker більше не копіює `migrations/`;
  статика роздається через `StaticFiles` без лістингу тек (`internal/handler/web/static.go:16`).
- **Заголовки для браузера.** Сторінки йдуть із `Content-Security-Policy: default-src 'self'; frame-ancestors 'none';
  base-uri 'self'; form-action 'self'`, `X-Frame-Options: DENY`, `X-Content-Type-Options: nosniff` і
  `Referrer-Policy: same-origin` (`internal/middleware/security.go`), а HTMX не вставляє власний `<style>`
  (`includeIndicatorStyles: false` у `layout.html`), тож політика тримається без `'unsafe-inline'`; статика
  кешується на добу з `nosniff` (`Cache-Control: public, max-age=86400`: у вбудованих файлів немає mtime для
  валідаторів); тіло форми входу обмежене 16 КіБ (`internal/handler/web/auth.go:13,30`). Усе це додано за
  підсумками [фінального рев'ю Теми 10](https://github.com/tatarchuk/golang-course-lesson-10#фінальне-ревю).
