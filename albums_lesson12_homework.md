# Домашнє завдання: Тема 12 — Go Serverless: Lambda-функції на Go

## Наскрізний проєкт: Albums API (система обліку музичних альбомів, варіант `albums`)

> Умову зібрано з трьох курсових джерел: оцінювання «[Tasks 12] Go Serverless», слайд 48 «Домашнє
> завдання» презентації «Go Serverless: Lambda-функції на стероїдах» і шаблон `homework/README.md`
> демо-проєкту `libratrack-lambda-demo`. Презентація й демо називають тему «Урок 11»; у цьому
> проходженні курсу це Тема 12. Приклад із презентації (практична частина, слайди 39–47) — курсовий
> демо-проєкт LibraTrack, скопійований без змін у `serverless/libratrack-lambda-demo/`; усі зміни,
> потрібні, щоб він запрацював у цьому середовищі, перелічено у звіті. Власна функція —
> `album.submitted` (нормалізація заявки на альбом за полями `internal/model/album.go`) у
> `serverless/albums-lambda/`. «Цієї функції» в завданні 2 означає власну функцію: слайд 43 просить
> адаптувати промпт «до реального тригера, тайм-ауту та потреб у пам'яті вашої функції». Середовище —
> курсовий `Vagrantfile` (VM `DockerGoGroup`: Docker, Go, Terraform, AWS CLI, SAM CLI); на Apple Silicon у
> ньому змінено лише те, що там не працює, — перелік у заголовку `serverless/Vagrantfile`. Презентація не
> згадує Floci: його додає демо як локальну AWS для розгортання, S3-тригера, Terraform і бенчмарку, тож ці
> кроки README демо теж виконуються. Частини 3 шаблону демо (нотатка «serverless чи постійний сервер»)
> в оцінюванні немає, тому вона не виконується. Контракт власної функції, який фіксують тести, описано в
> останньому розділі.

**Мета роботи:** пояснити модель виконання serverless і перевагу скомпільованого бінарника Go під час
холодного старту; написати, зібрати й локально протестувати Lambda-функцію на Go з AWS Lambda Go SDK і
SAM CLI; генерувати й критично перевіряти Terraform, створений ШІ; проєктувати з урахуванням тайм-ауту,
пам'яті та відсутності стану.

---

### Оцінювання («[Tasks 12] Go Serverless»)

| Завдання | Бали |
|---|---|
| 1. Запустіть на власному ПК приклади з презентації | 70 |
| 2. Згенеруйте Terraform-код і проаналізуйте його рядок за рядком | +20 |
| 3. Напишіть serverless-функцію з HTTP-тригером | +10 |

Здача: скриншоти або відеозапис, що показують роботу застосунку, і посилання на GitHub-репозиторій із
застосунками та конфігураційними файлами.

---

### ЗАВДАННЯ 1: Приклади з презентації на власному ПК (70 балів)

> «Запустіть на власному ПК приклади з презентації. Виконайте тестовий запуск прикладу, пройшовши всі
> кроки.» (слайд 48)

Кроки практичної частини (слайди 40–44):

1. **Функція** (слайд 41). Lambda на Go, що обробляє вебхук: валідує вхідний JSON-payload і перетворює
   його (у прикладі — нормалізує заявку на пожертву книги для LibraTrack); `events.APIGatewayProxyRequest`
   / `Response`; `400` для некоректного вводу, `200` для успіху; мінімум залежностей.
2. **Локальний тест із SAM CLI** (слайди 20, 42). Збірка `GOOS=linux GOARCH=arm64 go build -ldflags="-s -w"
   -o bootstrap`, `sam local invoke` з валідною і невалідною фікстурами (обидва виклики повертають
   очікувані коди), `sam local start-api` і `curl` на `http://127.0.0.1:3000/webhook`.
3. **IaC від ШІ** (слайд 43). Terraform за шаблоном промпта зі слайда 33, адаптованим до функції; прочитати
   кожен ресурс (Lambda, IAM-роль, API Gateway), звузити IAM, переглянути вивід `terraform plan` (не
   `apply`; без облікових даних достатньо `terraform validate` і уважного читання).
4. **Бенчмарк холодних стартів** (слайд 44). Записати тривалість ініціалізації з виводу `sam local invoke`,
   знайти свіжий опублікований бенчмарк (Go проти Node.js чи Python), порівнювати подібне з подібним і
   прямо зазначити, що в продакшені AWS холодні старти будуть іншими.

