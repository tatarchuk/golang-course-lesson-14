# AI-generated Terraform draft (verbatim)

- Date: 2026-10-23
- Model: Claude, run as a fresh Claude Code general-purpose subagent of this session (no files, no tools)
- How: the prompt below was sent as is: the course's "Промпт 1 · Terraform для функції" and "Додаток для Floci"
  from `serverless/libratrack-lambda-demo/docs/ai-prompts.md`, after one instruction line. The reply under
  "Reply" is copied unchanged (verbatim). Its four files went unchanged into `serverless/albums-lambda/terraform/`
  in the commit "chore: AI-generated Terraform draft for the album webhook (unreviewed)"; the line-by-line review
  is in `README.md`, section "Lesson 12 Report".

## Prompt

```
Answer the request below as you would answer a user in a chat. Do not read or write files and do not use tools.

Згенеруй Terraform (hashicorp/aws ~> 6.0) для AWS Lambda функції на Go:

- runtime provided.al2023, architectures ["x86_64"], handler "bootstrap"
- deployment package: ../build/webhook.zip, з source_code_hash
- timeout 10 секунд, memory 256 МБ
- викликається через API Gateway REST: POST /webhook, інтеграція AWS_PROXY,
  стадія "dev"
- IAM-роль ЛИШЕ з правами на запис у CloudWatch Logs для цієї функції
  (жодних Action = "*", жодних Resource = "*" окрім де це неминуче)
- регіон і ім'я функції винеси у variables
- додай aws_lambda_permission для API Gateway

Окремо поясни, які ресурси створюються, і познач усе, що ти припустив.

Налаштуй provider "aws" для локального емулятора Floci (http://localhost:4566):
access_key/secret_key = "test", skip_credentials_validation,
skip_metadata_api_check, skip_requesting_account_id = true,
s3_use_path_style = true, і блок endpoints для кожного використаного сервісу.
Ендпоїнт винеси у змінну, щоб прибрати блок для справжньої AWS.
```

## Reply

Нижче чотири файли для каталогу поруч із `build/`, наприклад `infra/`. Мітки `П1`–`П15` у коментарях відсилають до списку припущень у кінці. Я нічого не запускав: ні `terraform validate`, ні `plan`, ні `apply` проти Floci.

### `providers.tf`

```hcl
terraform {
  required_version = ">= 1.5.0" # П15

  required_providers {
    aws = {
      source  = "hashicorp/aws"
      version = "~> 6.0"
    }
  }
}

locals {
  # Ендпоїнт заданий → емулятор; null або "" → справжня AWS.
  use_emulator = var.aws_endpoint != null && var.aws_endpoint != ""
}

provider "aws" {
  region = var.aws_region

  # П10: ключі й skip-прапорці діють лише для емулятора. Для AWS ключі = null
  # (стандартний ланцюжок: env, ~/.aws, SSO, роль), прапорці = false.
  access_key = local.use_emulator ? "test" : null
  secret_key = local.use_emulator ? "test" : null

  skip_credentials_validation = local.use_emulator
  skip_metadata_api_check     = local.use_emulator
  skip_requesting_account_id  = local.use_emulator # П12
  s3_use_path_style           = local.use_emulator

  # Для AWS for_each порожній, тож блоку endpoints немає взагалі.
  dynamic "endpoints" {
    for_each = local.use_emulator ? [var.aws_endpoint] : []

    content {
      apigateway = endpoints.value # REST API, /webhook, метод, інтеграція, деплой, стадія
      iam        = endpoints.value # роль та її inline-політика
      lambda     = endpoints.value # функція та aws_lambda_permission
      logs       = endpoints.value # log group функції
      sts        = endpoints.value # П11: запобіжник, прямих викликів немає
    }
  }
}
```

### `variables.tf`

