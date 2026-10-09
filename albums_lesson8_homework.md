# Домашнє завдання: Тема 8 — Інтеграція БД (PostgreSQL + GORM з AI)

## Наскрізний проєкт: Albums API (система обліку музичних альбомів, варіант `albums`)

> Умову адаптовано з курсового проєкту LibraTrack (варіант `books`): `Book` → `Album`, `authors` → `artists`,
> `categories` → `genres`, `book_authors` → `album_artists`, `category_id` → `genre_id`, `author_ids` → `artist_ids`.
> Поле `label` лишається текстовим (як `isbn` у книг), а унікальність альбому задає пара `(title, label)`.
> Імена файлів і міграцій узгоджено з перевірками CI (`.github/workflows/theme8-check.yml`), див. останній розділ.

**Часовий ліміт:** до 2 годин (завдяки використанню GenAI інструментів для прискорення розробки).

**Мета роботи:** перевести проєкт **Albums API** зі спрощеного in-memory сховища (з Теми 7) на реальну реляційну базу даних PostgreSQL за допомогою ORM GORM та міграцій. Виявити й виправити класичну проблему N+1, а також закріпити навички AI-асистованої розробки.

---

### ЧАСТИНА 1: Промпт-інжиніринг (Проєктування БД та Генерація міграцій) [~35 хв]

*Цей етап виконується за допомогою LLM (ChatGPT / Claude / Copilot Chat).*

#### Контекст та завдання

У Темі 7 сутність альбому (`Album`) містила виконавця (`artist`) та жанр (`genre`) у вигляді простих текстових полів. Тепер нам потрібно нормалізувати схему до **3NF (третьої нормальної форми)**:

1. Створити окрему таблицю `artists` (id, name, bio).
2. Створити окрему таблицю `genres` (id, name).
3. Зв'язати `albums` та `genres` як **One-to-Many** (в одного жанру багато альбомів).
4. Зв'язати `albums` та `artists` як **Many-to-Many** через проміжну (join) таблицю `album_artists`.

Поля `title`, `label`, `release_year` і `notes` лишаються в таблиці `albums`. CI додатково очікує таблиці `copies` (примірники альбому) та `users`, див. розділ «Що перевіряє CI».

#### Ваші дії

1. **Написання промпту:** складіть промпт до вашого AI-асистента для генерації файлів міграції SQL для інструменту `golang-migrate`.
   * **Вимога до промпту:** передайте ваші Go-структури моделей або детальний опис зв'язків. Попросіть згенерувати два файли: `000001_init_schema.up.sql` та `000001_init_schema.down.sql` (саме ці імена перевіряє CI; тестові дані йдуть окремою парою `000002_seed_demo_data.up.sql` / `.down.sql`).
   * **Обов'язково вкажіть в промпті:** генерацію явних обмежень зовнішніх ключів (Foreign Keys), поведінку `ON DELETE RESTRICT` (для жанрів і виконавців) або `ON DELETE CASCADE` (для таблиці зв'язків `album_artists` та примірників `copies`), а також індекси на поля зовнішніх ключів для оптимізації швидкодії.
2. **Аналіз результату:** перевірте згенерований AI `.down.sql` файл. Переконайтеся, що він правильно видаляє всі створені таблиці, індекси та обмеження у зворотному порядку.
3. **Запуск міграцій:** створіть файли міграцій локально та виконайте команду `migrate -database $DATABASE_URL -path migrations up`.

---

### ЧАСТИНА 2: Використання AI-агента (Моделі GORM та Репозиторій) [~45 хв]

*Цей етап виконується за допомогою AI-агента/асистента (наприклад, Cursor, GitHub Copilot, GoLand AI або ChatGPT).*

#### Контекст та завдання

Необхідно оновити Go-моделі та замінити in-memory репозиторій на роботу з GORM, **не змінюючи при цьому шар HTTP-обробників (інтерфейс `AlbumRepository` має залишитися незмінним)**.

#### Ваші дії

1. **Оновлення моделей:** попросіть AI-агента оновити Go-структури у файлі `internal/model/album.go` відповідно до нової схеми. Структура `Album` повинна містити зв'язки:
   * `` Genre   Genre    `gorm:"foreignKey:GenreID"` ``
   * `` Artists []Artist `gorm:"many2many:album_artists;"` ``
