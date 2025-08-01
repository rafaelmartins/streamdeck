// Copyright 2025 Rafael G. Martins. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package streamdeck

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"io/fs"
	"os"

	"golang.org/x/image/bmp"
	"golang.org/x/image/draw"
	"rafaelmartins.com/p/usbhid"
)

// imageColor represents a uniform color image with specified bounds.
type imageColor struct {
	c color.Color
	b image.Rectangle
}

func (ic *imageColor) ColorModel() color.Model {
	return ic
}

func (ic *imageColor) Bounds() image.Rectangle {
	return ic.b
}

func (ic *imageColor) At(x, y int) color.Color {
	return ic.c
}

func (ic *imageColor) Convert(color.Color) color.Color {
	return ic.c
}

type imageFormat byte

const (
	imageFormatBMP imageFormat = iota
	imageFormatJPEG
)

type imageTransform byte

const (
	imageTransformFlipVertical imageTransform = (1 << iota)
	imageTransformFlipHorizontal
	imageTransformRotate90
)

func getScaledRect(src image.Rectangle, dst image.Rectangle) image.Rectangle {
	srcRatio := float64(src.Dx()) / float64(src.Dy())
	dstRatio := float64(dst.Dx()) / float64(dst.Dy())

	if srcRatio > dstRatio {
		newHeight := int(float64(dst.Dx()) / srcRatio)
		y0 := dst.Min.Y + (dst.Dy()-newHeight)/2
		return image.Rect(dst.Min.X, y0, dst.Max.X, y0+newHeight)
	}

	newWidth := int(float64(dst.Dy()) * srcRatio)
	x0 := dst.Min.X + (dst.Dx()-newWidth)/2
	return image.Rect(x0, dst.Min.Y, x0+newWidth, dst.Max.Y)
}

func genImage(img image.Image, rect image.Rectangle, ifmt imageFormat, transform imageTransform) ([]byte, error) {
	if img == nil {
		return nil, wrapErr(ErrImageInvalid)
	}

	scaled := image.NewRGBA(rect)
	imgBounds := img.Bounds()
	if imgBounds.Dx() == rect.Dx() && imgBounds.Dy() == rect.Dy() {
		draw.Copy(scaled, image.Point{}, img, imgBounds, draw.Src, nil)
	} else {
		draw.BiLinear.Scale(scaled, getScaledRect(imgBounds, rect), img, imgBounds, draw.Src, nil)
	}

	final := image.NewRGBA(rect)
	for x := scaled.Bounds().Min.X; x < scaled.Bounds().Max.X; x++ {
		for y := scaled.Bounds().Min.Y; y < scaled.Bounds().Max.Y; y++ {
			xd := x
			yd := y

			if transform&imageTransformFlipHorizontal == imageTransformFlipHorizontal {
				xd = scaled.Bounds().Dx() - 1 - xd
			}

			if transform&imageTransformFlipVertical == imageTransformFlipVertical {
				yd = scaled.Bounds().Dy() - 1 - yd
			}

			if transform&imageTransformRotate90 == imageTransformRotate90 {
				if rect.Dx() != rect.Dy() {
					return nil, fmt.Errorf("%w: cannot rotate non-square canvas", ErrImageInvalid)
				}

				xxd := xd
				xd = yd
				yd = scaled.Bounds().Dx() - 1 - xxd
			}

			c := scaled.At(x, y)
			if ifmt == imageFormatBMP {
				r, g, b, _ := c.RGBA()
				c = color.RGBA{
					R: byte(r),
					G: byte(g),
					B: byte(b),
					A: 0xff,
				}
			}
			final.Set(xd, yd, c)
		}
	}

	buf := bytes.Buffer{}
	switch ifmt {
	case imageFormatBMP:
		if err := bmp.Encode(&buf, final); err != nil {
			return nil, err
		}

	case imageFormatJPEG:
		if err := jpeg.Encode(&buf, final, &jpeg.Options{Quality: 100}); err != nil {
			return nil, err
		}

	default:
		return nil, errors.New("invalid key image format")
	}
	return buf.Bytes(), nil
}

