# Аудит безпеки Albums API (Тема 9, завдання 3.1)

Рев'ю виконав окремий AI-агент (Claude Code, свіжий контекст, лише читання) за інструкцією з умови
(промпт 3.1 у [prompts.txt](prompts.txt)) після того, як увесь код Теми 9 був написаний і тести
зелені. Необроблений звіт агента: [docs/auth/audit-raw.md](docs/auth/audit-raw.md). Тут — критична
оцінка кожної знахідки, рішення та що саме змінено. Обсяг рев'ю: `internal/auth`, `internal/middleware`,
`internal/handler/{auth,user,album,copy}_handler.go`, `errors.go`, `request.go`, `internal/repository`,
`internal/model/user.go`, `internal/app/app.go`, `cmd/api/main.go`, `cmd/bootstrap/main.go`,
`docker-compose.yml`, `.env.example`, `scripts/smoke.sh`.

Агент не знайшов SQL-ін'єкцій (усі запити параметризовані, `Raw`/`Exec` і конкатенації немає),
mass assignment (запити декодуються лише у вузькі `*Input`, роль при реєстрації задає сервер) чи
маршрутів без `Auth`/`RequireRole`; хеш не потрапляє у відповіді, токени й заголовок `Authorization`
не логуються. Сім знахідок стосуються країв дизайну. Додано восьму, знайдену під час ручного рев'ю
(завдання 1.2) ще до запуску агента.

## Підсумок тріажу

| № | Знахідка | Файл | Серйозність | Статус | Дія |
|---|---|---|---|---|---|
| 1 | Логер GORM на рівні `info` (і на `warn`/`error` для повільних чи невдалих запитів) друкував значення параметрів: bcrypt-хеш при реєстрації та bootstrap, e-mail при кожному логіні | `internal/repository/db.go:33-38` | Medium | **Fixed Now** | `ParameterizedQueries: true` (у лозі `$1, $2`), `Config.LogOutput` для тесту; `TestOpenLogsSQLWithoutValues`; докази [до](docs/auth/log-before-fix.txt) і [після](docs/auth/log-after-fix.txt); перевірка CI «No bcrypt hash in the server log» |
| 2 | Роль береться з 24-годинного stateless-токена: пониження або видалення акаунта діє лише після `exp`, відкликати токен неможливо | `internal/middleware/rbac.go:23`, `internal/auth/token.go` | Medium | **Tracked for Later** | так вимагає умова: middleware має брати роль із claim, а `TestRBACMatrix` чеканить staff/admin-токени для акаунтів, що в базі лишаються `member`; запропонований агентом DB-lookup у `Auth` зламав би контракт. `GET /me` уже відповідає `401` для видаленого акаунта. План: коротший TTL + refresh, колонка `token_version` (порівняння з `iat`) або denylist `jti` для logout |
| 3 | Немає rate limiting на `/auth/login` і `/auth/register`: перебір паролів, credential stuffing, bcrypt як підсилювач CPU, необмежена реєстрація з «адресами» до 1 MiB | `internal/app/app.go:49-50` | Medium | **Tracked for Later** (частково виправлено) | зараз: e-mail обмежено 254 байтами і формою `name@domain` (`emailProblems`, `TestEmailShapeAndLength`). Лімітер за IP та e-mail (`golang.org/x/time/rate` або на reverse proxy) і блокування після N невдач — наступна ітерація |
| 4 | Секрет перевірявся лише на порожність; швидкий старт копіював публічний секрет з `.env.example`; токени без `iss`/`aud` | `cmd/api/main.go:40-43`, `.env.example` | Medium | **Fixed Now** (`iss`/`aud` — Tracked) | `validateSecret`: щонайменше 32 байти, заготовка `change-me…` відхиляється (`TestValidateSecret`); `.env.example` містить лише заготовку, README і CURL.md генерують секрет через `openssl rand -base64 48`; локальний секрет ротовано; CI використовує власний довгий секрет |
| 5 | `POST /auth/register` відповідає `422 email already registered` і тому є оракулом існування акаунтів, на відміну від захищеного логіну | `internal/handler/auth_handler.go:52-55` | Low | **Tracked for Later** | саме це повідомлення й код вимагає умова (тест `register rejects a duplicate email with 422`, перевірка CI). Компроміс задокументовано; пом'якшення — лімітер з №3 або реєстрація через підтвердження e-mail (`202`) |
| 6 | У [docs/auth/token-flow.txt](docs/auth/token-flow.txt) були повні JWT, зокрема чинний admin-токен, підписаний публічним dev-секретом | `docs/auth/token-flow.txt` | Low | **Fixed Now** | транскрипт перегенеровано з маскуванням токенів; секрет ротовано, тож раніше закомічені токени мертві (і за добу прострочені); статична перевірка CI «No JWT committed under docs/ or in the reports» |
| 7 | PostgreSQL публікувався на `0.0.0.0:5432` з типовим паролем `albums`: будь-хто в мережі міг переписати `users.password_hash` | `docker-compose.yml:12` | Low | **Fixed Now** | порт прив'язано до `127.0.0.1` |
| 8 | (знайдено вручну до запуску агента) акаунт із порожнім хешем — seed-акаунти до bootstrap — відхилявся bcrypt миттєво (`ErrHashTooShort`), тож їх можна було вирізнити за часом відповіді | `internal/handler/auth_handler.go:90-98` | Medium | **Fixed Now** (у завданні 1.2) | порівняння з `auth.DummyHash` і для порожнього хеша; `TestAuthHandlerLogin` перевіряє однакові тіла й нижню межу часу |

