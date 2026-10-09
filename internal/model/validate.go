package model

import (
	"errors"
	"fmt"
	"strings"
)

// ErrValidation marks errors that become a 422 response.
var ErrValidation = errors.New("validation error")

type field struct{ name, value string }

// blankFields returns the names of the fields whose value is empty or whitespace.
// Missing, null, "" and "   " all end up as an empty string after TrimSpace.
// Wrong JSON types never get here: the decoder rejects them first.
func blankFields(fields ...field) []string {
	var missing []string
	for _, f := range fields {
		if strings.TrimSpace(f.value) == "" {
			missing = append(missing, f.name)
		}
	}
	return missing
}

// requiredProblem describes blank required fields, or is empty when all are present.
func requiredProblem(fields ...field) []string {
	if missing := blankFields(fields...); len(missing) > 0 {
		return []string{"required fields missing or empty: " + strings.Join(missing, ", ")}
	}
	return nil
}

// validationError joins the problems into one ErrValidation, or returns nil.
func validationError(problems []string) error {
	if len(problems) == 0 {
		return nil
	}
	return fmt.Errorf("%w: %s", ErrValidation, strings.Join(problems, "; "))
}