func imageSend(dev *usbhid.Device, id byte, hdr []byte, imgData []byte, updateCb func(hdr []byte, page byte, last byte, size uint16)) error {
	if !dev.IsOpen() {
		return wrapErr(ErrDeviceIsClosed)
	}
	if updateCb == nil {
		return errors.New("image update callback not set")
	}

	var (
		start uint16
		page  byte
		last  byte
	)

	for last == 0 {
		end := start + dev.GetOutputReportLength() - uint16(len(hdr))
		if l := uint16(len(imgData)); end >= l {
			end = l
			last = 1
		}

		to_send := imgData[start:end]
		updateCb(hdr, page, last, uint16(len(to_send)))

		payload := append(hdr, to_send...)
		payload = append(payload, make([]byte, dev.GetOutputReportLength()-uint16(len(payload)))...)
		if err := dev.SetOutputReport(id, payload); err != nil {
			return err
		}

		start += dev.GetOutputReportLength() - uint16(len(hdr))
		page++
	}
	return nil
}

// SetKeyImage draws a given image.Image to an Elgato Stream Deck key
// background display. The image is scaled as needed.
func (d *Device) SetKeyImage(key KeyID, img image.Image) error {
	if !d.IsOpen() {
		return wrapErr(ErrDeviceIsClosed)
	}

	data, err := genImage(img, d.model.keyImageRect, d.model.keyImageFormat, d.model.keyImageTransform)
	if err != nil {
		return wrapErr(err)
	}
	return wrapErr(d.model.keyImageSend(d.dev, key, data))
}

// SetKeyImageFromReader draws an image from an io.Reader to an Elgato Stream
// Deck key background display. The image is decoded and scaled as needed.
func (d *Device) SetKeyImageFromReader(key KeyID, r io.Reader) error {
	if r == nil {
		return wrapErr(ErrImageInvalid)
	}
	img, _, err := image.Decode(r)
	if err != nil {
		return wrapErr(err)
	}
	return d.SetKeyImage(key, img)
}

// SetKeyImageFromReadCloser draws an image from an io.ReadCloser to an Elgato
// Stream Deck key background display. The ReadCloser is automatically closed
// after reading.
func (d *Device) SetKeyImageFromReadCloser(key KeyID, r io.ReadCloser) error {
	if r == nil {
		return wrapErr(ErrImageInvalid)
	}
	defer r.Close()
	return d.SetKeyImageFromReader(key, r)
}

// SetKeyImageFromFile draws an image from a file to an Elgato Stream Deck key
// background display. The image is loaded, decoded and scaled as needed.
func (d *Device) SetKeyImageFromFile(key KeyID, name string) error {
	fp, err := os.Open(name)
	if err != nil {
		return wrapErr(err)
	}
	return d.SetKeyImageFromReadCloser(key, fp)
}

// SetKeyImageFromFS draws an image from a filesystem to an Elgato Stream Deck
// key background display. The image is loaded from a filesystem, decoded and
// scaled as needed.
func (d *Device) SetKeyImageFromFS(key KeyID, ffs fs.FS, name string) error {
	fp, err := ffs.Open(name)
	if err != nil {
		return wrapErr(err)
	}
	return d.SetKeyImageFromReadCloser(key, fp)
}

// SetKeyColor sets a color to an Elgato Stream Deck key background display.
func (d *Device) SetKeyColor(key KeyID, c color.Color) error {
	if !d.IsOpen() {
		return wrapErr(ErrDeviceIsClosed)
	}

	return d.SetKeyImage(key, &imageColor{
		c: c,
		b: d.model.keyImageRect,
	})
}

// ClearKey clears the Elgato Stream Deck key background display.
func (d *Device) ClearKey(key KeyID) error {
	return d.SetKeyColor(key, color.Black)
}

// GetKeyImageRectangle returns an image.Rectangle representing the geometry
// of the Elgato Stream Deck key background displays.
func (d *Device) GetKeyImageRectangle() (image.Rectangle, error) {
	return d.model.keyImageRect, nil // at some point there could be a stream deck without key display?
}

