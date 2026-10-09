package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
)

const maxBodyBytes = 1 << 20 // 1 MiB is plenty for one album

// decodeJSON reads the whole body and unmarshals it into dst.
// json.Unmarshal (unlike Decoder.Decode) rejects an empty body and trailing garbage, and it
// returns an error for a top-level array or for a field of the wrong type.
func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxBodyBytes))
	if err != nil {
		return err
	}
	return json.Unmarshal(body, dst)
}

// decodeMessage turns a decoding error into a client-safe message (no Go type names).
func decodeMessage(err error) string {
	var typeErr *json.UnmarshalTypeError
	if errors.As(err, &typeErr) && typeErr.Field != "" {
		return fmt.Sprintf("field %q has an invalid type", typeErr.Field)
	}
	return "request body must be a valid JSON object"
}

// parseID accepts an integer in the uint32 range (0..4294967295).
// ParseUint rejects "abc", "-1", "1.5" and anything that does not fit in 32 bits.
func parseID(raw string) (int64, bool) {
	n, err := strconv.ParseUint(raw, 10, 32)
	if err != nil {
		return 0, false
	}
	return int64(n), true
}

// parsePositiveInt reads an optional query parameter that must be an integer >= 1.
// Absent (or empty) means "use the default".
func parsePositiveInt(q url.Values, key string, def int) (int, bool) {
	raw := q.Get(key)
	if raw == "" {
		return def, true
	}
	n, err := strconv.ParseInt(raw, 10, strconv.IntSize)
	if err != nil && !errors.Is(err, strconv.ErrRange) {
		return 0, false // not a number at all ("abc", "1.5")
	}
	// On ErrRange ParseInt returns the clamped MaxInt/MinInt: "99999999999999999999" is a
	// legitimate integer that simply does not fit, so we keep MaxInt and let the caller treat
	// it as "far beyond the data" (page) or clamp it (limit). MinInt fails the next check.
	if n < 1 {
		return 0, false
	}
	return int(n), true
}

// parseOptionalID reads an optional query parameter that, when present and not empty,
// must be a positive integer (used for the genre_id filter). Absent means "no filter".
func parseOptionalID(q url.Values, key string) (int64, bool) {
	raw := q.Get(key)
	if raw == "" {
		return 0, true
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || n < 1 {
		return 0, false
	}
	return n, true
}

// validator is implemented by every request body type.
type validator interface {
	Validate() error
}

// readInput decodes the body into dst and validates it. On failure it writes the 422
// response and returns false, so a handler can simply return.
func readInput(w http.ResponseWriter, r *http.Request, dst validator) bool {
	if err := decodeJSON(w, r, dst); err != nil {
		WriteError(w, http.StatusUnprocessableEntity, CodeValidation, decodeMessage(err))
		return false
	}
	if err := dst.Validate(); err != nil {
		WriteError(w, http.StatusUnprocessableEntity, CodeValidation, err.Error())
		return false
	}
	return true
}

// pathID parses the {id} path value. On failure it writes the 400 response and returns false.
func pathID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, ok := parseID(r.PathValue("id"))
	if !ok {
		WriteError(w, http.StatusBadRequest, CodeInvalidID, msgInvalidID)
		return 0, false
	}
	return id, true
}

const msgInvalidID = "id must be an integer between 0 and 4294967295"
