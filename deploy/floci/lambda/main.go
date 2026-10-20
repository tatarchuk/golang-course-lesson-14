// Command demo-api is the Go Lambda of deck Demo 2 (slides 33-36). It runs on the provided.al2023
// custom runtime: the binary is named bootstrap and is built for linux/arm64 (the VM's CPU).
package main

import (
	"context"
	"fmt"

	"github.com/aws/aws-lambda-go/lambda"
)

type Event struct {
	Name string `json:"name"`
}

type Response struct {
	StatusCode int    `json:"statusCode"`
	Body       string `json:"body"`
}

func handler(ctx context.Context, e Event) (Response, error) {
	name := e.Name
	if name == "" {
		name = "world"
	}
	return Response{StatusCode: 200, Body: fmt.Sprintf("hello, %s from floci", name)}, nil
}

func main() {
	lambda.Start(handler)
}