```hcl
# П3: значення за замовчуванням обрав я; без -var конфігурація працює з Floci.

variable "aws_region" {
  description = "Регіон AWS (той самий регіон використовується і в емуляторі)."
  type        = string
  default     = "us-east-1"
}

variable "function_name" {
  description = "Ім'я Lambda-функції; від нього утворюються імена ролі, log group і API."
  type        = string
  default     = "webhook"

  validation {
    # Ліміт Lambda — 64 символи, але ім'я ролі "<function_name>-role" теж має вкластися в 64.
    condition     = can(regex("^[A-Za-z0-9_-]{1,59}$", var.function_name))
    error_message = "Ім'я функції: 1-59 символів, лише латинські літери, цифри, '-' і '_'."
  }
}

variable "aws_endpoint" {
  description = "Ендпоїнт емулятора Floci. Порожній рядок або null — справжня AWS без блоку endpoints."
  type        = string
  default     = "http://localhost:4566"
}
```

### `main.tf`

```hcl
locals {
  # П1, П2: архів зібраний заздалегідь, у його корені лежить виконуваний bootstrap.
  lambda_zip = "${path.module}/../build/webhook.zip"
}

# ---------- IAM: роль, яка може лише писати логи цієї функції ----------

data "aws_iam_policy_document" "lambda_assume_role" {
  statement {
    effect  = "Allow"
    actions = ["sts:AssumeRole"]

    principals {
      type        = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

resource "aws_iam_role" "lambda" {
  name               = "${var.function_name}-role"
  assume_role_policy = data.aws_iam_policy_document.lambda_assume_role.json
}

# П4: групу створює Terraform, тож ролі не потрібен logs:CreateLogGroup.
resource "aws_cloudwatch_log_group" "lambda" {
  name              = "/aws/lambda/${var.function_name}"
  retention_in_days = 14
}

data "aws_iam_policy_document" "lambda_logs" {
  statement {
    sid    = "WriteOwnLogStreams"
    effect = "Allow"
    actions = [
      "logs:CreateLogStream",
      "logs:PutLogEvents",
    ]
    # П5: ":*" означає будь-який потік лише цієї групи; імена потоків генерує Lambda.
    resources = ["${aws_cloudwatch_log_group.lambda.arn}:*"]
  }
}

resource "aws_iam_role_policy" "lambda_logs" {
  name   = "${var.function_name}-logs"
  role   = aws_iam_role.lambda.name
  policy = data.aws_iam_policy_document.lambda_logs.json
}

# ---------- Lambda ----------

resource "aws_lambda_function" "webhook" {
  function_name = var.function_name
  role          = aws_iam_role.lambda.arn

  runtime       = "provided.al2023"
  architectures = ["x86_64"]
  handler       = "bootstrap"

  filename         = local.lambda_zip
  source_code_hash = filebase64sha256(local.lambda_zip)

  timeout     = 10
  memory_size = 256

  # Група логів і права на неї мають з'явитися до першого запуску функції.
  depends_on = [
    aws_cloudwatch_log_group.lambda,
    aws_iam_role_policy.lambda_logs,
  ]
}

# ---------- API Gateway (REST, v1): POST /webhook -> Lambda ----------

# П7: endpoint_configuration не задано, тож тип ендпоїнта за замовчуванням (EDGE).
resource "aws_api_gateway_rest_api" "webhook" {
  name = "${var.function_name}-api"
}

resource "aws_api_gateway_resource" "webhook" {
  rest_api_id = aws_api_gateway_rest_api.webhook.id
  parent_id   = aws_api_gateway_rest_api.webhook.root_resource_id
  path_part   = "webhook"
}

resource "aws_api_gateway_method" "webhook_post" {
  rest_api_id   = aws_api_gateway_rest_api.webhook.id
  resource_id   = aws_api_gateway_resource.webhook.id
  http_method   = "POST"
  authorization = "NONE" # П6
}

resource "aws_api_gateway_integration" "webhook_lambda" {
  rest_api_id = aws_api_gateway_rest_api.webhook.id
  resource_id = aws_api_gateway_resource.webhook.id
  http_method = aws_api_gateway_method.webhook_post.http_method
  type        = "AWS_PROXY"
  uri         = aws_lambda_function.webhook.invoke_arn

  # Lambda завжди викликається POST-ом, незалежно від методу клієнта.
  integration_http_method = "POST"
}

resource "aws_api_gateway_deployment" "webhook" {
  rest_api_id = aws_api_gateway_rest_api.webhook.id

  # П9: новий деплой, коли змінюється маршрут або ціль інтеграції.
  # Ці посилання ще й задають порядок: деплой створюється після інтеграції.
  triggers = {
    redeployment = sha1(jsonencode([
      aws_api_gateway_resource.webhook.id,
      aws_api_gateway_method.webhook_post.id,
      aws_api_gateway_integration.webhook_lambda.id,
      aws_api_gateway_integration.webhook_lambda.uri,
    ]))
  }

  lifecycle {
    create_before_destroy = true
  }
}

resource "aws_api_gateway_stage" "dev" {
  rest_api_id   = aws_api_gateway_rest_api.webhook.id
  deployment_id = aws_api_gateway_deployment.webhook.id
  stage_name    = "dev"
}

# П8: викликати функцію може лише цей API, стадія dev, POST /webhook:
# source_arn = <execution_arn>/dev/POST/webhook
resource "aws_lambda_permission" "apigw_invoke" {
  statement_id  = "AllowAPIGatewayInvoke"
  action        = "lambda:InvokeFunction"
  function_name = aws_lambda_function.webhook.function_name
  principal     = "apigateway.amazonaws.com"
  source_arn    = "${aws_api_gateway_rest_api.webhook.execution_arn}/${aws_api_gateway_stage.dev.stage_name}/${aws_api_gateway_method.webhook_post.http_method}${aws_api_gateway_resource.webhook.path}"
}
```

