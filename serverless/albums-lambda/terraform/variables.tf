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
