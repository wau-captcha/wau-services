package biz

import (
	"crypto/rand"
	"image/color"
	mrand "math/rand/v2"

	"golang.org/x/image/font/gofont/gomonobold"
	"golang.org/x/image/font/opentype"
)

const (
	codeLength        = 5
	alphanumericChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789"
	uppercaseChars    = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
)

// codeFont is Go Mono Bold, which keeps 0/O and 1/l/I distinguishable.
var codeFont = func() *opentype.Font {
	parsed, err := opentype.Parse(gomonobold.TTF)
	if err != nil {
		panic(err)
	}
	return parsed
}()

// newRandomSource returns a ChaCha8 generator seeded from crypto/rand.
func newRandomSource() *mrand.Rand {
	var seed [32]byte
	rand.Read(seed[:])
	return mrand.New(mrand.NewChaCha8(seed))
}

// newCode returns codeLength random characters from chars.
func newCode(chars string) string {
	random := newRandomSource()
	code := make([]byte, codeLength)
	for i := range code {
		code[i] = chars[random.IntN(len(chars))]
	}
	return string(code)
}

func randomColor(random *mrand.Rand, darkest, lightest int) color.RGBA {
	channel := func() uint8 { return uint8(darkest + random.IntN(lightest-darkest+1)) }
	return color.RGBA{channel(), channel(), channel(), 255}
}