Демо додатково розгортає приклади в Floci (AWS CLI, API Gateway, S3-тригер мініатюр, Terraform `plan` і
`apply`, бенчмарк cold/warm) — ці кроки його README теж виконуються.

---

### ЗАВДАННЯ 2: Terraform від ШІ, рядок за рядком (+20 балів)

> «Згенеруйте Terraform і проаналізуйте його рядок за рядком. Попросіть ШІ згенерувати конфігурацію
> Terraform для розгортання цієї функції та проаналізуйте згенерований код рядок за рядком.» (слайд 48)

- **Промпт:** «Промпт 1 · Terraform для функції» і «Додаток для Floci» з `docs/ai-prompts.md` демо (шаблон
  слайда 33, адаптований до Terraform, тригера, тайм-ауту й пам'яті функції).
- **Аналіз:** таблиця `| Ресурс / рядок | Що створює | Ризик | Чи змінили? Чому |` для кожного ресурсу й
  ризикованого рядка.
- **Перевірка (слайд 32):** обсяг IAM-політики; середовище виконання й архітектура збігаються зі збіркою;
  жорстко закодовані регіон, ID акаунта й імена стають змінними; `terraform plan` прочитано до `apply`.
- **План:** `terraform plan` на Floci (`make plan-tf`) і висновок про те, чого ви **не очікували** побачити.

---

### ЗАВДАННЯ 3: Serverless-функція з HTTP-тригером (+10 балів)

> «Напишіть serverless-функцію з HTTP-тригером. Напишіть serverless-функцію, що приймає JSON через
> HTTP-тригер і повертає оброблений результат — наприклад, валідацію чи перетворення даних.» (слайд 48)

Вимоги шаблону демо: за основу — `cmd/webhook`, але з іншою предметною областю (тут — `album.submitted`);
щонайменше дві фікстури в `events/` (валідна й невалідна); `go test` проходить; `400` для поганого вводу,
не `502`; клієнти й конфігурація — на рівні пакета.

---

### Чеклісти з презентації

Чекліст тестів збірки (слайд 45):

| Сценарій | Очікуваний результат |
|---|---|
| Коректний payload вебхука | `200` із перетвореним/провалідованим результатом |
| Некоректне JSON-тіло | `400` зі зрозумілим повідомленням про помилку, а не `502` |
| Відсутнє обов'язкове поле | `400`, а не паніка чи необроблений виняток |
| Час холодного старту в `sam local invoke` | Записано та порівняно з бенчмарком |
| Вивід `terraform plan` | Переглянуто — кожен ресурс і дозвіл зрозумілий |
| IAM-політика в згенерованому Terraform | Обмежена лише тим, що потрібно функції |

Чекліст перед завершенням (слайд 47):

- Функція правильно обробляє і коректний, і некоректний ввід — підтверджено щонайменше двома локальними
  фікстурами `sam invoke`, а не лише «щасливим шляхом».
- Клієнти SDK і конфігурацію ініціалізовано поза обробником (змінні рівня пакета).
- Кожен рядок згенерованого Terraform прочитано, зрозуміло й виправлено там, де значення за замовчуванням
  були надто широкими.
- Відомо приблизний час холодного старту функції та як він порівнюється зі знайденим бенчмарком.

---

### Вимоги до здачі домашнього завдання (Deliverables)

1. **Скриншоти або відеозапис** роботи застосунку: кожен крок прикладу, `terraform plan`, виклики власної
   функції через HTTP — у `docs/screenshots/serverless/`, з посиланнями у звіті.
2. **Посилання на публічний GitHub-репозиторій** із застосунками й конфігураційними файлами
   (`serverless/`: `Vagrantfile`, демо, власна функція, SAM-шаблони, Terraform).
3. **`README.md`** з розділом «Lesson 12 Report»: середовище, результати кожного кроку, таблиця аналізу
   Terraform, порівняння холодних стартів, обидва чеклісти з позначками й посиланнями на докази, зелений CI.
4. **`prompts.txt`:** промпт генерації Terraform і промпт фінального рев'ю.
5. **Докази** у `docs/serverless/`: текстові копії виводів, що стоять за скриншотами.

---

### Контракт `album.submitted` (що фіксують тести `serverless/albums-lambda/cmd/webhook`)

- **Запит:** `POST /webhook`, тіло `{"event":"album.submitted","data":{"title","label","artists","genre","release_year","notes"}}`.
- **`200`** `{"status":"processed","album":{...}}`: назва, лейбл та імена виконавців — без зайвих пробілів
  (зокрема нерозривних, табуляцій і переносів); повтори виконавців без урахування регістру прибрано, лишається
  перше написання; жанр у нижньому регістрі; `notes` обрізано, порожнє значення → `null`; `release_year` і
  `notes` завжди присутні в JSON.
- **`400`, ніколи не `502`** (обробник не повертає Go-помилку): порожнє тіло; некоректний base64; некоректний
  JSON; непідтримувана подія; `data` не JSON-об'єкт; поле не того типу (`invalid field type` з назвою поля);
  помилки валідації (`validation failed`, усі проблеми одразу): обов'язкові `title` і `label`, хоча б один
  виконавець і жодного порожнього імені, жанр зі списку `ALLOWED_GENRES` (за замовчуванням `electronic`,
  `jazz`, `rock` — жанри сид-даних), рік випуску від `MIN_RELEASE_YEAR` (за замовчуванням 1900) до наступного
  року включно.
- **Фаза init:** логер і конфігурація створюються на рівні пакета один раз; некоректна конфігурація
  зупиняє ініціалізацію.

---

# Homework Assignment: Lesson 12 — Go Serverless: Lambda Functions in Go

## Capstone Project: Albums API (music album catalogue, the `albums` variant)

> The assignment combines three course sources: the "[Tasks 12] Go Serverless" grading, slide 48
> "Домашнє завдання" of the deck "Go Serverless: Lambda Functions on Steroids", and the
> `homework/README.md` template of the `libratrack-lambda-demo` project. The deck and the demo call the
> topic "Lesson 11"; in this course run it is Lesson 12. The example of the deck (the practical part,
> slides 39–47) is the course's LibraTrack demo, copied unchanged into
> `serverless/libratrack-lambda-demo/`; every change needed to run it in this environment is listed in the
> report. The own function is `album.submitted` (normalizing an album submission with the fields of
> `internal/model/album.go`) in `serverless/albums-lambda/`. "This function" in task 2 means the own
> function: slide 43 asks to adapt the prompt "to the real trigger, timeout and memory needs of your
> function". The environment is the course `Vagrantfile` (VM `DockerGoGroup`: Docker, Go, Terraform, AWS
> CLI, SAM CLI); on Apple Silicon only what fails there is changed, listed in the header of
> `serverless/Vagrantfile`. The deck does not mention Floci: the demo adds it as a local AWS for deployment,
> the S3 trigger, Terraform and the benchmark, so those steps of the demo README are run too. Part 3 of the
> demo template (the "serverless or a persistent server" note) is not in the grading, so it is not done. The
> contract of the own function that the tests pin down is in the last section.

