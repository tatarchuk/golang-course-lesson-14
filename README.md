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
Додаткова частина Теми 10 — DevOps Basics (віртуальна машина з Docker, Floci і k3s, демо з презентації,
Docker Compose, Kubernetes): умова [albums_lesson10_devops_homework.md](albums_lesson10_devops_homework.md),
звіт — розділ [Lesson 10 — додаткова частина](#lesson-10--додаткова-частина-devops-basics-vm-docker-floci-k3s).
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
  `TestWebStaticAssets`, `TestWebSecurityHeadersAndRootFallback`; таблиця в розділі [Частина 3](#частина-3-goembed-і-тести));
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
`migrations/embed.go`, вебобробники з їхніми тестами, `prompts.txt`; лише `html/template` і жодного з типів
`template.HTML`, `HTMLAttr`, `JS`, `JSStr`, `URL`, `CSS`, `Srcset`; `go:embed` і `fs.Sub` у `web/`, `ParseFS` без `ParseGlob`; вбудовані міграції без `COPY
migrations` у `Dockerfile`; прапорці cookie (`HttpOnly`, `SameSite=Lax`), `CrossOriginProtection` в `app.go`,
`CookieAuth` без `Authorization` і `Auth` без cookie; контрольна сума htmx 2.0.11; `COOKIE_SECURE` у
`.env.example` і `docker-compose.yml`; `make build`; JWT-guard охоплює й `prompts.txt`. В інтеграційній
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
`forbidden`. Докладніше в розділі [Частина 1](#частина-1-архітектура-вебшару-та-аудит-автоекранування).

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
albums_lesson10_homework.md          умова Теми 10, адаптована під варіант albums
albums_lesson10_devops_homework.md   умова додаткової частини Теми 10 (DevOps Basics), UA + EN
Vagrantfile                          VM для додаткової частини: Ubuntu 24.04 arm64 (VirtualBox), порти на 127.0.0.1, репозиторій у /app, провізіонери docker-engine, floci, aws-tools, k3s
deploy/vagrant/                      провізіонери (Docker Engine, AWS CLI + awslocal, k3s), env-status.sh, обхід перевірки портів Vagrant на macOS 27 (port_check_fix.rb + тест)
deploy/floci/                        Floci (compose.floci.yaml), Demo 1 (demo1-ec2.sh), Demo 2 (demo2-lambda.sh; lambda/ — окремий Go-модуль із тестами)
deploy/k8s/                          k3s: PostgreSQL (StatefulSet), API (Deployment, NodePort), Ingress (Traefik), bootstrap Job, create-secrets.sh
prompts.txt                          промпти завдань 2.1, 2.2, 3.1, 3.2 з нотатками про результат
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
  підсумками [фінального рев'ю](#фінальне-ревю).

## Lesson 10 Report: Full-Stack Go з HTMX та go:embed

Умова: [albums_lesson10_homework.md](albums_lesson10_homework.md). Стан виконання:

| Частина | Що потрібно | Стан |
|---|---|---|
| 1. Вручну | архітектура cookie замість `localStorage`, аудит шаблонів (лише `html/template`, без `template.HTML`); тест ізоляції статичного бінарника з порожньої теки | готово (крок 7): розділи нижче, [docs/web/template-audit.txt](docs/web/template-audit.txt), [docs/web/isolation.txt](docs/web/isolation.txt), скриншоти |
| 2. Промпт-інжиніринг | промпти для шаблонів (`layout`, `catalog`, партіали, `login`) і для HTMX з обробниками (`internal/handler/web`, cookie-сесія); [prompts.txt](prompts.txt) | готово: промпти складено до генерації (крок 1), виконано у кроках 3 і 4 |
| 3. AI-агент | `go:embed` для шаблонів, статики й міграцій; тести `internal/handler/web/catalog_test.go`; приймальні тести `internal/app/web_test.go` зелені | приймальні тести написано й червоні (крок 1); 3.1 `go:embed` готово (крок 3); 3.2 тести на фейках готово (крок 5) |
| Додаткова частина: DevOps | VM з Docker, Floci і перевіреним k3s; Demo 1 і 2 з презентації; Compose із базою даних; Kubernetes | готово: розділ [Lesson 10 — додаткова частина](#lesson-10--додаткова-частина-devops-basics-vm-docker-floci-k3s) |

### Частина 1. Архітектура вебшару та аудит автоекранування

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

У коді: cookie ставить `sessionCookie` (`internal/handler/web/auth.go:64-74`) після `handler.Authenticate`
(`internal/handler/authenticate.go`) — тієї самої перевірки, що й в API, тож невідомий e-mail і хибний пароль дають
побайтово однакову сторінку `401` без `Set-Cookie` (`TestWebLoginAndLogout`); `CookieAuth` читає лише cookie
(`internal/middleware/cookie.go:16`), `Auth` — лише заголовок; вебгрупа в `internal/app/app.go:88-106` обгорнута
`http.CrossOriginProtection` з відповіддю-конвертом `403`. Сценарій з реальними відповідями (вхід, cookie з
`#HttpOnly_` у jar, матриця видалення, логаут): [docs/web/cookie-flow.txt](docs/web/cookie-flow.txt).

**Аудит шаблонів** ([docs/web/template-audit.txt](docs/web/template-audit.txt)). `grep -rn '"text/template"'
--include='*.go' .` і `grep -rnE 'template\.HTML\b' --include='*.go' cmd internal web` порожні: єдине місце розбору
шаблонів — `internal/handler/web/templates.go` з `html/template.ParseFS`, і жодне значення не позначається як
безпечний HTML (`template.HTML`, `template.JS`, `template.URL`, `template.HTMLAttr` не використовуються). Контексти
динамічних значень: назва, виконавці, жанр, лейбл, рік і лічильники — текст у `<td>`; e-mail і роль — текст у
`<span>`; `q` — значення атрибута `value="..."` поля пошуку; `.ID` — атрибути `id="album-N"` і
`hx-delete="/web/albums/N"` (int64); повідомлення про помилку входу — текст у `<p>`. Живу перевірку проти бінарника
зроблено з альбомом `<script>alert("x")</script> & "quotes"` і запитом `?q=<b>x</b>`: у сторінці назва стає
`&lt;script&gt;alert(&#34;x&#34;)&lt;/script&gt; &amp; &#34;quotes&#34;`, запит у полі — `value="&lt;b&gt;x&lt;/b&gt;"`, а
фрагмент рядків не містить жодного `<script>`; те саме фіксують `TestCatalogPageRendersRowsAndEscapes` (пакет `web`),
`TestCatalogPage` (фейки) і `TestWebCatalogPage` (приймальний). CI-перевірки «Only html/template» і «template.HTML
and the other safe-content types are never used» стежать, щоб так і лишалося.

### Тест ізоляції єдиного бінарника

Рецепт завдання 1.2 і те, що зроблено:

```sh
make build                                        # CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o bin/api ./cmd/api
DIR=$(mktemp -d /tmp/verify_albums.XXXXXX) && cp bin/api "$DIR"/ && cd "$DIR"
set -a; . "$OLDPWD/.env"; set +a; unset MIGRATIONS_PATH     # лише DATABASE_URL і JWT_SECRET мають значення
./api                                             # ні web/, ні templates/, ні migrations/ поруч немає
curl -s -o /dev/null -w '%{http_code} %{content_type}\n' localhost:8080/catalog
curl -s -o /dev/null -w '%{http_code} %{content_type}\n' localhost:8080/static/app.css
curl -s -o /dev/null -w '%{http_code} %{content_type}\n' localhost:8080/static/htmx.min.js
```

Виконано 2026-10-16 ([docs/web/isolation.txt](docs/web/isolation.txt)): `bin/api` (25 МБ, `CGO_ENABLED=0`,
`-trimpath -ldflags='-s -w'`) скопійовано сам у порожню теку `/tmp/verify_albums.*` і запущено лише з `DATABASE_URL`
та `JWT_SECRET`; у лозі `migrations: applying the embedded set` і `schema version 3`, після чого `/catalog` →
`200 text/html` з усіма 10 seed-альбомами, `/catalog?q=kid` → один рядок, `/static/app.css` → `200 text/css`,
`/static/htmx.min.js` → `200 text/javascript` (52 182 байти, htmx 2.0.11), `/web/albums?q=kid` з `HX-Request` →
`200` лише `<tr>`, без заголовка → `303` на `/catalog?q=kid`, `/static/` → `404`. На macOS `otool -L` показує лише
системні бібліотеки (так злінковано кожен Go-бінарник на Darwin, власних залежностей немає), а збірка `GOOS=linux`
з тими самими прапорцями — `ELF 64-bit ... statically linked`; те саме перевіряє CI (`file bin/api` на Ubuntu і
запуск із `mktemp -d`). Браузер (headless Chrome 155 проти того самого процесу): [каталог](docs/screenshots/catalog.png),
[пошук](docs/screenshots/catalog-search.png), [вхід](docs/screenshots/login.png). Живий пошук перевірено в
Chrome 155 через розширення: введення `kid` у поле оновило таблицю запитом `GET /web/albums` без перезавантаження
сторінки (адреса лишилася `/catalog`), консоль без помилок і без порушень Content-Security-Policy. SQL-лог однієї
сторінки: [docs/web/queries-catalog.txt](docs/web/queries-catalog.txt): 5 `SELECT` для анонімного відвідувача, +1 для
залогіненого.

### Частина 2. Промпти для генерації коду

Промпти складено до генерації і збережено у [prompts.txt](prompts.txt); виконуються в цьому репозиторії
через Claude Code (Claude Fable 5.1) у порядку 2.1 → 3.1 → 2.2 → 3.2 (обробники 2.2 споживають вбудовані
файлові системи з 3.1). Код писався test-first: приймальні тести `internal/app/web_test.go` написано першими,
вони були червоними до кроків 2–4 і зелені відтоді ([до](docs/web/tests-before.txt), [після](docs/web/tests-after.txt)).

**2.1. Макет, сторінка каталогу, партіали, сторінка входу.** Промпт фіксує контракт даних шаблонів
(`.Title`, `.User`, `.CanDelete`, `.Query`, `.Albums`, `.Error`; рядок з `.ID`, `.Title`, `.Artists`,
`.Genre`, `.ReleaseYear`, `.Label`, `.Available`, `.Total`, `.CanDelete`), `{{block "content" .}}` і
`{{block "title" .}}` у `layout.html`, форму пошуку з трьома HTMX-атрибутами, що працює і без JavaScript,
партіал `album_rows` з порожнім рядком, кнопку видалення лише для `.CanDelete`, заборону `template.HTML`
і сторінку входу, яка не повертає введений e-mail (однакові тіла для двох видів невдалого входу).
Результат: згенеровано всі п'ять шаблонів і `web/static/app.css` точно за промптом, без ручних правок;
`web/static/htmx.min.js` не генерувався — це офіційна мініфікована збірка htmx.org 2.0.11 (npm-тег
`latest` на 2026-10-16), завантажена дослівно; версію й sha256 зафіксовано в
[docs/web/htmx.sha256](docs/web/htmx.sha256). До запуску промпту написано тести `web/web_test.go`, які
розбирають вбудований набір так само, як це робитимуть обробники, і рендерять три сторінки (розділ
[Тести та перевірки](#тести-та-перевірки)); вони зелені.

**2.2. HTMX-інтерактивність, обробники, cookie-сесія.** Промпт описує `ListOptions.Query` з `ILIKE` і
екрануванням, `AvailabilityForAlbums` одним запитом, винесення перевірки облікових даних у
`handler.Authenticate`, `CookieAuth`, пакет `internal/handler/web` (сторінка, фрагмент за `HX-Request`,
видалення з порожнім `200`, логін/логаут з атрибутами cookie) і реєстрацію вебгрупи в `app.go` під
`http.CrossOriginProtection`. Результат: частину 1 (репозиторії) виконано кроком 2 ще до шаблонів, решту — кроком 4; обидві частини
test-first (`internal/repository/catalog_search_test.go`, `internal/middleware/cookie_test.go`,
`internal/handler/authenticate_test.go` написано до коду). Згенеровано `internal/handler/authenticate.go`
(`Authenticate` + `ErrInvalidCredentials`; `AuthHandler.Login` тепер викликає його, відповіді й тести часу
без змін), `internal/middleware/cookie.go`, пакет `internal/handler/web` і вебгрупу в `app.go` під
`http.CrossOriginProtection` з JSON-конвертом `403`. Ручні відхилення від промпту: `StaticFiles` відповідає
JSON `404` і для відсутнього файлу, не лише для теки; фрагмент рядків визначає відвідувача тим самим
пошуком акаунта, що й сторінка (cookie видаленого акаунта рендериться анонімно на сторінці й у фрагменті; маршрут видалення довіряє токену до `exp`); `GET /login` теж
за `CookieAuth`, щоб залогінений бачив свій акаунт у шапці. Приймальні тести Теми 10 позеленіли з першого
запуску після реалізації.

### Частина 3. go:embed і тести

**3.1. `go:embed`.** Згенеровано за промптом: `web/templates.go` (`//go:embed templates/*.html
templates/partials/*.html`, `Templates()` через `fs.Sub`), `web/static.go` (`//go:embed static`, `Static()`
через `fs.Sub(staticFS, "static")`), `web/doc.go`, `migrations/embed.go` (`//go:embed *.sql`,
`migrations.FS`), `repository.MigrateFS(dsn, fsys)` з `Migrate(dsn, dir)` як обгорткою над `os.DirFS`,
перемикач у `cmd/api/main.go` (`applyMigrations`: порожній `MIGRATIONS_PATH` означає вбудований набір, лог
каже, яке джерело застосовано) і той самий перемикач у `cmd/bootstrap/main.go`, `app.Config{Templates,
Static, CookieSecure}` із fail fast на `nil`, `COOKIE_SECURE` через `envBool`, ціль `make build`, `Dockerfile`
без `COPY migrations` і `MIGRATIONS_PATH`, `docker-compose.yml` з `COOKIE_SECURE`, `.env.example` із
закоментованим `MIGRATIONS_PATH`. Ручні правки: змінну `migrations` в обох `main` перейменовано на
`migrationsDir`, бо вона затіняла імпортований пакет; `internal/testdb` мігрує з `migrations.FS` і втратив
`MigrationsDir`. Тести адаптовано вручну: `newRouter` передає `web.Templates()` і `web.Static()`,
`TestNewRouterPanicsWithoutTemplatesOrStatic`, `TestMigrateFSAppliesTheEmbeddedSet`, дисковий тест `Migrate`
дивиться на `../../migrations`, тест bootstrap працює без `MIGRATIONS_PATH`. Перевірено: `go test ./...
-race` (червоними лишаються лише приймальні тести Теми 10), `go mod tidy` без змін, `docker compose config
-q`, `docker compose build api`, і запуск `bin/api`, скопійованого самого в порожню теку, без
`MIGRATIONS_PATH` проти compose-бази: у лозі `migrations: applying the embedded set` і `schema version 3`
(схему створено з нуля), `GET /health` → `200`, `GET /api/v1/albums` → `200`. Повний тест ізоляції з
`/catalog` і статикою — у розділі [Тест ізоляції](#тест-ізоляції-єдиного-бінарника) після кроку 4.

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
| `TestWebStaticAssets` | `app.css` і `htmx.min.js` з правильними типами, `Cache-Control` і `nosniff`, `404` для невідомого файлу, лістинг `/static/` не віддається |
| `TestWebSecurityHeadersAndRootFallback` | CSP, `X-Frame-Options`, `nosniff` і `Referrer-Policy` на сторінках і їх відсутність в API; `GET /` лишається JSON `404` |

Після реалізації (кроки 2–4) приймальні тести позеленіли, а завдання 3.2 додало табличні тести обробників на
фейках у `internal/handler/web/catalog_test.go` (`TestCatalogPage`, `TestRowsFragment`, `TestDeleteRow`,
`TestLoginAndLogout`; 31 підтест без бази, маршрутизація як в `app.go`, тож `PathValue`, `CookieAuth`,
`RequireRole` і `CrossOriginProtection` працюють як у продакшені). Повний прогін
([docs/web/tests-after.txt](docs/web/tests-after.txt)):

```text
$ go test ./... -race -count=1
ok  	homework/cmd/api	1.823s
ok  	homework/cmd/bootstrap	4.292s
?   	homework/docs	[no test files]
ok  	homework/internal/app	23.150s
ok  	homework/internal/auth	4.492s
ok  	homework/internal/handler	10.611s
ok  	homework/internal/handler/web	5.399s
ok  	homework/internal/middleware	1.415s
ok  	homework/internal/model	1.371s
ok  	homework/internal/repository	23.899s
?   	homework/internal/testdb	[no test files]
?   	homework/migrations	[no test files]
ok  	homework/web	1.388s
# top-level tests passed: 127, subtests: 168
```

### Фінальне рев'ю

Коли код і документація були готові, окремий агент (Claude Code, свіжий контекст, лише читання; промпт у
[prompts.txt](prompts.txt), розділ «Final review») перечитав увесь diff Теми 10, README, умову, докази й CI.
Необроблений звіт: [docs/web/review-raw.md](docs/web/review-raw.md). Високих знахідок немає; тріаж 13 низьких:

| № | Знахідка | Рішення |
|---|---|---|
| 1 | `Scan`-запити GORM логуються через `logger.Recorder`, який ігнорує `ParameterizedQueries`: запит наявності потрапляв у лог зі значеннями (видно в першій версії `queries-catalog.txt`) | **Fixed Now**: `logger.RecorderParamsFilter` у `repository.Open` прибирає параметри; `TestOpenLogsSQLWithoutValues` покриває `Scan`; докази перегенеровано — тепер `$1$` |
| 2 | Cookie видаленого чи пониженого акаунта діє на маршруті видалення до `exp`, хоча сторінка показує його анонімним | **Documented**: та сама stateless-семантика, що й у staff-маршрутів API; описано у «Відомих обмеженнях» і в «Рішеннях Теми 10» |
| 3 | `ParseForm` без ліміту тіла (типово 10 МіБ) на неавтентифікованому маршруті з bcrypt; обмеження частоти немає | **Fixed Now** (ліміт 16 КіБ → `400`, тест) / **Tracked**: лімітер частоти для обох маршрутів входу, як і в Темі 9 |
| 4 | Статика без `Cache-Control` (у вбудованих файлів немає mtime, тож браузер качає htmx щоразу) | **Fixed Now**: `Cache-Control: public, max-age=86400` + `nosniff`, тест і smoke |
| 5 | Сторінки без CSP, `X-Frame-Options`, `nosniff`, `Referrer-Policy` | **Fixed Now**: `middleware.SecurityHeaders` на вебгрупі, `includeIndicatorStyles: false` для htmx, тест, smoke, перевірка в браузері |
| 6 | На `401`/`403`/`404` з маршруту видалення HTMX нічого не показує (рядок лишається) | **Documented** як відоме обмеження; `HX-Retarget` чи розширення `response-targets` — кандидат на наступну тему |
| 7 | `colspan="7"` у порожньому рядку при шести колонках для відвідувача | **Fixed Now**: `{{if .CanDelete}}7{{else}}6{{end}}`, два тести |
| 8 | CI-guard ловив лише `template.HTML`, не `HTMLAttr`/`JS`/`URL`/`CSS` | **Fixed Now**: regex розширено в workflow і в аудиті |
| 9 | `prompts.txt` називав кількість підтестів, якої не було в доказах | **Fixed Now**: `tests-after.txt` тепер містить обидва лічильники, текст виправлено |
| 10 | Три пункти контракту в умові перевіряв CI, а не тести; `Secure` не мав тесту | **Fixed Now**: тести на `GET /` → JSON `404` і на `Secure`; розділ умови перейменовано на «Що перевіряють тести і CI» |
| 11 | «5 `SELECT`» без уточнення, що для залогіненого це 6 | **Fixed Now**: уточнено в трьох місцях |
| 12 | Ліміт 50 рядків каталогу не був у README | **Fixed Now**: у таблиці вебмаршрутів |
| 13 | Два діапазони рядків у посиланнях file:line зсунуті на один | **Fixed Now** |

Після виправлень: `go test ./... -race` зелений (127 тестів верхнього рівня, 168 підтестів,
[docs/web/tests-after.txt](docs/web/tests-after.txt)), `scripts/smoke.sh` 102/102
([docs/web/smoke.txt](docs/web/smoke.txt)), докази в `docs/web/` перегенеровано тим самим скриптом.

## Lesson 10 — додаткова частина: DevOps Basics (VM, Docker, Floci, k3s)

Умова: [albums_lesson10_devops_homework.md](albums_lesson10_devops_homework.md), за презентацією «DevOps
Basics for GenAI Developers». «Flossy» в умові — це Floci, локальний емулятор AWS (слайди 29–30);
«демо для застосунків 1 і 2» — Demo 1 (EC2, слайд 32) і Demo 2 (Lambda на Go, слайди 33–36). Стан виконання:

| Частина | Що потрібно | Стан |
|---|---|---|
| 1. Середовище (70) | віртуальна машина зі встановленим Docker, запущений Floci, перевірений k3s | готово: [Vagrantfile](Vagrantfile), [vm.txt](docs/devops/vm.txt), [docker.txt](docs/devops/docker.txt), [floci.txt](docs/devops/floci.txt), [k3s.txt](docs/devops/k3s.txt); скриншот [environment.png](docs/screenshots/devops/environment.png) |
| 1. Демо (70) | Demo 1 і Demo 2 з презентації | готово: [demo1-ec2.txt](docs/devops/demo1-ec2.txt), [demo2-lambda.txt](docs/devops/demo2-lambda.txt); скриншоти [demo1-ec2.png](docs/screenshots/devops/demo1-ec2.png), [demo2-lambda.png](docs/screenshots/devops/demo2-lambda.png) |
| 2. Docker Compose (+30) | Go-застосунок у контейнері разом із базою даних у Docker Compose | готово: [Dockerfile](Dockerfile) і [docker-compose.yml](docker-compose.yml) запущено у VM, [compose.txt](docs/devops/compose.txt), smoke 102/102 ([smoke-compose.txt](docs/devops/smoke-compose.txt)); скриншоти [compose-ps.png](docs/screenshots/devops/compose-ps.png) (стек у VM і `/health` з Mac), [compose-browser.png](docs/screenshots/devops/compose-browser.png) (каталог від імені адміністратора) |
| 3. Kubernetes (**) | розгорнути застосунок у Kubernetes | готово: [deploy/k8s/](deploy/k8s/), [k8s.txt](docs/devops/k8s.txt), smoke 102/102 ([smoke-k8s.txt](docs/devops/smoke-k8s.txt)); скриншоти [k8s-pods.png](docs/screenshots/devops/k8s-pods.png), [k8s-browser.png](docs/screenshots/devops/k8s-browser.png) |

### Середовище

Робоча машина — Mac на Apple Silicon (M2 Pro, 16 ГБ). Одна віртуальна машина під Vagrant 2.4.9 і
VirtualBox 7.2.20 (box `bento/ubuntu-24.04` 202510.26.0, arm64, 4 vCPU, 6 ГБ) запускає все: Docker із
застосунком у Compose і Floci, а поруч k3s зі своїм containerd. Репозиторій змонтовано у VM як `/app`
(vboxsf, в обидва боки). Порти переадресовано лише на `127.0.0.1` Mac: 8080 (Compose), 30080 (NodePort
k3s), 8081 → 80 (Traefik Ingress), 6443 (Kubernetes API). Усе встановлюють ідемпотентні провізіонери
Vagrantfile; повторний `vagrant provision` нічого не перевстановлює (провізіонер `floci` лише заново
тягне образ рантайму Lambda за рухомим тегом `al2023`). Відтворюваність перевірено: `vagrant destroy -f &&
vagrant up` будує все середовище заново за 3 хв 41 с без ручних кроків ([rebuild.txt](docs/devops/rebuild.txt)).

| Компонент | Версія | Де | Перевірка |
|---|---|---|---|
| VM | Ubuntu 24.04.3 LTS, aarch64 | [Vagrantfile](Vagrantfile) | [vm.txt](docs/devops/vm.txt), скриншоти [vagrant_run.png](docs/screenshots/devops/vagrant_run.png), [vagrant-ubuntu.png](docs/screenshots/devops/vagrant-ubuntu.png), [virtualbox.png](docs/screenshots/devops/virtualbox.png) |
| Docker Engine | 29.9.1, Compose v5.6.0 | [deploy/vagrant/docker-engine.sh](deploy/vagrant/docker-engine.sh) | [docker.txt](docs/devops/docker.txt) |
| Floci | `floci/floci:2.3.0` | [deploy/floci/compose.floci.yaml](deploy/floci/compose.floci.yaml) | [floci.txt](docs/devops/floci.txt) |
| AWS CLI, awslocal | 2.38.2, 0.22.2 | [deploy/vagrant/aws-tools.sh](deploy/vagrant/aws-tools.sh) | [demo1-ec2.txt](docs/devops/demo1-ec2.txt) |
| k3s | v1.36.5+k3s1 | [deploy/vagrant/k3s.sh](deploy/vagrant/k3s.sh) | [k3s.txt](docs/devops/k3s.txt) |

### Як відтворити

```sh
brew install --cask virtualbox vagrant             # потрібен пароль адміністратора macOS
vagrant up                                         # VM + Docker + Floci + AWS CLI + k3s (усі провізіонери)
vagrant ssh -c /app/deploy/vagrant/env-status.sh   # VM, контейнери, здоров'я Floci, вузол і поди k3s

# Demo 1 і Demo 2 (Lambda збирається на Mac для linux/arm64)
vagrant ssh -c /app/deploy/floci/demo1-ec2.sh
(cd deploy/floci/lambda && GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags lambda.norpc -o bootstrap . && zip function.zip bootstrap)
vagrant ssh -c /app/deploy/floci/demo2-lambda.sh

# Docker Compose у VM (JWT_SECRET береться з .env, див. «Швидкий запуск»)
vagrant ssh -c 'docker compose -f /app/docker-compose.yml up -d --build'
vagrant ssh -c 'docker compose -f /app/docker-compose.yml exec -T -e ADMIN_PASSWORD=Albums-Dev-2026 api /app/bootstrap -email admin@albums.local'
open http://localhost:8080/catalog

# Kubernetes (k3s): образ у k3s, секрети, маніфести, адміністратор
vagrant ssh -c 'docker build -t albums-api:1.0 /app'
vagrant ssh -c 'docker save --platform linux/arm64 -o /tmp/albums-api.tar albums-api:1.0'
vagrant ssh -c 'sudo k3s ctr -n k8s.io images import /tmp/albums-api.tar'
vagrant ssh -c 'kubectl apply -f /app/deploy/k8s/namespace.yaml'
vagrant ssh -c /app/deploy/k8s/create-secrets.sh
vagrant ssh -c 'kubectl apply -k /app/deploy/k8s'
vagrant ssh -c 'kubectl -n albums rollout status statefulset/postgres --timeout=300s'
vagrant ssh -c 'kubectl -n albums rollout status deploy/albums-api --timeout=300s'
vagrant ssh -c 'kubectl apply -f /app/deploy/k8s/bootstrap-job.yaml'   # лише коли PostgreSQL уже готовий
vagrant ssh -c 'kubectl -n albums wait --for=condition=complete job/albums-bootstrap --timeout=120s'
open http://localhost:30080/catalog                # NodePort
open http://albums.localhost:8081/catalog          # Traefik Ingress (*.localhost браузер веде на 127.0.0.1)
```

Після `vagrant halt` / `vagrant up` провізіонери не повторюються: Floci повертається сам
(`restart: unless-stopped`), k3s — сервіс systemd, стек Compose піднімає
`vagrant ssh -c 'docker compose -f /app/docker-compose.yml up -d'`.

### Рішення

- **arm64 скрізь.** VirtualBox на Apple Silicon запускає лише ARM-гостей, тому box arm64, а не
  `ubuntu/jammy64` зі слайда 6 (лише amd64). Lambda збирається з `GOARCH=arm64` і створюється з
  `--architectures arm64`: Floci запускає Lambda в контейнері з архітектурою хоста.
- **Версії зафіксовано** (слайди 7, 15, 38, 46): box 202510.26.0, `floci/floci:2.3.0`, AWS CLI 2.38.2,
  awscli-local 0.22.2, k3s v1.36.5+k3s1, образ `albums-api:1.0` для k3s. Без фіксації лишилися пакети
  Docker Engine з репозиторію Docker (ставиться актуальний 29.x), образ рантайму Lambda
  `provided:al2023` (рухомий тег AWS) і локальний образ Compose `app-api`, який Compose збирає сам із
  тегом `latest`.
- **Floci 2.3.0 замість `hectorvent/floci:latest`.** Образ зі слайда 30 — заморожена збірка 1.5.5 того
  самого проєкту (він переїхав у `floci-io/floci`); 1.5.5 працює від UID 1001 без доступу до
  Docker-сокета, а 2.x сам налаштовує сокет і мережу для Lambda. LocalStack не підійшов як запасний
  варіант: з 2026.03 він вимагає токен акаунта.
- **EC2 у режимі mock** (`FLOCI_SERVICES_EC2_MOCK=true`). Floci 2.x запускає контейнер на кожен інстанс,
  а AMI з Demo 1 (`ami-0abcdef1234567892`) у каталозі Floci — Ubuntu 20.04 x86_64, який на arm64 не
  стартує. Mock дає саме поведінку, описану на слайді 32: інстанс — запис в емуляторі, а не VM.
- **Demo 2 з викликом функції.** Слайди 33–36 зупиняються на `get-function`; скрипт ще й викликає
  функцію з `{"name":"albums"}` і з `{}` (обидва `StatusCode 200`), що й доводить виконання коду.
  Обробник зі слайда 34 покрито двома тестами, написаними до коду; Lambda — окремий Go-модуль
  (`aws-lambda-go` v1.55.1 вимагає Go 1.26), тож `go ... ./...` кореневого модуля й CI його не збирають.
- **Секрети поза Git.** `.env` не відстежується; секрети Kubernetes генерує
  [create-secrets.sh](deploy/k8s/create-secrets.sh) у VM (`~/.albums-k8s.env`, права 600) і
  повторно використовує, бо PostgreSQL зберігає перший пароль. Пароль БД — hex: `DATABASE_URL` є URL,
  а застосунок його не кодує.
- **Kubernetes.** PostgreSQL — StatefulSet із томом local-path за headless Service: стабільне ім'я
  `postgres-0`, і дані пережили видалення пода (15 альбомів до і після). API — Deployment на 2 репліки:
  initContainer чекає на `pg_isready` (без нього API завершується після 10 невдалих спроб підключення),
  probes на `/health`, ліміти ресурсів, `runAsNonRoot` з `runAsUser: 1000` (kubelet перевіряє лише
  числовий UID), read-only root filesystem. `COOKIE_SECURE=false`, бо в лабораторії HTTP. Ingress
  маршрутизує за хостом `albums.localhost`, а не за префіксом шляху: UI посилається на кореневі шляхи
  (`/static`, `/catalog`). Адміністратора створює одноразовий Job.
- **Образ у k3s без реєстру:** `docker save --platform linux/arm64` + `k3s ctr -n k8s.io images import`,
  `imagePullPolicy: IfNotPresent`. У VM голий `ctr` належить containerd Docker'а, тому лише `k3s ctr`.
- **Обхід для Vagrant на macOS 27** ([port_check_fix.rb](deploy/vagrant/port_check_fix.rb)). Vagrant
  2.4.9 перевіряє, чи зайнятий порт хоста, неблокуючим `connect` з тайм-аутом, а на macOS 27 такий
  `connect` повідомляє про успіх навіть для відхиленого з'єднання (відтворено вбудованим Ruby Vagrant і
  системним Ruby, у пісочниці й поза нею). Тож кожен порт, навіть SSH 2222, «зайнятий», і `vagrant up`
  зупиняється з `ForwardPortCollision`. Виправлення — блокуючий `connect` з тайм-аутом 1 с;
  [port_check_test.rb](deploy/vagrant/port_check_test.rb) перевіряє обидва напрями (закритий порт
  вільний, порт зі слухачем зайнятий). Відтворення — у [vm.txt](docs/devops/vm.txt).
- **kubeconfig (слайд 40).** `kubectl` у VM — це сам k3s, і без `KUBECONFIG` він читає
  адміністративний `/etc/rancher/k3s/k3s.yaml` (доступний лише root), а не `~/.kube/config`.
  Провізіонер копіює kubeconfig користувачеві `vagrant` і додає `KUBECONFIG` у `/etc/environment`.

### Що в презентації не спрацювало як написано

| Слайд | У презентації | Що виявилося | Як зроблено тут |
|---|---|---|---|
| 6 | box `ubuntu/jammy64` | лише amd64, на Apple Silicon не завантажується | `bento/ubuntu-24.04` arm64 |
| 6 | `apt-get install -y golang` | у Ubuntu 22.04 це Go 1.18, а модуль albums вимагає Go 1.25.11 | збірка в Docker (`golang:1.25-alpine`, `GOTOOLCHAIN=auto`) |
| 16 | стретч-мета: `HEALTHCHECK` для образу на `distroless/static` | у distroless немає shell, curl і wget | runtime-образ albums — alpine; здоров'я перевіряють `/health` і probes Kubernetes |
| 25 | Terraform посилається на `aws_iam_role.lambda` | ресурс ролі ніде не визначено | Terraform умова не вимагає |
| 30 | `hectorvent/floci:latest` | заморожена 1.5.5, без доступу до Docker-сокета на Linux | `floci/floci:2.3.0` |
| 31 | IAM-політика `Version = "2026-10-17"`, `Service = "://amazonaws.com"` | версія має бути `2012-10-17`, сервіс — напр. `lambda.amazonaws.com`; справжній AWS таке відхилить | Terraform умова не вимагає |
| 32 | AMI `ami-0abcdef1234567892` | у Floci 2.x це x86_64, на arm64 інстанс завершується | EC2 у режимі mock |
| 35 | `GOARCH=amd64` | на arm64-VM Floci не виконає amd64-бінарник | `GOARCH=arm64`, `--architectures arm64` |
| 36 | демо закінчується на `get-function` | виконання коду не показано | два виклики `invoke` |
| 37 | `awslocal logs tail` | Floci 2.3.0 створює групу й потоки логів, але подій не пише (storedBytes 0) | показано в доказах, Demo 3 умова не вимагає |
| 40 | копія kubeconfig у `~/.kube/config` | `kubectl` від k3s її не читає без `KUBECONFIG` | провізіонер задає `KUBECONFIG` |
| 42–43 | секрет `llm` створюється двічі: з ключем `key` і з ключем `LLM_API_KEY` | другий `create` падає (`already exists`); `envFrom` бере імена змінних із ключів | `create-secrets.sh`: ключі = імена змінних, `--dry-run=client -o yaml \| kubectl apply` |

### Докази

- [docs/devops/vm.txt](docs/devops/vm.txt): VM (aarch64, Ubuntu 24.04.3, 4 CPU, 5,8 ГБ, `/app` через
  vboxsf) і відтворення проблеми з перевіркою портів.
- [docs/devops/docker.txt](docs/devops/docker.txt): Docker Engine 29.9.1, Compose v5.6.0, `hello-world`,
  повторний провізіонер.
- [docs/devops/compose.txt](docs/devops/compose.txt),
  [docs/devops/smoke-compose.txt](docs/devops/smoke-compose.txt): стек Compose у VM (образ 16,3 МБ,
  міграції, адміністратор), запити з Mac через переадресований порт, smoke 102/102.
- [docs/devops/floci.txt](docs/devops/floci.txt): Floci 2.3.0, `ec2`/`lambda`/`logs` running, образ
  Lambda `linux/arm64`.
- [docs/devops/demo1-ec2.txt](docs/devops/demo1-ec2.txt): ключ, каталог AMI, інстанс `running`;
  `awslocal` = `aws --endpoint-url http://localhost:4566`.
- [docs/devops/demo2-lambda.txt](docs/devops/demo2-lambda.txt): функція Active на arm64, два виклики
  з відповідями обробника, контейнер Lambda.
- [docs/devops/k3s.txt](docs/devops/k3s.txt): вузол Ready (arm64), системні поди, local-path, DNS
  кластера, Docker і k3s поруч.
- [docs/devops/k8s.txt](docs/devops/k8s.txt), [docs/devops/smoke-k8s.txt](docs/devops/smoke-k8s.txt):
  розгортання, NodePort і Ingress з Mac, вхід через cookie, самовідновлення, збереження даних,
  smoke 102/102.
- [docs/devops/rebuild.txt](docs/devops/rebuild.txt): відтворюваність — `vagrant destroy -f && vagrant up`
  за 3 хв 41 с заново ставить Docker, Floci, AWS CLI і k3s (awscli-local 0.22.2), стан середовища після.
- Скриншоти в [docs/screenshots/devops/](docs/screenshots/devops/): VM ([vagrant_run.png](docs/screenshots/devops/vagrant_run.png),
  [vagrant-ubuntu.png](docs/screenshots/devops/vagrant-ubuntu.png), [virtualbox.png](docs/screenshots/devops/virtualbox.png)),
  усе середовище ([environment.png](docs/screenshots/devops/environment.png)), демо
  ([demo1-ec2.png](docs/screenshots/devops/demo1-ec2.png), [demo2-lambda.png](docs/screenshots/devops/demo2-lambda.png)),
  Compose ([compose-ps.png](docs/screenshots/devops/compose-ps.png),
  [compose-browser.png](docs/screenshots/devops/compose-browser.png)), Kubernetes
  ([k8s-pods.png](docs/screenshots/devops/k8s-pods.png), [k8s-browser.png](docs/screenshots/devops/k8s-browser.png)).

### Фінальне рев'ю

Коли все було готово, окремий агент (Claude Code, свіжий контекст, Opus, лише читання репозиторію й
запущеної VM) перечитав увесь diff гілки разом з умовою, презентацією, планом і журналом рішень.
Необроблений звіт: [docs/devops/review-raw.md](docs/devops/review-raw.md). Критичних знахідок немає;
усі п'ять ризиків з плану закрито, кластер збігається із закомітженими маніфестами (`kubectl diff`
порожній), секретів у репозиторії немає. Тріаж:

| № | Знахідка | Рішення |
|---|---|---|
| I1 | Скриншоти Compose збігалися байт у байт із рендерами Теми 10 (та сама сторінка, ті самі дані), а запланованого `compose-ps.png` не було | **Fixed Now**: [compose-ps.png](docs/screenshots/devops/compose-ps.png) (стек у VM і `/health` з Mac) і [compose-browser.png](docs/screenshots/devops/compose-browser.png) (каталог від імені адміністратора); дублікати видалено |
| M1 | В умові був шлях `docs/screenshots/devops-*.png`, якого немає | **Fixed Now**: `docs/screenshots/devops/` в обох половинах |
| M2 | README обіцяв, що `latest` ніде не використовується і що повторний provision нічого не змінює | **Fixed Now**: awscli-local зафіксовано (0.22.2), у «Рішеннях» перелічено, що зафіксовано, а що ні |
| M3 | У «Як відтворити» Job адміністратора стартував, не чекаючи PostgreSQL | **Fixed Now**, підвищено до важливої (читач міг лишитися без входу): очікування обох rollout і завершення Job, як у перевіреному прогоні |
| M4 | Вбудований провізіонер `floci` не зупинявся на першій помилці | **Fixed Now**: `set -euo pipefail` |
| M5 | Обхід перевірки портів не прив'язаний до версії Vagrant | **Documented**: перевірено на Vagrant 2.4.9; прибрати, коли перевірку виправлять у Vagrant або macOS |
| — | Повний `vagrant up` з нуля жодного разу не запускався | **Fixed Now**: `vagrant destroy -f && vagrant up` — 3 хв 41 с, [rebuild.txt](docs/devops/rebuild.txt) |
| — | CI не перевіряє скрипти й маніфести DevOps і не запускає тести Lambda | **Tracked**: кандидати — `bash -n` для `deploy/**/*.sh`, `docker compose config` для Floci, `go -C deploy/floci/lambda test` |

Після виправлень CI зелений: [прогін 39427373820](https://github.com/tatarchuk/golang-course-lesson-10/actions/runs/39427373820)
(статичні перевірки, зокрема нові обов'язкові файли й `gofmt` для модуля Lambda; міграції, тести, smoke API).