2. **Генерація репозиторію:** використовуючи AI-агент, згенеруйте код для нового репозиторію `gormAlbumRepository` у файлі `internal/repository/album_repository.go` (ім'я файлу, яке очікує CI).
   * **Вимога до промпту/агента:** «Реалізуй інтерфейс `AlbumRepository` з Теми 7 за допомогою GORM. Використовуй `db *gorm.DB`. Обов'язково прокидай контекст у кожен запит через `.WithContext(ctx)`. Напиши методи: `Create`, `Get`, `List`, `Update`, `Delete`».
   * *Примітка:* поточний інтерфейс `AlbumRepository` (`internal/repository/repository.go`) не має параметра `ctx`. Щоб прокидати контекст, додайте `ctx context.Context` першим параметром у методи інтерфейсу та передавайте `r.Context()` з обробників.
3. **Конфігурація DB:** налаштуйте підключення до PostgreSQL в `cmd/api/main.go` через Docker Compose (`DATABASE_URL` з `.env`, міграції з `MIGRATIONS_PATH`). Переконайтеся, що параметри пулу з'єднань (`SetMaxOpenConns`, `SetMaxIdleConns`) конфігуруються відповідно до найкращих практик.

---

### ЧАСТИНА 3: Ручне виконання (Верифікація, Діагностика N+1 та Фікс) [~40 хв]

*Цей етап виконується виключно вами вручну для глибокого розуміння роботи ORM.*

#### Контекст та завдання

Популярна проблема ORM — N+1 запит, коли завантаження списку сутностей супроводжується окремим SQL-запитом для кожної асоціації в циклі.

#### Ваші дії

1. **Підготовка середовища:** увімкніть логування всіх SQL-запитів у GORM за допомогою `logger.Info`.
2. **Запуск та тестування:** запустіть застосунок та базу даних через `docker-compose up -d`. Наповніть базу тестовими даними (мінімум 10 альбомів, 5 виконавців, 3 жанри; seed-міграція для CI має містити щонайменше 8 альбомів і 20 примірників).
3. **Аналіз логів (Виявлення N+1):**
   * Виконайте HTTP-запит `GET /api/v1/albums`.
   * Подивіться в консоль логів вашого Go-додатка. Ви побачите, що для кожного альбому робиться додатковий запит для отримання виконавців та жанру.
   * Зафіксуйте кількість запитів (наприклад: 1 загальний запит + N запитів до `artists` + N запитів до `genres`).
4. **Мануальний фікс:**
   * Самостійно (без допомоги AI) змініть метод `List` у вашому GORM-репозиторії, додавши завантаження зв'язків через `.Preload("Artists").Preload("Genre")`.
5. **Повторний замір:** знову виконайте запит `GET /api/v1/albums` і перевірте лог терміналу. Переконайтеся, що кількість запитів скоротилася до константи (2–3 запити сумарно, незалежно від кількості альбомів).

---

### Вимоги до здачі домашнього завдання (Deliverables)

Студент має надати посилання на свій репозиторій, де обов'язково є:

1. Створені `.sql` файли міграцій (`golang-migrate`).
2. Оновлений код моделей та GORM-репозиторію.
3. Файл **`README.md` з розділом "Lesson 8 Report"**, який містить:
   * використаний промпт для генерації міграцій з Частини 1;
   * **докази виправлення N+1:** фрагмент логів терміналу «ДО» (із повторюваними SQL-запитами) та логів «ПІСЛЯ» (де видно роботу `Preload` та батчевий `SELECT ... WHERE id IN (...)`);
   * різницю в локальній затримці (latency) у мілісекундах для обох варіантів.

---

### Що перевіряє CI (`.github/workflows/theme8-check.yml`)

Робочий процес адаптовано під варіант `albums`; він червоний, доки не виконано все нижче.

- **Файли.** `docker-compose.yml` (postgres із `healthcheck`), `.env.example` з `DATABASE_URL`, `.env` у `.gitignore`; `migrations/000001_init_schema.{up,down}.sql` і `migrations/000002_seed_demo_data.{up,down}.sql`; моделі `internal/model/{album,artist,genre,copy,user}.go`; репозиторії `internal/repository/{errors,db,migrate,album_repository,artist_repository,genre_repository,copy_repository}.go`; обробники `internal/handler/{errors,album_handler,artist_handler,genre_handler,copy_handler}.go`; тести в пакетах `model`, `repository` і `handler`.
- **Код.** Модуль `homework`; `gorm.io/gorm`, `gorm.io/driver/postgres` і `golang-migrate/migrate` у `go.mod`; без `AutoMigrate`; `ErrNotFound` оголошено рівно один раз; лише ASCII у `.go`-файлах; `gofmt`, `go vet`, `go build` чисті, `go mod tidy` нічого не змінює; у `Album` немає текстових полів `Artist`/`Genre`, є тег `many2many:album_artists`.
- **Схема.** Шість таблиць `genres`, `artists`, `albums`, `album_artists`, `copies`, `users`; `UNIQUE (title, label)` в `albums`; `UNIQUE inventory_no` та `CHECK` на `status` (`available`, `lost`, …) у `copies`; `CHECK` на `role` у `users`; FK `albums.genre_id` відхиляє неіснуючий жанр; видалення альбому каскадом прибирає його `copies` та `album_artists`; `migrate up` → `down -all` → `up` проходить без помилок.
- **Seed.** Щонайменше 8 альбомів і 20 примірників; є альбом із двома виконавцями та виконавець із двома альбомами.
- **API (`/api/v1`).** `genres` і `artists`: `POST` → 201, дубль імені жанру → 409 `conflict`, порожнє ім'я → 422, `GET` списку та за `id`, невідомий `id` → 404. `albums`: тіло з `title`, `label`, `genre_id`, `artist_ids` (+ `release_year`, `notes`), відповідь із вкладеними `genre` та `artists`; невідомий `genre_id`/`artist_id` → 422 `invalid_reference`, порожній `artist_ids` або відсутній `title` → 422 `validation_error`, дубль `title`+`label` → 409; `GET ?genre_id=`, `page`, `limit`; `PUT` замінює список виконавців; `DELETE` → 204. `POST`/`GET /albums/{id}/copies` (`status` типово `available`, дубль `inventory_no` → 409, хибний `status` → 422, невідомий альбом → 404); `GET /albums/{id}/availability` → `{album_id, total_copies, available_copies}`. `/health`, Swagger UI та CORS працюють, як у Темі 7.
- **N+1.** `GET /albums?limit=100` виконує сталу кількість `SELECT` (від 1 до 6) незалежно від кількості альбомів; у репозиторії є `Preload("Artists")`, маркера `NAIVE` немає; логер GORM на рівні Info, інакше запити не видно в логу.
- **Запуск.** Сервер стартує з `DATABASE_URL`, `MIGRATIONS_PATH=migrations`, `PORT=8080`; `go test -race ./...` виконується з `TEST_DATABASE_URL`, без `SKIP`, щонайменше 20 тестів.

---

# Homework Assignment: Lesson 8 — DB Integration (PostgreSQL + GORM with AI)

## Capstone Project: Albums API (music album catalog, the `albums` variant)

> Adapted from the LibraTrack assignment (the `books` variant): `Book` → `Album`, `authors` → `artists`,
> `categories` → `genres`, `book_authors` → `album_artists`, `category_id` → `genre_id`, `author_ids` → `artist_ids`.
> The `label` field stays a plain string (like `isbn` for books) and album uniqueness is the pair `(title, label)`.
> File and migration names match the CI checks (`.github/workflows/theme8-check.yml`), see the last section.

**Time Budget:** Up to 2 hours (engineered for speed and efficiency using GenAI tools).

**Objective:** Migrate the **Albums API** project from the simplified in-memory storage (developed in Lesson 7) to a durable PostgreSQL database using the GORM ORM and schema migrations. Identify and resolve a classic N+1 query problem, and strengthen your AI-assisted development skills.

---

### PART 1: Prompt Engineering (DB Schema Design & Migration Generation) [~35 mins]

*This stage is completed using an LLM (ChatGPT / Claude / Copilot Chat).*

#### Context and Task

In Lesson 7, the `Album` entity kept the artist and the genre as basic string fields. We now need to normalize this database schema to **3rd Normal Form (3NF)**:

1. Create a separate `artists` table (id, name, bio).
2. Create a separate `genres` table (id, name).
3. Associate `albums` and `genres` as a **One-to-Many** relationship (one genre has many albums).
4. Associate `albums` and `artists` as a **Many-to-Many** relationship via a join table named `album_artists`.

`title`, `label`, `release_year` and `notes` stay in the `albums` table. CI additionally expects the `copies` (physical copies of an album) and `users` tables, see "What CI checks".

#### Steps to follow

1. **Write the Prompt:** Construct a detailed prompt for your AI assistant to generate the standard Up and Down SQL migration files for `golang-migrate`.
   * **Prompt Requirements:** Provide your model structs or a precise description of the relationships. Ask for `000001_init_schema.up.sql` and `000001_init_schema.down.sql` (these are the names CI checks; demo data goes into a separate pair `000002_seed_demo_data.up.sql` / `.down.sql`).
   * **Mandatory Specifications:** Instruct the AI to include explicit foreign key constraints, define proper behavior on delete (`ON DELETE RESTRICT` for genres/artists, `ON DELETE CASCADE` for the `album_artists` join table and for `copies`), and indexes on foreign key columns.
2. **Analyze the Output:** Critically review the AI-generated `.down.sql` file. Verify that it correctly drops all tables, indexes, and constraints in the exact reverse order.
3. **Execute Migrations:** Create the files locally and run the migrations command: `migrate -database $DATABASE_URL -path migrations up`.

---

### PART 2: AI Agent Usage (GORM Models & Repository Implementation) [~45 mins]

*This stage is completed using an AI agent/coding assistant (Cursor, GitHub Copilot, GoLand AI, or ChatGPT).*

#### Context and Task

You must update your Go models and replace the in-memory repository implementation with GORM **without modifying the HTTP handler layer (the `AlbumRepository` interface must remain completely unchanged)**.

#### Steps to follow

1. **Update Models:** Use your AI agent to update the Go structs in `internal/model/album.go`. The `Album` struct must define associations:
   * `` Genre   Genre    `gorm:"foreignKey:GenreID"` ``
   * `` Artists []Artist `gorm:"many2many:album_artists;"` ``
2. **Generate the Repository:** Prompt the AI agent to write the new `gormAlbumRepository` in `internal/repository/album_repository.go` (the file name CI expects).
   * **Agent/Prompt Guidance:** "Implement the `AlbumRepository` interface from Lesson 7 using GORM. Use `db *gorm.DB`. Ensure you propagate the Go context in every single database call using `.WithContext(ctx)`. Implement `Create`, `Get`, `List`, `Update`, and `Delete`."
   * *Note:* the current `AlbumRepository` interface (`internal/repository/repository.go`) has no `ctx` parameter. To propagate the context, add `ctx context.Context` as the first parameter of every interface method and pass `r.Context()` from the handlers.
3. **DB Configuration:** Establish the PostgreSQL connection in `cmd/api/main.go` using the Docker Compose setup (`DATABASE_URL` from `.env`, migrations from `MIGRATIONS_PATH`). Set appropriate connection pool parameters (`SetMaxOpenConns`, `SetMaxIdleConns`) following industry best practices.

---

### PART 3: Manual Execution (Verification, N+1 Diagnostics & Fixing) [~40 mins]

*This stage must be performed entirely by you manually to ensure a solid, fundamental understanding of ORMs.*

#### Context and Task

A notorious ORM pitfall is the N+1 query problem, where loading a list of entities triggers separate SQL queries for each individual row's association in a loop.

#### Steps to follow

1. **Prepare Environment:** Enable verbose SQL logging in GORM using `logger.Info` in your DB configuration.
2. **Run and Seed:** Start your environment via `docker-compose up -d`. Seed the database with sample data (at least 10 albums, 5 artists, and 3 genres; the seed migration checked by CI needs at least 8 albums and 20 copies).
3. **Analyze Logs (Diagnose N+1):**
   * Trigger the `GET /api/v1/albums` endpoint.
   * Inspect your Go API terminal logs. Identify how many SQL queries are fired to retrieve the list.
   * Document this pattern (e.g., 1 main query + N artist queries + N genre queries).
4. **Manual Fix:**
   * Personally (without AI assistance) edit the `List` method in your GORM repository to pre-load associations via GORM's built-in eager loading: `.Preload("Artists").Preload("Genre")`.
5. **Re-Test & Benchmark:** Trigger `GET /api/v1/albums` again. Verify the log output. Ensure that the total query count is now a flat constant (2-3 queries total, regardless of the album count).

---

### Homework Deliverables

Submit a link to your repository containing:

1. The `.sql` up/down migration files.
2. The updated model structures and the fully integrated GORM repository.
3. A **`README.md` file with a "Lesson 8 Report"** section that includes:
   * The prompt you used to generate the migrations in Part 1.
   * **N+1 Proof:** Plain-text terminal query logs "BEFORE" the fix (highlighting repeating SQL selects) and "AFTER" the fix (highlighting the batched `SELECT ... WHERE id IN (...)` queries).
   * Local endpoint latency measurements in milliseconds (Before vs After).

---

### What CI checks (`.github/workflows/theme8-check.yml`)

The workflow is adapted to the `albums` variant and stays red until everything below is done.

- **Files.** `docker-compose.yml` (postgres with a `healthcheck`), `.env.example` with `DATABASE_URL`, `.env` in `.gitignore`; `migrations/000001_init_schema.{up,down}.sql` and `migrations/000002_seed_demo_data.{up,down}.sql`; models `internal/model/{album,artist,genre,copy,user}.go`; repositories `internal/repository/{errors,db,migrate,album_repository,artist_repository,genre_repository,copy_repository}.go`; handlers `internal/handler/{errors,album_handler,artist_handler,genre_handler,copy_handler}.go`; test files in the `model`, `repository` and `handler` packages.
- **Code.** Module `homework`; `gorm.io/gorm`, `gorm.io/driver/postgres` and `golang-migrate/migrate` in `go.mod`; no `AutoMigrate`; `ErrNotFound` declared exactly once; ASCII only in `.go` files; `gofmt`, `go vet`, `go build` clean and `go mod tidy` changes nothing; `Album` has no plain-text `Artist`/`Genre` fields and carries the `many2many:album_artists` tag.
- **Schema.** Six tables `genres`, `artists`, `albums`, `album_artists`, `copies`, `users`; `UNIQUE (title, label)` on `albums`; `UNIQUE inventory_no` and a `CHECK` on `status` (`available`, `lost`, …) on `copies`; a `CHECK` on `role` on `users`; the FK `albums.genre_id` rejects an unknown genre; deleting an album cascades to its `copies` and `album_artists`; `migrate up` → `down -all` → `up` succeeds.
- **Seed.** At least 8 albums and 20 copies; one album with two artists and one artist with two albums.
- **API (`/api/v1`).** `genres` and `artists`: `POST` → 201, duplicate genre name → 409 `conflict`, empty name → 422, `GET` list and by `id`, unknown `id` → 404. `albums`: body with `title`, `label`, `genre_id`, `artist_ids` (+ `release_year`, `notes`), response with nested `genre` and `artists`; unknown `genre_id`/`artist_id` → 422 `invalid_reference`, empty `artist_ids` or missing `title` → 422 `validation_error`, duplicate `title`+`label` → 409; `GET ?genre_id=`, `page`, `limit`; `PUT` replaces the artist list; `DELETE` → 204. `POST`/`GET /albums/{id}/copies` (`status` defaults to `available`, duplicate `inventory_no` → 409, invalid `status` → 422, unknown album → 404); `GET /albums/{id}/availability` → `{album_id, total_copies, available_copies}`. `/health`, Swagger UI and CORS keep working as in Lesson 7.
- **N+1.** `GET /albums?limit=100` runs a constant number of `SELECT`s (1 to 6) regardless of the album count; the repository contains `Preload("Artists")` and no `NAIVE` marker; the GORM logger is at Info level, otherwise no queries show up in the log.
- **Run.** The server starts from `DATABASE_URL`, `MIGRATIONS_PATH=migrations` and `PORT=8080`; `go test -race ./...` runs with `TEST_DATABASE_URL`, with no `SKIP` and at least 20 tests.
