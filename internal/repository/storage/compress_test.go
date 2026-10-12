package storage

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"testing"

	"github.com/stretchr/testify/suite"
)

func TestCompressSuite(t *testing.T) {
	suite.Run(t, new(compressSuite))
}

type compressSuite struct {
	suite.Suite
}

func (s *compressSuite) encodePNG(width, height int) []byte {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 120, A: 255})
		}
	}
	var buffer bytes.Buffer
	s.Require().NoError(png.Encode(&buffer, img))
	return buffer.Bytes()
}

func (s *compressSuite) TestLargeImageIsShrunkToJPEG() {
	compressed, ok := compressImage(s.encodePNG(4000, 1000))

	s.Require().True(ok)
	decoded, err := jpeg.Decode(bytes.NewReader(compressed))
	s.Require().NoError(err)
	s.Equal(proofMaxDimension, decoded.Bounds().Dx())
	s.Equal(500, decoded.Bounds().Dy())
}

func (s *compressSuite) TestSmallImageKeepsItsSize() {
	compressed, ok := compressImage(s.encodePNG(300, 200))

	s.Require().True(ok)
	decoded, err := jpeg.Decode(bytes.NewReader(compressed))
	s.Require().NoError(err)
	s.Equal(300, decoded.Bounds().Dx())
	s.Equal(200, decoded.Bounds().Dy())
}

func (s *compressSuite) TestNonImageIsLeftAsIs() {
	pdf := []byte("%PDF-1.4 a receipt")

	compressed, ok := compressImage(pdf)

	s.False(ok)
	s.Equal(pdf, compressed)
}
