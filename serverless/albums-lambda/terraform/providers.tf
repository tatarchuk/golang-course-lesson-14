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
