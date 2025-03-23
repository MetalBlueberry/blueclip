package asciipng

import (
	"image"

	"golang.org/x/image/draw"
)

// ResizeOptions contains parameters for image resizing
type ResizeOptions struct {
	MaxWidth  int
	MaxHeight int
}

// ResizeImage resizes an image maintaining aspect ratio
func resizeImage(src image.Image, opts ResizeOptions) image.Image {
	bounds := src.Bounds()
	origWidth := bounds.Max.X - bounds.Min.X
	origHeight := bounds.Max.Y - bounds.Min.Y

	newWidth, newHeight := calculateDimensions(origWidth, origHeight, opts.MaxWidth, opts.MaxHeight)

	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	draw.NearestNeighbor.Scale(dst, dst.Rect, src, src.Bounds(), draw.Over, nil)

	return dst
}

// calculateDimensions calculates new dimensions maintaining aspect ratio
func calculateDimensions(origWidth, origHeight, maxWidth, maxHeight int) (newWidth, newHeight int) {
	// If no max dimensions specified, return original
	if maxWidth == 0 && maxHeight == 0 {
		return origWidth, origHeight
	}

	// If only one dimension specified, use it to scale proportionally
	if maxWidth == 0 {
		ratio := float64(maxHeight) / float64(origHeight)
		return int(float64(origWidth) * ratio), maxHeight
	}
	if maxHeight == 0 {
		ratio := float64(maxWidth) / float64(origWidth)
		return maxWidth, int(float64(origHeight) * ratio)
	}

	// Calculate ratios
	widthRatio := float64(maxWidth) / float64(origWidth)
	heightRatio := float64(maxHeight) / float64(origHeight)

	// Use the smaller ratio to ensure image fits within bounds
	ratio := widthRatio
	if heightRatio < widthRatio {
		ratio = heightRatio
	}

	return int(float64(origWidth) * ratio), int(float64(origHeight) * ratio)
}
