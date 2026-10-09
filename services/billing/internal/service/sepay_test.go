package service

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"billing/internal/conf"
)

func TestSepayIPNRefusesLargeBodies(t *testing.T) {
	ipn := NewSepayIPN(nil, &conf.Sepay{SecretKey: "ipn-secret"})
	body := `{"notification_type":"ORDER_PAID","padding":"` + strings.Repeat("a", 20<<10) + `"}`
	request := httptest.NewRequest(http.MethodPost, SepayIPNPath, strings.NewReader(body))
	request.Header.Set("X-Secret-Key", "ipn-secret")
	recorder := httptest.NewRecorder()
	ipn.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusRequestEntityTooLarge || !strings.Contains(recorder.Body.String(), `"success":false`) {
		t.Fatalf("got %d %s", recorder.Code, recorder.Body.String())
	}
}
