package asciipng

import (
	"image"
	"image/png"
	"io"
)

func init() {
	image.RegisterFormat("png", "png", png.Decode, png.DecodeConfig)
}

func Run(in io.Reader) (string, error) {
	pixels, err := getPixels(in)
	if err != nil {
		return "", err
	}

	ascii := getAscii(pixels, false)
	return ascii, nil
}
