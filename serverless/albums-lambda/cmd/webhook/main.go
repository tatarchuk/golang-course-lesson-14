// Command webhook is the album.submitted Lambda of Lesson 12: it takes an album submission over an
// API Gateway HTTP trigger, cleans it up and validates it against the albums catalogue rules.
package main

import (
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
)

// Package-level values are created once, in the init phase of a cold start, and reused by every
// warm invocation of the same sandbox (slides 9, 22 and 23).
var (
	// Structured JSON on stdout ends up in CloudWatch Logs without extra setup.
	logger = slog.New(slog.NewJSONHandler(os.Stdout, nil))
	cfg    = mustLoadConfig(os.Getenv)
)

// main only hands control to the SDK; the logic lives in the handler (slide 18).
func main() {
	lambda.Start(handler)
}
