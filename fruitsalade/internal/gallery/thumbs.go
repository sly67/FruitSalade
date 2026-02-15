package gallery

import (
	"bytes"
	"image"
	"image/jpeg"
	"io"
	"strings"

	"github.com/disintegration/imaging"
)

const (
	ThumbMaxSize = 400
	ThumbQuality = 80
)

// ThumbS3Key returns the thumbnail storage key for a given original S3 key.
func ThumbS3Key(originalPath string) string {
	key := strings.TrimPrefix(originalPath, "/")
	return "_thumbs/" + key
}

// GenerateThumbnail reads an image, generates a 400x400 max thumbnail,
// applies EXIF orientation correction, and returns the JPEG bytes.
func GenerateThumbnail(r io.Reader, orientation int) ([]byte, int, int, error) {
	img, _, err := image.Decode(r)
	if err != nil {
		return nil, 0, 0, err
	}

	// Apply EXIF orientation
	img = applyOrientation(img, orientation)

	// Fit within ThumbMaxSize x ThumbMaxSize preserving aspect ratio
	thumb := imaging.Fit(img, ThumbMaxSize, ThumbMaxSize, imaging.Lanczos)

	bounds := thumb.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, thumb, &jpeg.Options{Quality: ThumbQuality}); err != nil {
		return nil, 0, 0, err
	}

	return buf.Bytes(), w, h, nil
}

// applyOrientation transforms an image according to EXIF orientation value.
func applyOrientation(img image.Image, orientation int) image.Image {
	switch orientation {
	case 2:
		return imaging.FlipH(img)
	case 3:
		return imaging.Rotate180(img)
	case 4:
		return imaging.FlipV(img)
	case 5:
		return imaging.Transpose(img)
	case 6:
		return imaging.Rotate270(img)
	case 7:
		return imaging.Transverse(img)
	case 8:
		return imaging.Rotate90(img)
	default:
		return img
	}
}

// ImageDimensions decodes an image just enough to get its dimensions.
func ImageDimensions(r io.Reader) (width, height int, err error) {
	cfg, _, err := image.DecodeConfig(r)
	if err != nil {
		return 0, 0, err
	}
	return cfg.Width, cfg.Height, nil
}
