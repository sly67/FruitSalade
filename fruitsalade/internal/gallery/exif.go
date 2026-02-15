package gallery

import (
	"fmt"
	"io"
	"math"
	"time"

	"github.com/rwcarlsen/goexif/exif"
	"github.com/rwcarlsen/goexif/tiff"
)

// ExifData holds extracted EXIF metadata.
type ExifData struct {
	Width        int
	Height       int
	CameraMake   string
	CameraModel  string
	LensModel    string
	FocalLength  float32
	Aperture     float32
	ShutterSpeed string
	ISO          int
	Flash        bool
	DateTaken    *time.Time
	Latitude     *float64
	Longitude    *float64
	Altitude     *float32
	Orientation  int
}

// ExtractExif reads EXIF data from an image reader.
// Returns nil ExifData (not an error) if no EXIF is present.
func ExtractExif(r io.Reader) (*ExifData, error) {
	x, err := exif.Decode(r)
	if err != nil {
		// No EXIF data is not an error
		return &ExifData{Orientation: 1}, nil
	}

	d := &ExifData{Orientation: 1}

	// Camera make/model
	d.CameraMake = getTagString(x, exif.Make)
	d.CameraModel = getTagString(x, exif.Model)
	d.LensModel = getTagString(x, exif.LensModel)

	// Exposure
	if fl, err := x.Get(exif.FocalLength); err == nil {
		if nums, denom, err := fl.Rat2(0); err == nil && denom != 0 {
			d.FocalLength = float32(nums) / float32(denom)
		}
	}

	if ap, err := x.Get(exif.FNumber); err == nil {
		if nums, denom, err := ap.Rat2(0); err == nil && denom != 0 {
			d.Aperture = float32(nums) / float32(denom)
		}
	}

	if ss, err := x.Get(exif.ExposureTime); err == nil {
		if nums, denom, err := ss.Rat2(0); err == nil {
			if denom == 1 {
				d.ShutterSpeed = fmt.Sprintf("%ds", nums)
			} else {
				d.ShutterSpeed = fmt.Sprintf("%d/%d", nums, denom)
			}
		}
	}

	if iso, err := x.Get(exif.ISOSpeedRatings); err == nil {
		if v, err := iso.Int(0); err == nil {
			d.ISO = v
		}
	}

	if flash, err := x.Get(exif.Flash); err == nil {
		if v, err := flash.Int(0); err == nil {
			d.Flash = (v & 1) == 1 // bit 0 indicates flash fired
		}
	}

	// Date taken
	if dt, err := x.DateTime(); err == nil {
		d.DateTaken = &dt
	}

	// GPS
	if lat, lon, err := x.LatLong(); err == nil {
		if !math.IsNaN(lat) && !math.IsNaN(lon) {
			d.Latitude = &lat
			d.Longitude = &lon
		}
	}

	if alt, err := x.Get(exif.GPSAltitude); err == nil {
		if nums, denom, err := alt.Rat2(0); err == nil && denom != 0 {
			v := float32(nums) / float32(denom)
			d.Altitude = &v
		}
	}

	// Orientation
	if orient, err := x.Get(exif.Orientation); err == nil {
		if v, err := orient.Int(0); err == nil && v >= 1 && v <= 8 {
			d.Orientation = v
		}
	}

	// Dimensions from EXIF (PixelXDimension / PixelYDimension)
	if pw, err := x.Get(exif.PixelXDimension); err == nil {
		if v, err := pw.Int(0); err == nil {
			d.Width = v
		}
	}
	if ph, err := x.Get(exif.PixelYDimension); err == nil {
		if v, err := ph.Int(0); err == nil {
			d.Height = v
		}
	}

	return d, nil
}

// getTagString extracts a string value from an EXIF tag.
func getTagString(x *exif.Exif, f exif.FieldName) string {
	tag, err := x.Get(f)
	if err != nil {
		return ""
	}
	if tag.Format() == tiff.StringVal {
		s, _ := tag.StringVal()
		return s
	}
	return tag.String()
}
