package storage

import (
	"bytes"
	"image"
	"image/jpeg"
	_ "image/png" // registers the PNG decoder for image.Decode

	"golang.org/x/image/draw"
	_ "golang.org/x/image/webp" // registers the WebP decoder for image.Decode
)

// proofMaxDimension is the longest side a proof image is shrunk to -- enough to read a
// bank-transfer receipt, without storing a multi-MB phone photo or screenshot.
const proofMaxDimension = 2000

// proofJPEGQuality is the JPEG quality a proof image is re-encoded at.
const proofJPEGQuality = 80

// compressImage re-encodes a JPEG, PNG or WebP image as a JPEG no larger than
// proofMaxDimension on its longest side (never enlarged). Anything that isn't a decodable
// image (a PDF receipt, say) comes back untouched, with ok false -- it's stored as sent.
func compressImage(data []byte) (compressed []byte, ok bool) {
	source, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return data, false
	}

	bounds := source.Bounds()
	width, height := bounds.Dx(), bounds.Dy()
	if longest := max(width, height); longest > proofMaxDimension {
		scale := float64(proofMaxDimension) / float64(longest)
		width, height = max(1, int(float64(width)*scale)), max(1, int(float64(height)*scale))
	}

	// Drawn onto an opaque white canvas: JPEG has no transparency, and a transparent PNG
	// would otherwise come out with a black background.
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	draw.Draw(canvas, canvas.Bounds(), image.White, image.Point{}, draw.Src)
	draw.CatmullRom.Scale(canvas, canvas.Bounds(), source, bounds, draw.Over, nil)

	var buffer bytes.Buffer
	if err := jpeg.Encode(&buffer, canvas, &jpeg.Options{Quality: proofJPEGQuality}); err != nil {
		return data, false
	}
	return buffer.Bytes(), true
}
