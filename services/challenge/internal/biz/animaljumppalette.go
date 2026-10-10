package biz

import (
	"bytes"
	"image"
	"image/color"
	"slices"
)

// A pixel this far (squared, in RGB) from every paint is an anti-aliased blend worth its own entry.
const jumpBlendDistance = 20 * 20

// jumpPalette is the one palette of a round: transparent, then the flat paints the round was
// drawn with, then the blends its anti-aliased edges need most.
type jumpPalette struct {
	colors color.Palette
	lookup []uint8 // the nearest entry by colour at 6 bits a channel; 255 until worked out
}

func newJumpPalette(frame *image.RGBA, paints []color.NRGBA) *jumpPalette {
	palette := &jumpPalette{colors: color.Palette{color.RGBA{}}, lookup: bytes.Repeat([]byte{255}, 1<<18)}
	for _, paint := range paints[:min(len(paints), jumpPaletteSize-1)] {
		palette.colors = append(palette.colors, color.RGBA{paint.R, paint.G, paint.B, 255})
	}
	var blends [][3]uint8
	for i := 0; i < len(frame.Pix); i += 4 {
		pixel := [3]uint8{frame.Pix[i], frame.Pix[i+1], frame.Pix[i+2]}
		if _, distance := palette.nearest(pixel); distance > jumpBlendDistance {
			blends = append(blends, pixel)
		}
	}
	for _, blend := range medianCut(blends, jumpPaletteSize-len(palette.colors)) {
		palette.colors = append(palette.colors, color.RGBA{blend[0], blend[1], blend[2], 255})
	}
	return palette
}

// nearest is the opaque entry closest to pixel, and its squared distance.
func (p *jumpPalette) nearest(pixel [3]uint8) (uint8, int) {
	best, bestDistance := uint8(1), 1<<30
	for i := 1; i < len(p.colors); i++ {
		entry := p.colors[i].(color.RGBA)
		dr, dg, db := int(entry.R)-int(pixel[0]), int(entry.G)-int(pixel[1]), int(entry.B)-int(pixel[2])
		if distance := dr*dr + dg*dg + db*db; distance < bestDistance {
			best, bestDistance = uint8(i), distance
		}
	}
	return best, bestDistance
}

// quantize maps an opaque frame to palette indices; index 0, transparent, is never used here.
func (p *jumpPalette) quantize(frame *image.RGBA) []uint8 {
	indices := make([]uint8, len(frame.Pix)/4)
	for i := range indices {
		pixel := [3]uint8{frame.Pix[4*i], frame.Pix[4*i+1], frame.Pix[4*i+2]}
		key := int(pixel[0]>>2)<<12 | int(pixel[1]>>2)<<6 | int(pixel[2]>>2)
		if p.lookup[key] == 255 {
			p.lookup[key], _ = p.nearest(pixel)
		}
		indices[i] = p.lookup[key]
	}
	return indices
}

// medianCut splits the pixels into at most count boxes, each time halving the box with the widest
// spread of one channel weighed by its pixels, and returns each box's mean colour.
func medianCut(pixels [][3]uint8, count int) [][3]uint8 {
	if count <= 0 || len(pixels) == 0 {
		return nil
	}
	boxes := [][][3]uint8{pixels}
	for len(boxes) < count {
		widest, channel, score := -1, 0, 0
		for i, box := range boxes {
			for c := range 3 {
				lowest, highest := uint8(255), uint8(0)
				for _, pixel := range box {
					lowest, highest = min(lowest, pixel[c]), max(highest, pixel[c])
				}
				if spread := int(highest-lowest) * len(box); highest > lowest && spread > score {
					widest, channel, score = i, c, spread
				}
			}
		}
		if widest < 0 {
			break
		}
		box := boxes[widest]
		slices.SortFunc(box, func(a, b [3]uint8) int { return int(a[channel]) - int(b[channel]) })
		half := len(box) / 2
		boxes[widest] = box[:half]
		boxes = append(boxes, box[half:])
	}
	means := make([][3]uint8, len(boxes))
	for i, box := range boxes {
		var sum [3]int
		for _, pixel := range box {
			sum[0], sum[1], sum[2] = sum[0]+int(pixel[0]), sum[1]+int(pixel[1]), sum[2]+int(pixel[2])
		}
		means[i] = [3]uint8{uint8(sum[0] / len(box)), uint8(sum[1] / len(box)), uint8(sum[2] / len(box))}
	}
	return means
}
