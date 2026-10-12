package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNewChallengeObjectRotate(t *testing.T) {
	server := newChallengeServer(t)
	request := httptest.NewRequest(http.MethodGet, "/challenge/new?site_key=pk_shop&challenge_type=object_rotate", nil)
	request.RemoteAddr = "203.0.113.4:5123"
	recorder := httptest.NewRecorder()
	server.ServeHTTP(recorder, request)
	body := map[string]any{}
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil || recorder.Code != 200 {
		t.Fatalf("got %d %s", recorder.Code, recorder.Body.String())
	}
	if body["challenge_type"] != "object_rotate" || body["mime_type"] != "image/jpeg" || body["width"] != float64(300) ||
		body["height"] != float64(150) || body["frame_count"] != float64(8) {
		t.Fatalf("reply %v", body)
	}
	for _, field := range []string{"image", "second_image"} {
		encoded, _ := body[field].(string)
		data, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			t.Fatalf("%s: %v", field, err)
		}
		// Eight frames of 300 × 150 side by side, drawn at 2×.
		config, err := jpeg.DecodeConfig(bytes.NewReader(data))
		if err != nil || config.Width != 8*300*2 || config.Height != 150*2 {
			t.Fatalf("%s is not a strip of eight frames: %+v %v", field, config, err)
		}
	}
	if _, ok := body["piece_image"]; ok {
		t.Fatal("piece fields on an object_rotate challenge")
	}
	// The other types are unchanged: no frame_count, and second_image for animal_jump only.
	_, other := newChallenge(server, http.MethodGet)
	if _, ok := other["frame_count"]; ok || other["challenge_type"] != "image_distort" {
		t.Fatalf("image_distort reply %v", other)
	}
	request = httptest.NewRequest(http.MethodGet, "/challenge/new?site_key=pk_shop&challenge_type=animal_jump", nil)
	request.RemoteAddr = "203.0.113.4:5123"
	recorder = httptest.NewRecorder()
	server.ServeHTTP(recorder, request)
	jump := map[string]any{}
	_ = json.Unmarshal(recorder.Body.Bytes(), &jump)
	if _, ok := jump["frame_count"]; ok || jump["second_image"] == nil {
		t.Fatalf("animal_jump reply carries frame_count or lacks second_image: %d", recorder.Code)
	}
}