### `outputs.tf`

```hcl
output "function_name" {
  value = aws_lambda_function.webhook.function_name
}

output "rest_api_id" {
  value = aws_api_gateway_rest_api.webhook.id
}

# П14: для емулятора шлях у форматі LocalStack.
output "webhook_url" {
  description = "URL для POST-запитів на вебхук."
  value = (local.use_emulator
    ? "${trimsuffix(var.aws_endpoint, "/")}/restapis/${aws_api_gateway_rest_api.webhook.id}/${aws_api_gateway_stage.dev.stage_name}/_user_request_${aws_api_gateway_resource.webhook.path}"
    : "${aws_api_gateway_stage.dev.invoke_url}${aws_api_gateway_resource.webhook.path}"
  )
}
```

## Які ресурси створюються

`terraform plan` має показати `Plan: 11 to add`.

| Ресурс | Що робить |
|---|---|
| `aws_iam_role.lambda` | Роль виконання `<name>-role`. Взяти її на себе може лише сервіс `lambda.amazonaws.com`. Managed-політик немає. |
| `aws_iam_role_policy.lambda_logs` | Inline-політика ролі: `logs:CreateLogStream` і `logs:PutLogEvents` лише на потоки групи `/aws/lambda/<name>`. Інших прав роль не має. |
| `aws_cloudwatch_log_group.lambda` | Група `/aws/lambda/<name>`, куди Lambda пише логи за замовчуванням. Логи зберігаються 14 днів. |
| `aws_lambda_function.webhook` | Функція: `provided.al2023`, `x86_64`, 10 с, 256 МБ, код із `../build/webhook.zip`. `source_code_hash` змінюється разом із вмістом архіву, і так Terraform бачить, що код треба завантажити знову. `handler` для `provided.*` лише передається у змінну `_HANDLER`, а запускається завжди файл `bootstrap`. |
| `aws_api_gateway_rest_api.webhook` | REST API (v1) з назвою `<name>-api`. |
| `aws_api_gateway_resource.webhook` | Шлях `/webhook` під коренем API. |
| `aws_api_gateway_method.webhook_post` | `POST` на `/webhook`, без авторизації. |
| `aws_api_gateway_integration.webhook_lambda` | `AWS_PROXY`: API Gateway передає весь HTTP-запит у функцію як подію (payload 1.0) і повертає клієнту її відповідь без перетворень. |
| `aws_api_gateway_deployment.webhook` | Знімок конфігурації API. Без нього стадії нічого обслуговувати. |
| `aws_api_gateway_stage.dev` | Стадія `dev`, що вказує на деплой, з URL `https://<id>.execute-api.<region>.amazonaws.com/dev/webhook`. Вона окремим ресурсом, бо `stage_name` усередині `aws_api_gateway_deployment` застарів. |
| `aws_lambda_permission.apigw_invoke` | Resource-based policy функції: `apigateway.amazonaws.com` може викликати її лише з цього API, стадії `dev` і `POST /webhook`. Без цього дозволу API Gateway отримає відмову, а клієнт побачить 500. |

