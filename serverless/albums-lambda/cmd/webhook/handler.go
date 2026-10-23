package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambdacontext"
)

const supportedEvent = "album.submitted"

// webhookPayload is the envelope of a webhook call. Data stays raw until the event type is known,
// because its shape depends on the event (slide 19).
type webhookPayload struct {
	Event string          `json:"event"`
	Data  json.RawMessage `json:"data"`
}

// handler is the Lambda entry point behind the API Gateway HTTP trigger.
func handler(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	return handle(ctx, req, cfg, time.Now())
}

// handle validates and normalizes one album.submitted call. A client mistake comes back as a 400
// response with a nil error: a non-nil error would make API Gateway answer 502 (slide 19).
func handle(ctx context.Context, req events.APIGatewayProxyRequest, cfg Config, now time.Time) (events.APIGatewayProxyResponse, error) {
	requestID := requestIDFrom(ctx, req)
	log := logger.With("request_id", requestID)

	body := req.Body
	if req.IsBase64Encoded {
		decoded, err := base64.StdEncoding.DecodeString(body)
		if err != nil {
			log.WarnContext(ctx, "invalid base64 body", "error", err)
			return respond(requestID, 400, map[string]any{"error": "body is not valid base64"}), nil
		}
		body = string(decoded)
	}
	if strings.TrimSpace(body) == "" {
		log.WarnContext(ctx, "empty body")
		return respond(requestID, 400, map[string]any{"error": "request body is empty"}), nil
	}

	var payload webhookPayload
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		log.WarnContext(ctx, "invalid json", "error", err)
		return respond(requestID, 400, map[string]any{"error": "invalid json"}), nil
	}
	if payload.Event != supportedEvent {
		log.WarnContext(ctx, "unsupported event", "event", payload.Event)
		return respond(requestID, 400, map[string]any{"error": "unsupported event", "event": payload.Event}), nil
	}

	// null, a string or an array would decode into an empty Album and come back as a list of
	// "required" problems; name the actual mistake instead.
	if !bytes.HasPrefix(bytes.TrimSpace(payload.Data), []byte("{")) {
		log.WarnContext(ctx, "data is not an object")
		return respond(requestID, 400, map[string]any{"error": "data must be a JSON object"}), nil
	}
	var in Album
	if err := json.Unmarshal(payload.Data, &in); err != nil {
		var typeErr *json.UnmarshalTypeError
		if errors.As(err, &typeErr) {
			log.WarnContext(ctx, "invalid field type", "field", typeErr.Field, "error", err)
			return respond(requestID, 400, map[string]any{"error": "invalid field type", "field": typeErr.Field}), nil
		}
		log.WarnContext(ctx, "invalid data", "error", err)
		return respond(requestID, 400, map[string]any{"error": "data must be a JSON object"}), nil
	}

	album, problems := normalize(in, cfg, now)
	if len(problems) > 0 {
		log.WarnContext(ctx, "validation failed", "problems", problems)
		return respond(requestID, 400, map[string]any{"error": "validation failed", "details": problems}), nil
	}
	log.InfoContext(ctx, "album processed", "title", album.Title)
	return respond(requestID, 200, map[string]any{"status": "processed", "album": album}), nil
}

// requestIDFrom prefers the Lambda request id, then the API Gateway one; direct calls from tests
// have neither and get "local".
func requestIDFrom(ctx context.Context, req events.APIGatewayProxyRequest) string {
	if lc, ok := lambdacontext.FromContext(ctx); ok && lc.AwsRequestID != "" {
		return lc.AwsRequestID
	}
	if req.RequestContext.RequestID != "" {
		return req.RequestContext.RequestID
	}
	return "local"
}

func respond(requestID string, status int, v any) events.APIGatewayProxyResponse {
	b, _ := json.Marshal(v) // maps of strings, string slices and Album values always marshal
	return events.APIGatewayProxyResponse{
		StatusCode: status,
		Headers: map[string]string{
			"Content-Type": "application/json",
			"X-Request-Id": requestID,
		},
		Body: string(b),
	}
}
