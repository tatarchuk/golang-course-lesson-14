# Тема 14. Магія реального часу: WebSockets і живі оновлення — домашнє завдання (варіант `albums`)

Джерело: презентація «Урок 14 · Магія реального часу: WebSockets і живі оновлення» (50 слайдів, Модуль 3, фінальний
урок курсу «Go with GenAI»). Демо-проєкту до цього уроку немає, тому «приклади з презентації» — це код на слайдах.
Робота розділена на два репозиторії:

| Репозиторій | Що в ньому |
|---|---|
| [golang-course-lesson-14-examples](https://github.com/tatarchuk/golang-course-lesson-14-examples) | приклади з презентації, зібрані в Go-модуль `libratrack` і запущені на власному Mac |
| [golang-course-lesson-14](https://github.com/tatarchuk/golang-course-lesson-14) | застосунок `albums` із Тем 8–12 (історія збережена) з доданим онлайн-чатом |

## Оцінювання

Текст завдання (дослівно):

> -- Run the examples from the presentation on your own PC Verify using `go test -race` and manual testing of repeated
> connect/disconnect cycles.(70 points)
>
> -- Update your app by adding an online chat.
>
> Submit screenshots or a video recording demonstrating the application in operation. Also, provide a link to the
> GitHub repository containing the applications and configuration files.

| # | Завдання | Бали | Де |
|---|---|---|---|
| 1 | Запустити приклади з презентації на власному ПК; перевірити `go test -race` і ручним тестуванням повторних циклів підключення/відключення | 70 | examples |
| 2 | Додати до свого застосунку онлайн-чат | не вказано | albums |
| — | Скриншоти або відео роботи застосунку; посилання на GitHub-репозиторій із застосунками й конфігурацією | — | обидва |

Пункт 3 домашнього завдання зі слайда 48 (фінальна презентація на 5–10 хвилин) у тексті оцінювання відсутній і не
виконується.

## Завдання 1. Приклади з презентації (70)

Практична частина уроку (слайди 38–42) — чотири кроки: хаб на каналах, простий чат, живий дашборд і рефакторинг
конкурентності за допомогою ШІ. Код зі слайдів збирається в один модуль так:

| Слайд | Приклад | Файл у `golang-course-lesson-14-examples` |
|---|---|---|
| 8 | рукостискання: ключ `dGhlIHNhbXBsZSBub25jZQ==` дає `Sec-WebSocket-Accept: s3pPLMBiTxaQ9kYGzzhZRbK+xOo=` | журнал `curl` і тест `TestHandshakeAcceptMatchesSlide8` |
| 13, 16 | `websocket.Upgrader` із `CheckOrigin`, обробник, ліміти | `internal/realtime/ws_handler.go` |
| 14, 15, 27 | читання й запис, ping/pong, `readPump` і `writePump` | `internal/realtime/client.go` |
| 20, 25, 26, 28 | теми pub/sub, головний цикл хаба з єдиним власником, обмежені канали | `internal/realtime/hub.go` |
| 23 | мапа з'єднань без синхронізації (гонка даних) | `internal/naive/` (тест із тегом `racedemo`) |
| 24 | хаб із `sync.RWMutex` | `internal/mutexhub/` |
| 30 | Server-Sent Events | `internal/realtime/sse_handler.go` |
| 39 | ендпоінт `/ws` за JWT-middleware, токен у query-параметрі | `internal/auth/` |
| 40 | чат: автор береться з claims, а не з повідомлення клієнта | `internal/realtime/chat.go` |
| 41 | `LoanService.Return` публікує подію `loan_returned` | `internal/library/loan_service.go` |
| 34, 42 | промпти до ШІ: архітектура чату, огляд конкурентності хаба | `prompts.txt`, `docs/ai/` |

Порядок роботи повторює слайди: спочатку код рівно як на слайдах, потім тести з чекліста слайда 43, огляд ШІ за
промптом слайда 42 і виправлення знахідок (`go test -race` — справжній доказ, слайд 42). Ручна перевірка: три
вкладки чату, дашборд і SSE під час REST-виклику, повторні цикли підключення/відключення з браузера і з термінального
клієнта з порівнянням кількості горутин до і після, «мертвий» клієнт (`kill -STOP`) і тайм-аут ping/pong, відмова
`CheckOrigin` для чужого Origin.

## Завдання 2. Онлайн-чат у застосунку `albums`

Чат вбудовано в наявний застосунок (вимога слайдів 47–48: не окреме демо, а функція поверх REST API Тем 8–12).

| Що | Як |
|---|---|
| Сторінка | `GET /chat` у вебінтерфейсі; анонімного відвідувача переадресовує на `/login` |
| З'єднання | `GET /ws/chat?room=general\|staff` за `CookieAuth` (сесійний cookie `albums_token` з Теми 10) і `RequireRole` (Тема 9); токен ніколи не потрапляє в URL |
| Кімнати | `general` — member, staff, admin; `staff` — staff і admin (інакше 403); невідома кімната — 404 |
| Автор | e-mail облікового запису за `user_id` із claims; поля `author`, `room`, `type` від клієнта ігноруються |
| Origin | має збігатися з `Host`, інакше 403 (захист від cross-site WebSocket hijacking) |
| Повідомлення | клієнт надсилає `{"text":"..."}`; текст 1–1000 символів, інакше кадр `error` лише відправникові; понад 4096 байт — закриття 1009 |
| Кадри сервера | `message`, `notice`, `presence` (хто прийшов/пішов і скільки онлайн), `history` (останні 50 повідомлень кімнати при вході), `error` |
| Інтеграція з REST | створення альбому через API і видалення через API чи вебінтерфейс публікують `notice` у `general` |
| Статистика | `GET /api/v1/chat/stats` лише для admin: клієнти за кімнатами й кількість горутин |
| Життєвий цикл | ping/pong (60 с / 54 с), обмежений буфер відправки (256) з відключенням повільного клієнта (1008), коректне завершення сервера (1001 усім клієнтам) |

Перевірка: тести з `-race` (три клієнти без дублікатів, ізоляція кімнат, автор, ролі, Origin, ліміти, раптове
відключення, «мертвий» клієнт, 100 циклів підключення/відключення без витоку горутин, завершення сервера), базовий
сценарій у браузері (два користувачі в одній кімнаті) і цикли утилітою `cmd/chatload` зі статистикою до і після.

## Чеклісти з презентації

- Слайд 16 (основи сервера): `CheckOrigin` ніколи не `return true`; автентифікація під час рукостискання; `SetReadLimit`;
  тайм-аути читання й запису; `defer conn.Close()`; зняття реєстрації на будь-якому шляху виходу; причини відключень у лозі.
- Слайд 28 (одночасні з'єднання): мапою клієнтів володіє одна горутина; обмежені канали, повільний клієнт не гальмує
  розсилку; кожна горутина має один шлях виходу; `go test -race` у CI; кількість горутин як індикатор здоров'я.
- Слайд 43 (тести): 3 клієнти й повідомлення без дубліката у відправника; раптове закриття вкладки виявляється за
  тайм-аутом ping/pong; `go test -race ./...` без гонок; REST-виклик доходить до дашборда приблизно за 1 с; 100 клієнтів
  швидко підключаються й відключаються, кількість горутин повертається до вихідної.
- Слайд 44 (антипатерни): запис у з'єднання лише з `writePump`; жодних необмежених каналів відправки; `-race` завжди.
- Слайд 45 (перед завершенням): `h.clients` лише в `hub.go`; вихід `readPump` і `writePump` для закриття з обох боків;
  `go test -race`, а не просто `go test`; `CheckOrigin` справді відхиляє чужі запити.

## Що здати

- Скриншоти роботи: S1–S8 у репозиторії прикладів, C1–C2 у репозиторії застосунку (таблиці в README).
- Посилання на обидва репозиторії (вище). Журнали команд, тестів і оглядів ШІ лежать у `docs/` кожного репозиторію.

---

# Lesson 14. Real-time magic: WebSockets and live updates — homework (variant `albums`)

Source: the deck "Lesson 14 · Real-time magic: WebSockets and live updates" (50 slides, Module 3, the final lesson of
"Go with GenAI"). No demo project ships with this lesson, so "the examples from the presentation" are the code on the
slides. The work is split into two repositories:

| Repository | Content |
|---|---|
| [golang-course-lesson-14-examples](https://github.com/tatarchuk/golang-course-lesson-14-examples) | the slide examples, assembled into the Go module `libratrack` and run on the own Mac |
| [golang-course-lesson-14](https://github.com/tatarchuk/golang-course-lesson-14) | the `albums` app of Lessons 8-12 (history kept) with an online chat added |

## Grading

| # | Task | Points | Where |
|---|---|---|---|
| 1 | Run the examples from the presentation on your own PC; verify with `go test -race` and manual testing of repeated connect/disconnect cycles | 70 | examples |
| 2 | Update your app by adding an online chat | not stated | albums |
| — | Screenshots or a video of the application in operation; link to the GitHub repository with the applications and configuration files | — | both |

Item 3 of the slide 48 homework (a 5-10 minute final presentation) is not in the grading text and is not done.

## Task 1. The examples from the presentation (70)

The practical part (slides 38-42) has four steps: the channel hub, a simple chat, a live dashboard and an AI-assisted
concurrency refactor. The slide code maps to the module as in the Ukrainian table above (slides 8, 13-16, 20, 23-28,
30, 34, 39-42). The order follows the slides: first the code exactly as on the slides, then the tests of the slide 43
checklist, the AI review with the slide 42 prompt and the fixes of its findings (`go test -race` is the real proof,
slide 42). Manual checks: chat in three tabs, dashboard and SSE during a REST call, repeated connect/disconnect cycles
from the browser and from a terminal client with the goroutine count before and after, a dead client (`kill -STOP`)
and the ping/pong timeout, `CheckOrigin` refusing a foreign Origin.

## Task 2. Online chat in the `albums` app

The chat is built into the existing app (slides 47-48: not a separate demo but a feature on top of the Lesson 8-12
API): page `GET /chat` (anonymous visitors go to `/login`); connection `GET /ws/chat?room=general|staff` behind
`CookieAuth` (the Lesson 10 session cookie `albums_token`) and `RequireRole` (Lesson 9), so no token ever appears in a
URL; room `general` for member, staff and admin, room `staff` for staff and admin (else 403), unknown room 404; the
author is the account e-mail of the claims' `user_id`, and the client's `author`, `room` and `type` fields are ignored;
Origin must equal Host (else 403); the client sends `{"text":"..."}` with 1-1000 characters (else an `error` frame to
the sender only) and frames above 4096 bytes close the connection with 1009; server frames `message`, `notice`,
`presence`, `history` (the room's last 50 messages on join) and `error`; creating an album through the API and deleting
one through the API or the web UI publish a `notice` to `general`; `GET /api/v1/chat/stats` (admin only) shows clients
per room and the goroutine count; ping/pong 60 s / 54 s, a bounded send buffer (256) that disconnects a slow client
(1008), a clean server shutdown (1001 to every client).

Verification: tests with `-race` (three clients without duplicates, room isolation, author, roles, Origin, limits,
abrupt disconnect, dead client, 100 connect/disconnect cycles without a goroutine leak, server shutdown) and manual
cycles with `cmd/chatload`, with the stats before and after, and a basic case in a browser (two users in one room).

## Checklists from the deck

Slides 16, 28, 43, 44 and 45 as listed in the Ukrainian section: origin and authentication at the handshake, read
limit and timeouts, cleanup on every exit path; one goroutine owns the client map, bounded channels, one exit path per
goroutine, `-race` in CI; the five test scenarios of slide 43; writes only from `writePump`; the four questions of
slide 45.

## Deliverables

Screenshots S1-S8 in the examples repository and C1-C2 in the app repository (tables in the READMEs); links to both
repositories (above). Command logs, test runs and the AI reviews are under `docs/` in each repository.