Два `data "aws_iam_policy_document"` не є ресурсами AWS. Terraform сам формує з них JSON політик, без запитів до API.

Порядок Terraform виводить із посилань: роль → log group і політика → функція → інтеграція → деплой → стадія → дозвіл. REST API, `/webhook` і метод створюються паралельно з IAM.

## Floci чи справжня AWS

| `aws_endpoint` | Що отримує провайдер |
|---|---|
| `http://localhost:4566` (за замовчуванням) | ключі `test`, усі `skip_*` = true, `s3_use_path_style` = true, блок `endpoints` |
| `""` або `null` (у `.tfvars`) | блоку `endpoints` немає, ключі беруться зі стандартного ланцюжка, `skip_*` = false |

```sh
# 1. Пакет (з кореня проєкту; ./cmd/webhook — умовний шлях до main-пакета)
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags lambda.norpc -o build/bootstrap ./cmd/webhook
(cd build && zip -q webhook.zip bootstrap)

# 2. Floci (значення за замовчуванням)
cd infra
terraform init
terraform workspace new floci
terraform apply
curl -i -X POST "$(terraform output -raw webhook_url)" -H 'Content-Type: application/json' -d '{"ping":true}'

# 3. Справжня AWS
terraform workspace new aws
terraform apply -var 'aws_endpoint='
```

Для кожного режиму тримай окремий workspace, тобто окремий state, щоб `apply` в AWS не взяв state емулятора. Через `-var` для рядкової змінної передати `null` не вийде: `-var 'aws_endpoint=null'` дасть рядок `"null"`. Тому в командному рядку використовуй порожній рядок, а `null` лише у `.tfvars`.

## Припущення

**Пакет і розміщення**
- **П1.** Terraform-файли лежать у каталозі поруч із `build/`, бо шлях `../build/webhook.zip` рахується від `path.module`. Архів має існувати до `terraform plan`, бо `filebase64sha256` читає його вже на етапі плану.
- **П2.** У корені архіву лежить виконуваний `bootstrap`, зібраний під `linux/amd64` (це і є `x86_64`). Код використовує `aws-lambda-go` і повертає `events.APIGatewayProxyResponse`. Інакше `AWS_PROXY` віддасть клієнту 502 `Malformed Lambda proxy response`.

**Змінні**
- **П3.** Значення за замовчуванням `us-east-1`, `webhook` і `http://localhost:4566` обрав я. `function_name` обмежено 59 символами, бо ім'я ролі `<name>-role` має вкластися в ліміт IAM у 64 символи.

