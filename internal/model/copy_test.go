package model

import (
	"errors"
	"strings"
	"testing"
)

func TestCopyInputValidateRequiresInventoryNo(t *testing.T) {
	err := CopyInput{InventoryNo: " "}.Validate()
	if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), "inventory_no") {
		t.Fatalf("expected ErrValidation naming inventory_no, got %v", err)
	}
}

func TestCopyInputValidateRejectsUnknownStatus(t *testing.T) {
	err := CopyInput{InventoryNo: "INV-1", Status: "broken"}.Validate()
	if !errors.Is(err, ErrValidation) || !strings.Contains(err.Error(), "status") {
		t.Fatalf("expected ErrValidation naming status, got %v", err)
	}
}

func TestCopyInputAcceptsEveryKnownStatusAndEmpty(t *testing.T) {
	for _, s := range []string{"", CopyStatusAvailable, CopyStatusBorrowed, CopyStatusLost} {
		if err := (CopyInput{InventoryNo: "INV-1", Status: s}).Validate(); err != nil {
			t.Errorf("status=%q: expected no error, got %v", s, err)
		}
	}
}

func TestCopyInputStatusDefaultsToAvailable(t *testing.T) {
	if got := (CopyInput{InventoryNo: "INV-1"}).StatusOrDefault(); got != CopyStatusAvailable {
		t.Fatalf("expected %q, got %q", CopyStatusAvailable, got)
	}
	if got := (CopyInput{InventoryNo: "INV-1", Status: CopyStatusLost}).StatusOrDefault(); got != CopyStatusLost {
		t.Fatalf("expected %q, got %q", CopyStatusLost, got)
	}
}