// SetInfoBarImage draws a given image.Image to the info bar display available
// on some Elgato Stream Deck models. The image is scaled as needed.
func (d *Device) SetInfoBarImage(img image.Image) error {
	if !d.IsOpen() {
		return wrapErr(ErrDeviceIsClosed)
	}
	if d.model.infoBarImageSend == nil {
		return wrapErr(ErrDeviceInfoBarNotSupported)
	}

	data, err := genImage(img, d.model.infoBarImageRect, d.model.infoBarImageFormat, d.model.infoBarImageTransform)
	if err != nil {
		return wrapErr(err)
	}
	return wrapErr(d.model.infoBarImageSend(d.dev, data))
}

// SetInfoBarImageFromReader draws an image from an io.Reader to the info bar
// display available on some Elgato Stream Deck models. The image is decoded
// and scaled as needed.
func (d *Device) SetInfoBarImageFromReader(r io.Reader) error {
	if r == nil {
		return wrapErr(ErrImageInvalid)
	}
	img, _, err := image.Decode(r)
	if err != nil {
		return wrapErr(err)
	}
	return d.SetInfoBarImage(img)
}

// SetInfoBarImageFromReadCloser draws an image from an io.ReadCloser to the
// info bar display available on some Elgato Stream Deck models. The
// ReadCloser is automatically closed after reading.
func (d *Device) SetInfoBarImageFromReadCloser(r io.ReadCloser) error {
	if r == nil {
		return wrapErr(ErrImageInvalid)
	}
	defer r.Close()
	return d.SetInfoBarImageFromReader(r)
}

// SetInfoBarImageFromFile draws an image from a file to the info bar display
// available on some Elgato Stream Deck models. The image is loaded, decoded
// and scaled as needed.
func (d *Device) SetInfoBarImageFromFile(name string) error {
	fp, err := os.Open(name)
	if err != nil {
		return wrapErr(err)
	}
	return d.SetInfoBarImageFromReadCloser(fp)
}

// SetInfoBarImageFromFS draws an image from a filesystem to the info bar
// display available on some Elgato Stream Deck models. The image is loaded
// from a filesystem, decoded and scaled as needed.
func (d *Device) SetInfoBarImageFromFS(ffs fs.FS, name string) error {
	fp, err := ffs.Open(name)
	if err != nil {
		return wrapErr(err)
	}
	return d.SetInfoBarImageFromReadCloser(fp)
}

// SetInfoBarColor sets a color to an Elgato Stream Deck info bar display
// available on some Elgato Stream Deck models.
func (d *Device) SetInfoBarColor(c color.Color) error {
	if !d.IsOpen() {
		return wrapErr(ErrDeviceIsClosed)
	}

	return d.SetInfoBarImage(&imageColor{
		c: c,
		b: d.model.infoBarImageRect,
	})
}

// ClearInfoBar clears the info bar display available on some Elgato Stream
// Deck models.
func (d *Device) ClearInfoBar() error {
	return d.SetInfoBarColor(color.Black)
}

// GetInfoBarImageRectangle returns an image.Rectangle representing the
// geometry of the info bar display available on some Elgato Stream Deck
// models.
func (d *Device) GetInfoBarImageRectangle() (image.Rectangle, error) {
	if d.model.infoBarImageSend == nil {
		return image.Rectangle{}, wrapErr(ErrDeviceInfoBarNotSupported)
	}
	return d.model.infoBarImageRect, nil
}

// SetTouchPointColor sets a color to the touch point strip available in some
// Elgato Stream Deck models.
func (d *Device) SetTouchPointColor(tp TouchPointID, c color.Color) error {
	if !d.IsOpen() {
		return wrapErr(ErrDeviceIsClosed)
	}
	if d.model.touchPointColorSend == nil || d.model.touchPointCount == 0 {
		return wrapErr(ErrDeviceTouchPointNotSupported)
	}
	return d.model.touchPointColorSend(d.dev, tp, c)
}

// ClearTouchPoint clears the color set to a touch point strip available in
// some Elgato Stream Deck models.
func (d *Device) ClearTouchPoint(tp TouchPointID) error {
	return d.SetTouchPointColor(tp, color.Black)
}

func (d *Device) closeDisplays() error {
	if err := d.ForEachKey(d.ClearKey); err != nil {
		return err
	}

	if err := d.ForEachTouchPoint(d.ClearTouchPoint); err != nil {
		return err
	}

	if d.GetInfoBarSupported() {
		return d.ClearInfoBar()
	}
	return nil
}
