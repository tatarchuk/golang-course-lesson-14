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
