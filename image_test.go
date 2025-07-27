// Copyright 2025 Rafael G. Martins. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package streamdeck

import (
	"bytes"
	"errors"
	"image"
	"image/color"
	"image/jpeg"
	"testing"

	"golang.org/x/image/bmp"
	"golang.org/x/image/colornames"
)

func createTestImage(rect image.Rectangle) *image.RGBA {
	img := image.NewRGBA(rect)
	for y := 0; y < rect.Dy(); y++ {
		for x := 0; x < rect.Dx(); x++ {
			if x < rect.Dx()/2 && y < rect.Dy()/2 {
				img.Set(x, y, colornames.Red) // top left
			} else if x >= rect.Dx()/2 && y < rect.Dy()/2 {
				img.Set(x, y, colornames.Lime) // top right
			} else if x < rect.Dx()/2 && y >= rect.Dy()/2 {
				img.Set(x, y, colornames.Blue) // bottom left
			} else {
				img.Set(x, y, colornames.White) // bottom right
			}
		}
	}
	return img
}

func TestGenImage_BMPFormat(t *testing.T) {
	img := createTestImage(image.Rect(0, 0, 4, 4))
	rect := image.Rect(0, 0, 4, 4)

	data, err := genImage(img, rect, imageFormatBMP, 0)
	if err != nil {
		t.Fatalf("genImage failed: %v", err)
	}

	decoded, err := bmp.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to decode generated BMP: %v", err)
	}

	bounds := decoded.Bounds()
	if decoded.At(0, 0) != (color.RGBA{255, 0, 0, 255}) {
		t.Error("top-left corner doesn't match")
	}
	if decoded.At(bounds.Max.X-1, bounds.Max.Y-1) != colornames.White {
		t.Error("bottom-right corner doesn't match")
	}
}

func TestGenImage_JPEGFormat(t *testing.T) {
	img := createTestImage(image.Rect(0, 0, 4, 4))
	rect := image.Rect(0, 0, 4, 4)

	data, err := genImage(img, rect, imageFormatJPEG, 0)
	if err != nil {
		t.Fatalf("genImage failed: %v", err)
	}

	if _, err := jpeg.Decode(bytes.NewReader(data)); err != nil {
		t.Fatalf("failed to decode generated JPEG: %v", err)
	}
}

func TestGenImage_FlipHorizontal(t *testing.T) {
	img := createTestImage(image.Rect(0, 0, 4, 4))
	rect := image.Rect(0, 0, 4, 4)

	data, err := genImage(img, rect, imageFormatBMP, imageTransformFlipHorizontal)
	if err != nil {
		t.Fatalf("genImage failed: %v", err)
	}

	decoded, err := bmp.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to decode BMP: %v", err)
	}

	bounds := decoded.Bounds()
	if decoded.At(bounds.Max.X-1, 0) != colornames.Red {
		t.Error("horizontal flip failed: red should be at top-right")
	}
	if decoded.At(0, 0) != colornames.Lime {
		t.Error("horizontal flip failed: green should be at top-left")
	}
}

func TestGenImage_FlipVertical(t *testing.T) {
	img := createTestImage(image.Rect(0, 0, 4, 4))
	rect := image.Rect(0, 0, 4, 4)

	data, err := genImage(img, rect, imageFormatBMP, imageTransformFlipVertical)
	if err != nil {
		t.Fatalf("genImage failed: %v", err)
	}

	decoded, err := bmp.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to decode BMP: %v", err)
	}

	bounds := decoded.Bounds()
	if decoded.At(0, bounds.Max.Y-1) != colornames.Red {
		t.Error("vertical flip failed: red should be at bottom-left")
	}
	if decoded.At(0, 0) != colornames.Blue {
		t.Error("vertical flip failed: blue should be at top-left")
	}
}

func TestGenImage_Rotate90(t *testing.T) {
	img := createTestImage(image.Rect(0, 0, 4, 4))
	rect := image.Rect(0, 0, 4, 4)

	data, err := genImage(img, rect, imageFormatBMP, imageTransformRotate90)
	if err != nil {
		t.Fatalf("genImage failed: %v", err)
	}

	decoded, err := bmp.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to decode BMP: %v", err)
	}

	bounds := decoded.Bounds()
	if decoded.At(0, bounds.Max.Y-1) != colornames.Red {
		t.Error("90deg rotation failed: red should be at bottom-left")
	}
	if decoded.At(0, 0) != colornames.Lime {
		t.Error("90° rotation failed: green should be at top-left")
	}
}

func TestGenImage_NonSquareRotation(t *testing.T) {
	img := createTestImage(image.Rect(0, 0, 4, 4))
	rect := image.Rect(0, 0, 4, 6)

	if _, err := genImage(img, rect, imageFormatJPEG, imageTransformRotate90); !errors.Is(err, ErrImageInvalid) {
		t.Error("expected error for rotating non-square canvas")
	}
}

func TestGenImage_Scaling(t *testing.T) {
	// Test upscaling from 2x2 to 4x4
	img := createTestImage(image.Rect(0, 0, 2, 2))
	rect := image.Rect(0, 0, 4, 4)

	data, err := genImage(img, rect, imageFormatBMP, 0)
	if err != nil {
		t.Fatalf("upscaling failed: %v", err)
	}

	decoded, err := bmp.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("failed to decode upscaled BMP: %v", err)
	}

	bounds := decoded.Bounds()
	if bounds.Dx() != 4 || bounds.Dy() != 4 {
		t.Errorf("expected 4x4 output, got %dx%d", bounds.Dx(), bounds.Dy())
	}

	if decoded.At(0, 0) != colornames.Red {
		t.Error("upscaling lost top-left red color")
	}
	if decoded.At(3, 3) != colornames.White {
		t.Error("upscaling lost bottom-right white color")
	}
}

func TestGetScaledRect_AspectRatio(t *testing.T) {
	src := image.Rect(0, 0, 10, 5)
	dst := image.Rect(0, 0, 20, 20)

	result := getScaledRect(src, dst)
	expected := image.Rect(0, 5, 20, 15)
	if result != expected {
		t.Errorf("expected %v, got %v", expected, result)
	}
}