Інформаційні зауваження агента (не вразливості): токен і паролі в аргументах `curl` у smoke-скрипті
(локальний скрипт, прийнято); відсутні `X-Content-Type-Options: nosniff` і `Cache-Control: no-store`
(JSON HTML-екранується, тож XSS через відбиття неможливий; дешеве посилення, **Tracked for Later**);
`json.Unmarshal` мовчки ігнорує зайве поле `role` при реєстрації (безпечно; `DisallowUnknownFields`
зробив би такі спроби видимими); немає `WWW-Authenticate: Bearer` у `401` (питання контракту);
адміністратор може понизити сам себе (lockout, не вразливість).

Кандидати, які я очікував побачити у списку знахідок і які агент перевірив як коректні (тобто
**False Positive**, якби їх було заявлено): `Access-Control-Allow-Origin: *` разом із bearer-токенами
(безпечно: токен не додається браузером автоматично, `Allow-Credentials` не задано); `TRUNCATE` в
`internal/testdb` (константа, лише тести); `First(&user, "email = ?", email)` і `Update("role", role)`
(параметризовані, роль додатково валідується і обмежена `CHECK`).

## Деталі

### 1. Значення параметрів у SQL-лозі (Fixed Now)

Доказ до виправлення ([docs/auth/log-before-fix.txt](docs/auth/log-before-fix.txt)): після
`POST /auth/register` у `docker compose logs api` з'являється
`INSERT INTO "users" (...) VALUES ('log-evidence@albums.local','member','$2a$10$…', …)`. Агент
додатково показав за кодом GORM v1.31.2, що `warn` і `error` теж друкують параметри (повільні й
невдалі запити), тож коментар у `cmd/bootstrap` про «безпечний warn» був хибним. Виправлення:
`ParameterizedQueries: true` в `logger.Config` ([internal/repository/db.go](internal/repository/db.go)),
щоб у лозі були лише плейсхолдери; `Config.LogOutput` дозволяє тесту перехопити лог.
`TestOpenLogsSQLWithoutValues` реєструє акаунт через репозиторій і перевіряє, що в лозі є
`INSERT INTO "users"`, але немає ні `$2a$10$`, ні e-mail. Підрахунок `SELECT` для N+1 (smoke-тест,
CI, `scripts/measure_n1.sh`) не змінився, бо самі запити логуються як і раніше. Доказ після:
[docs/auth/log-after-fix.txt](docs/auth/log-after-fix.txt).

### 2. Stateless-роль у токені (Tracked for Later)

