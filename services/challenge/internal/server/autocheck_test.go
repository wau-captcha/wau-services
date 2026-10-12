package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewChallengeAutoCheck(t *testing.T) {
	server := newChallengeServer(t)
	get := func(query string) map[string]any {
		t.Helper()
		request := httptest.NewRequest(http.MethodGet, "/challenge/new?site_key=pk_shop&"+query, nil)
		request.RemoteAddr = "203.0.113.4:5123"
		recorder := httptest.NewRecorder()
		server.ServeHTTP(recorder, request)
		body := map[string]any{}
		if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil || recorder.Code != 200 {
			t.Fatalf("%s: got %d %s", query, recorder.Code, recorder.Body.String())
		}
		return body
	}
	// The key allows image_distort, animal_jump and object_rotate, not slide_puzzle: the server
	// picks the first allowed of its order instead.
	body := get("challenge_type=auto_check&fallback_type=slide_puzzle")
	if body["challenge_type"] != "auto_check" || body["fallback_type"] != "image_distort" || body["pow_difficulty"] != float64(18) {
		t.Fatalf("auto_check reply %v", body)
	}
	// No image: the codec writes the zero values of the plain fields, and none of the optional ones.
	if body["image"] != "" || body["mime_type"] != "" || body["width"] != float64(0) || body["height"] != float64(0) {
		t.Fatalf("auto_check reply carries media: %v", body)
	}
	for _, field := range []string{"piece_image", "second_image", "frame_count"} {
		if _, ok := body[field]; ok {
			t.Fatalf("auto_check reply carries %s: %v", field, body)
		}
	}
	if salt, _ := body["pow_salt"].(string); salt == "" {
		t.Fatalf("auto_check reply without a proof of work: %v", body)
	}
	// A fallback the key allows is kept.
	if body := get("challenge_type=auto_check&fallback_type=animal_jump"); body["fallback_type"] != "animal_jump" {
		t.Fatalf("auto_check with an allowed fallback: %v", body)
	}
	// The other types are unchanged: no fallback_type, the usual proof of work.
	for _, query := range []string{"challenge_type=image_distort&fallback_type=animal_jump", "challenge_type=animal_jump"} {
		other := get(query)
		if _, ok := other["fallback_type"]; ok || other["pow_difficulty"] != float64(15) || other["image"] == nil {
			t.Fatalf("%s reply %v", query, other)
		}
	}
}
