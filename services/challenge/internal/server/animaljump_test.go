package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image/gif"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewChallengeAnimalJump(t *testing.T) {
	server := newChallengeServer(t)
	request := httptest.NewRequest(http.MethodGet, "/challenge/new?site_key=pk_shop&challenge_type=animal_jump", nil)
	request.RemoteAddr = "203.0.113.4:5123"
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)
	body := map[string]any{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil || recorder.Code != 200 {
		t.Fatalf("got %d %s", recorder.Code, recorder.Body.String())
	}
	if body["challenge_type"] != "animal_jump" || body["mime_type"] != "image/gif" || body["width"] != float64(300) || body["height"] != float64(150) {
		t.Fatalf("reply %v", body)
	}
	for _, field := range []string{"image", "second_image"} {
		encoded, _ := body[field].(string)
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatalf("%s: %v", field, err)
		}
		animation, err := gif.DecodeAll(bytes.NewReader(data))
		if err != nil || len(animation.Image) != 50 || animation.LoopCount != -1 {
			t.Fatalf("%s is not a 50-frame GIF played once: %v", field, err)
		}
	}
	if _, ok := body["piece_image"]; ok {
		t.Fatal("piece fields on an animal_jump challenge")
	}
	// The other types are unchanged: no second_image at all.
	_, other := newChallenge(server, http.MethodGet)
	if _, ok := other["second_image"]; ok || other["challenge_type"] != "image_distort" {
		t.Fatalf("image_distort reply %v", other)
	}
}