Агент має рацію щодо наслідку: понижений чи видалений `staff`/`admin` зберігає права до `exp`
(до 24 годин), а скомпрометований токен не можна відкликати інакше, ніж ротацією `JWT_SECRET`.
Але запропоноване виправлення (читати роль з бази на кожен запит у `Auth`) суперечить умові Теми 9
(«middleware має брати роль із claim `role` токена») і зламало б `TestRBACMatrix`, який чеканить
staff- та admin-токени для акаунтів, що в базі мають роль `member`. Тому рішення зафіксовано як
відоме обмеження: у README описано, що нова роль діє з наступного логіну. Наступні кроки за
пріоритетом: TTL 1 година + refresh-токен; колонка `users.token_version`, яку `Auth` порівнює з
claim (одне PK-читання на запит, вже дозволене контрактом, бо роль лишається в claim); denylist
`jti` для явного logout.

### 3. Rate limiting (Tracked for Later, частково виправлено)

Підтверджено: лімітера немає, кожна анонімна спроба логіну коштує серверу одне bcrypt-порівняння
(~50-100 мс на ядро), реєстрація приймала будь-який рядок до 1 MiB як e-mail. Зараз зроблено
дешеву частину: `emailProblems` обмежує e-mail 254 байтами (RFC 5321) і формою `name@domain`
(`TestEmailShapeAndLength`). Повноцінний лімітер (за IP та за нормалізованим e-mail, з прогресивною
затримкою після N невдач) винесено в наступну ітерацію; у production його природне місце — reverse
proxy перед API.

### 4. Сила секрету та заготовка в .env.example (Fixed Now)

До виправлення сервер приймав будь-який непорожній секрет, а README радив `cp .env.example .env`
з публічним значенням. Тепер `validateSecret` у `cmd/api/main.go` відхиляє порожній секрет,
коротший за 32 байти та будь-який, що починається з `change-me` (заготовка з `.env.example`);
`TestValidateSecret` покриває всі чотири випадки. Швидкий старт у README і CURL.md генерує секрет
командою `openssl rand -base64 48`; CI задає власний секрет довжиною понад 32 байти; локальний
`.env` ротовано. `iss`/`aud` у claims — Tracked for Later: один сервіс, одне середовище.

### 5. Оракул існування акаунта в реєстрації (Tracked for Later)

Погоджуюсь із оцінкою Low і з тим, що це суперечить захисту логіну. Але `422` з повідомленням
`email already registered` зафіксовано умовою та приймальним тестом, і перевірка CI його очікує.
Компроміс прийнято свідомо: реєстрація — публічний маршрут, і без лімітера (№3) оракул дешевий.
Альтернатива на майбутнє: однакова відповідь `202` і завершення реєстрації листом з підтвердженням.

### 6. Чинні токени в закоміченому транскрипті (Fixed Now)

Перший запуск сценарію записав повні токени (admin-токен був чинним до 2026-10-14). Транскрипт
перегенеровано на свіжому стеку з маскуванням (`eyJ…` до 24 символів), секрет ротовано, тож старі
токени не верифікуються жодним сервером, і додано статичну перевірку CI, яка шукає JWT-подібні рядки
в `docs/`, README, prompts.txt і цьому файлі.

### 7. PostgreSQL на всіх інтерфейсах (Fixed Now)

`ports: - "127.0.0.1:${POSTGRES_PORT:-5432}:5432"` у `docker-compose.yml`: база доступна лише з
хоста, типовий пароль лишається внутрішньою справою локального стека.

### 8. Час відповіді для акаунта без пароля (Fixed Now, ручне рев'ю)

Знайдено під час завдання 1.2, до запуску агента: для рядка з `password_hash = ''`
`bcrypt.CompareHashAndPassword` повертає `ErrHashTooShort` без обчислень, тож три seed-акаунти можна
було вирізнити за часом відповіді. Логін тепер порівнює пароль з `auth.DummyHash`, якщо акаунт
невідомий або його хеш порожній; `TestAuthHandlerLogin` перевіряє побайтово однакові тіла для трьох
невдач і те, що випадок порожнього хеша коштує щонайменше 20 мс (одне bcrypt-порівняння).

## Висновок

З восьми знахідок п'ять виправлено з тестами (1, 4, 6, 7, 8), одну виправлено частково (3), дві
відкладено свідомо через вимоги умови (2, 5) і задокументовано як відомі обмеження. Після виправлень:
`go test ./... -race -count=1` зелений ([docs/auth/tests-after.txt](docs/auth/tests-after.txt)),
`scripts/smoke.sh` 78/78 ([docs/auth/smoke.txt](docs/auth/smoke.txt)).
