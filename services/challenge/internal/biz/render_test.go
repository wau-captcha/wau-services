package biz

import (
	"strings"
	"testing"
)

func TestNewCode(t *testing.T) {
	for _, chars := range []string{alphanumericChars, uppercaseChars} {
		code := newCode(chars)
		if len(code) != codeLength || strings.Trim(code, chars) != "" {
			t.Fatalf("newCode() = %q, want %d chars from %q", code, codeLength, chars)
		}
	}
}
