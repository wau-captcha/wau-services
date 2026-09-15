package biz

import (
	"strings"
	"testing"
)

func TestNewCode(t *testing.T) {
	code := newCode()
	if len(code) != codeLength || strings.Trim(code, codeChars) != "" {
		t.Fatalf("newCode() = %q, want %d chars from %q", code, codeLength, codeChars)
	}
}