**Objective:** explain the serverless execution model and the cold-start advantage of a compiled Go
binary; write, build and test a Go Lambda function locally with the AWS Lambda Go SDK and the SAM CLI;
generate AI-written Terraform and review it critically; design for the timeout, memory and statelessness
limits.

---

### Grading ("[Tasks 12] Go Serverless")

> -- Run the examples from the presentation on your own PC (70 points)
> -- Generate the Terraform code and analyze it line by line (+20 points)
> **Write a serverless function with an HTTP trigger (+10 points)
>
> Submit screenshots or a video recording demonstrating the application in operation. Also, provide a
> link to the GitHub repository containing the applications and  configuration files.

---

### TASK 1: The deck's example on your own PC (70 points)

> "Run the examples from the presentation on your own PC. Do a test run of the example, going through
> all the steps." (slide 48)

The steps of the practical part (slides 40–44):

1. **The function** (slide 41). A Go Lambda that handles a webhook: it validates the incoming JSON payload
   and transforms it (the example normalizes a LibraTrack book donation request);
   `events.APIGatewayProxyRequest` / `Response`; `400` for invalid input, `200` for success; minimal
   dependencies.
2. **Local test with the SAM CLI** (slides 20, 42). Build with `GOOS=linux GOARCH=arm64 go build
   -ldflags="-s -w" -o bootstrap`, `sam local invoke` with a valid and an invalid fixture (both return the
   expected status codes), `sam local start-api` and `curl` against `http://127.0.0.1:3000/webhook`.
3. **IaC from AI** (slide 43). Terraform from the slide 33 prompt template, adapted to the function; read
   every resource (Lambda, IAM role, API Gateway), narrow IAM, review the `terraform plan` output (not
   `apply`; without credentials `terraform validate` and a careful read are enough).
