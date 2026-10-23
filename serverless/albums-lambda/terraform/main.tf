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
