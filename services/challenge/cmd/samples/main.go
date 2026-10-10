// Command samples writes fresh challenges of every type, made by the generators /challenge/new
// uses, with a manifest.json of their answers, so anyone can benchmark a solver against real
// output. Live challenges are generated anew for every request and can be answered once.
//
//	go run ./cmd/samples -out ../../../wau-dashboard/public/captchas/samples
package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"image/gif"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"challenge/internal/biz"
)

// entry is one sample in manifest.json. Paths are relative to the manifest.
type entry struct {
	File string `json:"file"`
	Type string `json:"type"`
	// The answer the service stores: the characters for the text types, for slide_puzzle the
	// x where the left edge of the piece must end, within TolerancePx.
	Answer string `json:"answer"`
	// Size to display the media at, in CSS pixels.
	Width  int `json:"width"`
	Height int `json:"height"`
	// motion_noise: the first frame as a PNG, on its own nothing but noise.
	Poster string `json:"poster,omitempty"`
	// slide_puzzle: the piece, where it sits and how close to TargetX it must end.
	PieceFile   string   `json:"piece_file,omitempty"`
	TargetX     *int     `json:"target_x,omitempty"`
	PieceY      *int     `json:"piece_y,omitempty"`
	PieceWidth  *int     `json:"piece_width,omitempty"`
	PieceHeight *int     `json:"piece_height,omitempty"`
	TolerancePx *float64 `json:"tolerance_px,omitempty"`
	// animal_jump: File is round 1, SecondFile round 2, Poster round 1's first frame. Rounds holds
	// each round's target: where a click counts and when its high jump peaks, in ms.
	SecondFile string      `json:"second_file,omitempty"`
	Rounds     []jumpRound `json:"rounds,omitempty"`
}

// jumpRound is one round of the stored animal_jump answer, {"r": [...]}.
type jumpRound struct {
	X0   float64 `json:"x0"`
	X1   float64 `json:"x1"`
	Y0   float64 `json:"y0"`
	Y1   float64 `json:"y1"`
	Apex float64 `json:"apex"`
}

var extensions = map[string]string{"image/png": ".png", "image/gif": ".gif", "image/jpeg": ".jpg"}

func main() {
	out := flag.String("out", "samples", "directory to write the samples and manifest.json into")
	counts := map[string]*int{
		biz.TypeImageDistort: flag.Int("image-distort", 10, "image_distort samples"),
		biz.TypeMotionNoise:  flag.Int("motion-noise", 5, "motion_noise samples"),
		biz.TypeSlidePuzzle:  flag.Int("slide-puzzle", 10, "slide_puzzle samples"),
		biz.TypeAnimalJump:   flag.Int("animal-jump", 5, "animal_jump samples, two rounds each"),
	}
	flag.Parse()
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatal(err)
	}
	var manifest []entry
	for _, challengeType := range []string{biz.TypeImageDistort, biz.TypeMotionNoise, biz.TypeSlidePuzzle, biz.TypeAnimalJump} {
		for i := 1; i <= *counts[challengeType]; i++ {
			sample, err := write(*out, challengeType, i)
			if err != nil {
				log.Fatalf("%s %d: %v", challengeType, i, err)
			}
			manifest = append(manifest, sample)
		}
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(*out, "manifest.json"), append(data, '\n'), 0o644); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("wrote %d samples and manifest.json to %s\n", len(manifest), *out)
}

func write(dir, challengeType string, index int) (entry, error) {
	answer, media, err := biz.Sample(challengeType)
	if err != nil {
		return entry{}, err
	}
	name := fmt.Sprintf("%s-%02d", challengeType, index)
	sample := entry{File: name + extensions[media.MIMEType], Type: challengeType, Answer: answer, Width: media.Width, Height: media.Height}
	if media.SecondRound != nil {
		return writeJump(dir, name, sample, media)
	}
	if err := os.WriteFile(filepath.Join(dir, sample.File), media.Data, 0o644); err != nil {
		return entry{}, err
	}
	if challengeType == biz.TypeMotionNoise {
		sample.Poster = name + "-frame.png"
		if err := writeFirstFrame(filepath.Join(dir, sample.Poster), media.Data); err != nil {
			return entry{}, err
		}
	}
	if piece := media.Piece; piece != nil {
		targetX, err := strconv.Atoi(answer)
		if err != nil {
			return entry{}, err
		}
		tolerance := float64(biz.SlideTolerancePx)
		sample.PieceFile = name + "-piece.png"
		sample.TargetX, sample.PieceY, sample.PieceWidth, sample.PieceHeight = &targetX, &piece.Y, &piece.Width, &piece.Height
		sample.TolerancePx = &tolerance
		if err := os.WriteFile(filepath.Join(dir, sample.PieceFile), piece.Data, 0o644); err != nil {
			return entry{}, err
		}
	}
	return sample, nil
}

// writeJump writes both rounds of an animal_jump sample, a poster of the first, and the targets.
func writeJump(dir, name string, sample entry, media biz.Media) (entry, error) {
	var stored struct {
		R []jumpRound `json:"r"`
	}
	if err := json.Unmarshal([]byte(sample.Answer), &stored); err != nil {
		return entry{}, err
	}
	sample.File, sample.SecondFile, sample.Poster, sample.Rounds = name+"-1.gif", name+"-2.gif", name+"-frame.png", stored.R
	for file, data := range map[string][]byte{sample.File: media.Data, sample.SecondFile: media.SecondRound} {
		if err := os.WriteFile(filepath.Join(dir, file), data, 0o644); err != nil {
			return entry{}, err
		}
	}
	return sample, writeFirstFrame(filepath.Join(dir, sample.Poster), media.Data)
}

// writeFirstFrame saves the first frame of a GIF as a PNG.
func writeFirstFrame(path string, data []byte) error {
	animation, err := gif.DecodeAll(bytes.NewReader(data))
	if err != nil {
		return err
	}
	var frame bytes.Buffer
	if err := png.Encode(&frame, animation.Image[0]); err != nil {
		return err
	}
	return os.WriteFile(path, frame.Bytes(), 0o644)
}