4. **Cold-start benchmark** (slide 44). Record the init duration from the `sam local invoke` output, find a
   fresh published benchmark (Go against Node.js or Python), compare like with like, and state explicitly
   that cold starts in production AWS will differ.

The demo also deploys the examples to Floci (AWS CLI, API Gateway, the S3 thumbnail trigger, Terraform
`plan` and `apply`, a cold/warm benchmark); those steps of its README are run too.

---

### TASK 2: AI-generated Terraform, line by line (+20 points)

> "Generate Terraform and analyze it line by line. Ask AI to generate a Terraform configuration to deploy
> this function and analyze the generated code line by line." (slide 48)

- **Prompt:** "Промпт 1 · Terraform для функції" plus "Додаток для Floci" from the demo's
  `docs/ai-prompts.md` (the slide 33 template adapted to Terraform and to the function's trigger, timeout
  and memory).
- **Analysis:** the table `| Resource / line | What it creates | Risk | Changed? Why |` for every resource
  and every risky line.
- **Review (slide 32):** the IAM policy scope; runtime and architecture match the build; hard-coded region,
  account ID and names become variables; `terraform plan` is read before any `apply`.
- **Plan:** `terraform plan` on Floci (`make plan-tf`) and a conclusion about what you did **not** expect to see.

---

### TASK 3: A serverless function with an HTTP trigger (+10 points)

> "Write a serverless function with an HTTP trigger. Write a serverless function that accepts JSON through
> an HTTP trigger and returns a processed result, for example data validation or transformation." (slide 48)

The demo template's requirements: start from `cmd/webhook` but change the domain (here `album.submitted`);
at least two fixtures in `events/` (a valid and an invalid one); `go test` passes; `400` for bad input, not
`502`; clients and configuration at package level.

---

### Checklists from the deck

Build test checklist (slide 45):

| Scenario | Expected result |
|---|---|
| Valid webhook payload | `200` with the transformed/validated result |
| Malformed JSON body | `400` with a clear error message, not `502` |
| Missing required field | `400`, not a panic or an unhandled exception |
| Cold-start time in `sam local invoke` | Recorded and compared with the benchmark |
| `terraform plan` output | Reviewed: every resource and permission understood |
| IAM policy in the generated Terraform | Limited to what the function needs |

Checklist before finishing (slide 47):

- The function handles both valid and invalid input, shown by at least two local `sam invoke` fixtures,
  not only the happy path.
- SDK clients and configuration are initialized outside the handler (package-level variables).
- Every line of the generated Terraform was read, understood and fixed where the defaults were too broad.
- The approximate cold-start time of the function is known, and how it compares with the benchmark found.

---

### Evaluation Criteria & Deliverables

1. **Screenshots or a video** of the application in operation: every step of the example,
   `terraform plan`, calls to the own function over HTTP; in `docs/screenshots/serverless/`, linked from
   the report.
2. **A link to the public GitHub repository** with the applications and configuration files
   (`serverless/`: `Vagrantfile`, the demo, the own function, SAM templates, Terraform).
3. **`README.md`** with a "Lesson 12 Report" section: environment, the result of every step, the Terraform
   analysis table, the cold-start comparison, both checklists ticked with links to the evidence, the green CI run.
4. **`prompts.txt`:** the Terraform generation prompt and the final review prompt.
5. **Evidence** in `docs/serverless/`: text copies of the outputs behind the screenshots.

---

### The `album.submitted` contract (what the tests in `serverless/albums-lambda/cmd/webhook` pin down)

- **Request:** `POST /webhook`, body `{"event":"album.submitted","data":{"title","label","artists","genre","release_year","notes"}}`.
- **`200`** `{"status":"processed","album":{...}}`: title, label and artist names without extra whitespace
  (non-breaking spaces, tabs and newlines included); repeated artists are removed ignoring case, keeping the
  first spelling; the genre is lower-case; `notes` is trimmed, an empty value becomes `null`; `release_year`
  and `notes` are always present in the JSON.
- **`400`, never `502`** (the handler returns no Go error): empty body; invalid base64; malformed JSON; an
  unsupported event; `data` that is not a JSON object; a field of the wrong type (`invalid field type` with
  the field name); validation errors (`validation failed`, every problem at once): `title` and `label` are
  required, at least one artist and no blank names, a genre from `ALLOWED_GENRES` (default `electronic`,
  `jazz`, `rock`, the seed genres), a release year from `MIN_RELEASE_YEAR` (default 1900) up to next year.
- **Init phase:** the logger and the configuration are created once at package level; an invalid
  configuration stops the initialization.
