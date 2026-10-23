# П3: значення за замовчуванням обрав я; без -var конфігурація працює з Floci.

variable "aws_region" {
  description = "Регіон AWS (той самий регіон використовується і в емуляторі)."
  type        = string
  default     = "us-east-1"
}

variable "function_name" {
  description = "Ім'я Lambda-функції; від нього утворюються імена ролі, log group і API."
  type        = string
  default     = "albums-webhook" # Review: was "webhook"; the name of this repository's function

  validation {
    # Ліміт Lambda — 64 символи, але ім'я ролі "<function_name>-role" теж має вкластися в 64.
    condition     = can(regex("^[A-Za-z0-9_-]{1,59}$", var.function_name))
    error_message = "Ім'я функції: 1-59 символів, лише латинські літери, цифри, '-' і '_'."
  }
}

# Review: added. The draft hard-coded architectures = ["x86_64"], as the prompt asked, but the binary is
# built for this machine (arm64 here) and scripts/env.sh exports TF_VAR_lambda_architecture with the
# value it builds for. Floci ignores the declared architecture by default, so the mismatch would only
# fail on real AWS (slide 32: runtime and architecture must match the build).
variable "lambda_architecture" {
  description = "Lambda architecture; must match the GOARCH of build/webhook.zip."
  type        = string
  default     = "arm64"

  validation {
    condition     = contains(["x86_64", "arm64"], var.lambda_architecture)
    error_message = "lambda_architecture must be x86_64 or arm64."
  }
}

variable "aws_endpoint" {
  description = "Ендпоїнт емулятора Floci. Порожній рядок або null — справжня AWS без блоку endpoints."
  type        = string
  default     = "http://localhost:4566"
}