**IAM і логи**
- **П4.** Log group у завданні не було. Я її додав, щоб ролі не був потрібен `logs:CreateLogGroup`. Термін зберігання 14 днів теж обрав я; без нього логи зберігаються безстроково. Ціна такого рішення: якщо групу видалити вручну, функція не створить її знову, і логи губитимуться до наступного `terraform apply`.
- **П5.** `Resource = "*"` не знадобився взагалі. Суфікс `:*` в ARN означає «будь-який потік цієї однієї групи». Імена потоків Lambda створює під час роботи, тож вужче обмежити не можна. Інших прав функція не має: якщо код вебхука звертається до S3, SQS, Secrets Manager тощо, права треба додати.

**API Gateway**
- **П6.** `authorization = "NONE"`, тобто вебхук публічний. Справжність запиту, наприклад HMAC-підпис у заголовку, перевіряє сама функція. API-ключа, WAF і обмеження частоти запитів (throttling) немає.
- **П7.** Тип ендпоїнта не задано, тож AWS створить `EDGE` за замовчуванням. Для вебхука в одному регіоні краще підходить `endpoint_configuration { types = ["REGIONAL"] }`. Я його не додав, щоб не залежати від того, чи підтримує це Floci.
- **П8.** `source_arn` звужено до `<execution_arn>/dev/POST/webhook`. Через це кнопка Test у консолі API Gateway отримає відмову, бо вона викликає функцію через `test-invoke-stage`. Якщо вона потрібна, заміни `dev` на `*`.
- **П9.** Новий деплой створюється, коли змінюються ресурс, метод, інтеграція або її `uri`. Зміна атрибута методу на місці, наприклад `authorization`, нового деплою не запускає. Тоді потрібен `terraform apply -replace=aws_api_gateway_deployment.webhook`.

**Провайдер і Floci**
- **П10.** Це відхилення від буквального завдання: ключі `test`, `skip_*` і `s3_use_path_style` теж вмикаються лише для емулятора. Якби вони лишились і для AWS, ключі `test` замінили б справжні облікові дані, і кожен запит падав би з `InvalidClientTokenId`.
- **П11.** Ендпоїнти задано для сервісів, до яких звертається конфігурація: `apigateway`, `iam`, `lambda`, `logs`. `sts` доданий як запобіжник. З `skip_credentials_validation` і `skip_requesting_account_id` провайдер до STS не звертається. Але якщо хтось додасть `data "aws_caller_identity"`, запит піде в емулятор, а не в AWS, де ключі `test` дали б помилку. Ендпоїнта `s3` немає, бо S3 тут не використовується. `s3_use_path_style = true` залишено за завданням, але в цій конфігурації він ні на що не впливає.
- **П12.** Через `skip_requesting_account_id = true` провайдер не знає account ID емулятора. `execution_arn` він збирає сам, тож `source_arn` дозволу вийде з порожнім акаунтом: `arn:aws:execute-api:us-east-1::<api-id>/dev/POST/webhook`. Я припускаю, що Floci не перевіряє resource-policy функції при виклику з API Gateway. Якщо перевіряє і виклик через API отримує відмову, постав для емулятора `skip_requesting_account_id = false`. Тоді провайдер запитає account ID в STS емулятора, ендпоїнт для цього вже є.
- **П13.** Я припускаю, що Floci підтримує всі API, які провайдер v6 викликає для цих ресурсів, і справді запускає `provided.al2023`, найімовірніше через Docker. IAM-політики емулятор, найімовірніше, не застосовує, тож мінімальність прав перевіряється лише на справжній AWS. Нічого з цього я не перевіряв.
- **П14.** Локальний URL `<endpoint>/restapis/<api-id>/dev/_user_request_/webhook` має формат LocalStack. Я припускаю, що Floci з ним сумісний. Якщо ні, `rest_api_id` є в outputs, а правильний шлях треба взяти з документації Floci.

**Версії**
- **П15.** `required_version = ">= 1.5.0"` — це моя обережна нижня межа. Конфігурація не використовує нічого, що з'явилося в новіших версіях Terraform.
