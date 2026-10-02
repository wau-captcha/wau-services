package biz

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"math"
	mrand "math/rand/v2"
	"strconv"
)

const (
	// The background is drawn at its display size in CSS pixels, like motion_noise.
	slideWidth  = 300
	slideHeight = 150
	// The hole's left edge keeps this clear of where the piece starts, at x = 0, so a drag
	// always has some way to go; and this much from the right edge.
	holeStartGap  = 20
	holeRightGap  = 8
	holeTopMargin = 6
	// A JPEG blurs the edges of the holes a little, and keeps the background small.
	slideJPEGQuality = 80
)

// generateSlidePuzzle draws a new background and piece; the answer is the left edge of the hole.
func generateSlidePuzzle() (string, Media, error) {
	random := newRandomSource()
	shape := newPieceShape(random)
	decoy := newPieceShape(random)
	for decoy.sameTabsAs(shape) {
		decoy = newPieceShape(random)
	}
	x, y, decoyX, decoyY := placeHoles(random, shape, decoy)

	background := drawBackground(random)
	piece := cutPiece(background, shape, x, y)
	carveHole(background, shape, x, y)
	carveHole(background, decoy, decoyX, decoyY)

	var backgroundJPEG, piecePNG bytes.Buffer
	if err := jpeg.Encode(&backgroundJPEG, background.image(), &jpeg.Options{Quality: slideJPEGQuality}); err != nil {
		return "", Media{}, err
	}
	if err := png.Encode(&piecePNG, piece); err != nil {
		return "", Media{}, err
	}
	return strconv.Itoa(x), Media{
		Data:     backgroundJPEG.Bytes(),
		MIMEType: "image/jpeg",
		Width:    slideWidth,
		Height:   slideHeight,
		Piece:    &Piece{Data: piecePNG.Bytes(), Y: y, Width: shape.width, Height: shape.height},
	}, nil
}

// placeHoles puts the hole at a random spot in range and the decoy at least a piece's width
// away from it, preferably clear of the piece's starting place as well.
func placeHoles(random *mrand.Rand, shape, decoy pieceShape) (x, y, decoyX, decoyY int) {
	x = shape.width + holeStartGap + random.IntN(slideWidth-holeRightGap-2*shape.width-holeStartGap+1)
	y = holeTopMargin + random.IntN(slideHeight-2*holeTopMargin-shape.height+1)
	gap := max(shape.width, decoy.width)
	var clear, any []int
	for candidate := holeTopMargin; candidate <= slideWidth-holeTopMargin-decoy.width; candidate++ {
		if candidate > x-gap && candidate < x+gap {
			continue
		}
		any = append(any, candidate)
		if candidate >= shape.width+holeTopMargin {
			clear = append(clear, candidate)
		}
	}
	if len(clear) > 0 {
		any = clear
	}
	decoyX = any[random.IntN(len(any))]
	decoyY = holeTopMargin + random.IntN(slideHeight-2*holeTopMargin-decoy.height+1)
	return x, y, decoyX, decoyY
}

// cutPiece copies the piece out of the background at (x, y), with a light outline and a soft
// shadow so it stands out wherever it is dragged.
func cutPiece(background *canvas, shape pieceShape, x, y int) *image.NRGBA {
	cover := shape.coverage(0, 0)
	shadow := blurCover(shape.coverage(-1, -1), shape.width, shape.height)
	piece := image.NewNRGBA(image.Rect(0, 0, shape.width, shape.height))
	for j := range shape.height {
		for i := range shape.width {
			alpha := cover[j*shape.width+i]
			edge := alpha - minAround(cover, shape.width, shape.height, i, j)
			shade := 0.45 * shadow[j*shape.width+i] * (1 - alpha)
			total := alpha + shade
			if total <= 0 {
				continue
			}
			red, green, blue := background.at(x+i, y+j)
			lighten := func(channel float64) float64 { return channel + (255-channel)*0.75*edge }
			// Over the shadow, which is black: only the piece adds colour.
			piece.SetNRGBA(i, j, color.NRGBA{
				R: clampByte(lighten(red) * alpha / total),
				G: clampByte(lighten(green) * alpha / total),
				B: clampByte(lighten(blue) * alpha / total),
				A: clampByte(255 * total),
			})
		}
	}
	return piece
}

// carveHole darkens the shape into the background at (x, y), with an inner bevel: a shadow
// along its top-left edge and a light rim along the bottom-right one.
func carveHole(background *canvas, shape pieceShape, x, y int) {
	cover := shape.coverage(0, 0)
	topLeft := shape.coverage(-1.5, -1.5)
	bottomRight := shape.coverage(1.5, 1.5)
	for j := range shape.height {
		for i := range shape.width {
			index := j*shape.width + i
			alpha := cover[index]
			if alpha == 0 {
				continue
			}
			shadowEdge, lightEdge := alpha*(1-topLeft[index]), alpha*(1-bottomRight[index])
			background.update(x+i, y+j, func(channel float64) float64 {
				channel *= (1 - 0.5*alpha) * (1 - 0.5*shadowEdge)
				return channel + (255-channel)*0.35*lightEdge
			})
		}
	}
}

func minAround(cover []float64, width, height, i, j int) float64 {
	lowest := 1.0
	for y := j - 1; y <= j+1; y++ {
		for x := i - 1; x <= i+1; x++ {
			value := 0.0
			if x >= 0 && x < width && y >= 0 && y < height {
				value = cover[y*width+x]
			}
			lowest = min(lowest, value)
		}
	}
	return lowest
}

// blurCover softens a coverage map with a 5 × 5 box blur.
func blurCover(cover []float64, width, height int) []float64 {
	blurred := make([]float64, len(cover))
	for j := range height {
		for i := range width {
			var sum float64
			for y := j - 2; y <= j+2; y++ {
				for x := i - 2; x <= i+2; x++ {
					if x >= 0 && x < width && y >= 0 && y < height {
						sum += cover[y*width+x]
					}
				}
			}
			blurred[j*width+i] = sum / 25
		}
	}
	return blurred
}

func clampByte(value float64) uint8 {
	return uint8(math.Round(min(max(value, 0), 255)))
}
