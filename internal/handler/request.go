package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
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
