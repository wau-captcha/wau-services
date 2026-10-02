package biz

import (
	"math"
	mrand "math/rand/v2"
)

const (
	// The piece: a square body with a round tab on each side that sticks out or cuts in.
	pieceBody      = 44
	pieceTabRadius = 6.5
	// How far a tab's centre sits past its side, so the tab's neck is narrower than the tab.
	pieceTabReach = 4.5
	// Room around the piece for its shadow.
	piecePad = 3
)

// pieceShape is a jigsaw piece in a box of width × height: the body's top-left corner is at
// (left, top), and tabs holds the top, right, bottom and left tab.
type pieceShape struct {
	left, top     float64
	width, height int
	tabs          [4]pieceTab
}

type pieceTab struct {
	out    bool
	along  float64 // shift of the tab from the middle of its side
	radius float64
	reach  float64
}

// newPieceShape varies each tab's direction, place and size, so no two pieces are alike.
func newPieceShape(random *mrand.Rand) pieceShape {
	jitter := func(spread float64) float64 { return (random.Float64()*2 - 1) * spread }
	var shape pieceShape
	for side := range shape.tabs {
		shape.tabs[side] = pieceTab{
			out:    random.IntN(2) == 0,
			along:  jitter(4),
			radius: pieceTabRadius + jitter(0.6),
			reach:  pieceTabReach + jitter(0.6),
		}
	}
	extent := func(side int) float64 {
		if tab := shape.tabs[side]; tab.out {
			return tab.reach + tab.radius
		}
		return 0
	}
	shape.left, shape.top = piecePad+extent(3), piecePad+extent(0)
	shape.width = int(math.Ceil(shape.left + pieceBody + extent(1) + piecePad))
	shape.height = int(math.Ceil(shape.top + pieceBody + extent(2) + piecePad))
	return shape
}

// sameTabsAs reports whether both pieces have their tabs out and in on the same sides.
func (s pieceShape) sameTabsAs(other pieceShape) bool {
	for side, tab := range s.tabs {
		if tab.out != other.tabs[side].out {
			return false
		}
	}
	return true
}

// contains reports whether the point (px, py) of the box lies in the piece.
func (s pieceShape) contains(px, py float64) bool {
	inside := px >= s.left && px <= s.left+pieceBody && py >= s.top && py <= s.top+pieceBody
	for side, tab := range s.tabs {
		cx, cy := s.tabCentre(side)
		if (px-cx)*(px-cx)+(py-cy)*(py-cy) > tab.radius*tab.radius {
			continue
		}
		if tab.out {
			return true
		}
		inside = false
	}
	return inside
}

func (s pieceShape) tabCentre(side int) (float64, float64) {
	tab := s.tabs[side]
	reach := tab.reach
	if !tab.out {
		reach = -reach
	}
	middleX, middleY := s.left+pieceBody/2+tab.along, s.top+pieceBody/2+tab.along
	switch side {
	case 0:
		return middleX, s.top - reach
	case 1:
		return s.left + pieceBody + reach, middleY
	case 2:
		return middleX, s.top + pieceBody + reach
	default:
		return s.left - reach, middleY
	}
}

// coverage returns, per pixel of the box shifted by (dx, dy), the share of it inside the piece,
// sampled 4 × 4 for smooth edges.
func (s pieceShape) coverage(dx, dy float64) []float64 {
	cover := make([]float64, s.width*s.height)
	for j := range s.height {
		for i := range s.width {
			inside := 0
			for sy := range 4 {
				for sx := range 4 {
					if s.contains(float64(i)+dx+(float64(sx)+0.5)/4, float64(j)+dy+(float64(sy)+0.5)/4) {
						inside++
					}
				}
			}
			cover[j*s.width+i] = float64(inside) / 16
		}
	}
	return cover
}
