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

// TouchStripImageRectangleError represents an error when a provided image
// rectangle would not fit the touch strip display available on some Elgato
// Stream Deck devices.
type TouchStripImageRectangleError struct {
	Rect image.Rectangle
}

// Error returns a string representation of a touch strip image rectangle error.
func (b TouchStripImageRectangleError) Error() string {
	return fmt.Sprintf("invalid rectangle, would not fit the touch strip display: %s", b.Rect)
}

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

func (d *Device) setKeyImage(key KeyID, img image.Image) error {
	data, err := genImage(img, d.model.keyImageRect, d.model.keyImageFormat, d.model.keyImageTransform)
	if err != nil {
		return wrapErr(err)
	}
	return wrapErr(d.model.keyImageSend(d.dev, key, data))
}

func (d *Device) setKeyImageFromReader(key KeyID, r io.Reader) error {
	img, _, err := image.Decode(r)
	if err != nil {
		return wrapErr(err)
	}

	return d.setKeyImage(key, img)
}

// SetKeyImage draws a given image.Image to an Elgato Stream Deck key
// background display. The image is scaled as needed.
func (d *Device) SetKeyImage(key KeyID, img image.Image) error {
	if err := d.validateOpen(); err != nil {
		return err
	}

	if err := d.validateKey(key); err != nil {
		return err
	}

	return d.setKeyImage(key, img)
}

// SetKeyImageFromReader draws an image from an io.Reader to an Elgato Stream
// Deck key background display. The image is decoded and scaled as needed.
func (d *Device) SetKeyImageFromReader(key KeyID, r io.Reader) error {
	if err := d.validateOpen(); err != nil {
		return err
	}

	if err := d.validateKey(key); err != nil {
		return err
	}

	if r == nil {
		return wrapErr(ErrImageInvalid)
	}

	return d.setKeyImageFromReader(key, r)
}

// SetKeyImageFromReadCloser draws an image from an io.ReadCloser to an Elgato
// Stream Deck key background display. The ReadCloser is automatically closed
// after reading.
func (d *Device) SetKeyImageFromReadCloser(key KeyID, r io.ReadCloser) error {
	if err := d.validateOpen(); err != nil {
		return err
	}

	if err := d.validateKey(key); err != nil {
		return err
	}

	if r == nil {
		return wrapErr(ErrImageInvalid)
	}
	defer r.Close()

	return d.setKeyImageFromReader(key, r)
}

// SetKeyImageFromFile draws an image from a file to an Elgato Stream Deck key
// background display. The image is loaded, decoded and scaled as needed.
func (d *Device) SetKeyImageFromFile(key KeyID, name string) error {
	if err := d.validateOpen(); err != nil {
		return err
	}

	if err := d.validateKey(key); err != nil {
		return err
	}

	fp, err := os.Open(name)
	if err != nil {
		return wrapErr(err)
	}
	defer fp.Close()

	return d.setKeyImageFromReader(key, fp)
}

// SetKeyImageFromFS draws an image from a filesystem to an Elgato Stream Deck
// key background display. The image is loaded from a filesystem, decoded and
// scaled as needed.
func (d *Device) SetKeyImageFromFS(key KeyID, ffs fs.FS, name string) error {
	if err := d.validateOpen(); err != nil {
		return err
	}

	if err := d.validateKey(key); err != nil {
		return err
	}

	fp, err := ffs.Open(name)
	if err != nil {
		return wrapErr(err)
	}
	defer fp.Close()

	return d.setKeyImageFromReader(key, fp)
}

// SetKeyColor sets a color to an Elgato Stream Deck key background display.
func (d *Device) SetKeyColor(key KeyID, c color.Color) error {
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

func (d *Device) setInfoBarImage(img image.Image) error {
	data, err := genImage(img, d.model.infoBarImageRect, d.model.infoBarImageFormat, d.model.infoBarImageTransform)
	if err != nil {
		return wrapErr(err)
	}

	return wrapErr(d.model.infoBarImageSend(d.dev, data))
}

func (d *Device) setInfoBarImageFromReader(r io.Reader) error {
	img, _, err := image.Decode(r)
	if err != nil {
		return wrapErr(err)
	}

	return d.setInfoBarImage(img)
}

// SetInfoBarImage draws a given image.Image to the info bar display available
// on some Elgato Stream Deck models. The image is scaled as needed.
func (d *Device) SetInfoBarImage(img image.Image) error {
	if err := d.validateOpen(); err != nil {
		return err
	}

	if d.model.infoBarImageSend == nil {
		return wrapErr(ErrDeviceInfoBarNotSupported)
	}

	return d.setInfoBarImage(img)
}

// SetInfoBarImageFromReader draws an image from an io.Reader to the info bar
// display available on some Elgato Stream Deck models. The image is decoded
// and scaled as needed.
func (d *Device) SetInfoBarImageFromReader(r io.Reader) error {
	if err := d.validateOpen(); err != nil {
		return err
	}

	if err := d.validateInfoBar(); err != nil {
		return err
	}

	if r == nil {
		return wrapErr(ErrImageInvalid)
	}

	return d.setInfoBarImageFromReader(r)
}

// SetInfoBarImageFromReadCloser draws an image from an io.ReadCloser to the
// info bar display available on some Elgato Stream Deck models. The
// ReadCloser is automatically closed after reading.
func (d *Device) SetInfoBarImageFromReadCloser(r io.ReadCloser) error {
	if err := d.validateOpen(); err != nil {
		return err
	}

	if err := d.validateInfoBar(); err != nil {
		return err
	}

	if r == nil {
		return wrapErr(ErrImageInvalid)
	}
	defer r.Close()

	return d.setInfoBarImageFromReader(r)
}

// SetInfoBarImageFromFile draws an image from a file to the info bar display
// available on some Elgato Stream Deck models. The image is loaded, decoded
// and scaled as needed.
func (d *Device) SetInfoBarImageFromFile(name string) error {
	if err := d.validateOpen(); err != nil {
		return err
	}

	if err := d.validateInfoBar(); err != nil {
		return err
	}

	fp, err := os.Open(name)
	if err != nil {
		return wrapErr(err)
	}
	defer fp.Close()

	return d.setInfoBarImageFromReader(fp)
}

// SetInfoBarImageFromFS draws an image from a filesystem to the info bar
// display available on some Elgato Stream Deck models. The image is loaded
// from a filesystem, decoded and scaled as needed.
func (d *Device) SetInfoBarImageFromFS(ffs fs.FS, name string) error {
	if err := d.validateOpen(); err != nil {
		return err
	}

	if err := d.validateInfoBar(); err != nil {
		return err
	}

	fp, err := ffs.Open(name)
	if err != nil {
		return wrapErr(err)
	}
	defer fp.Close()

	return d.setInfoBarImageFromReader(fp)
}

// SetInfoBarColor sets a color to an Elgato Stream Deck info bar display
// available on some Elgato Stream Deck models.
func (d *Device) SetInfoBarColor(c color.Color) error {
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
	if err := d.validateOpen(); err != nil {
		return err
	}

	if err := d.validateTouchPoint(tp); err != nil {
		return err
	}

	return d.model.touchPointColorSend(d.dev, tp, c)
}

// ClearTouchPoint clears the color set to a touch point strip available in
// some Elgato Stream Deck models.
func (d *Device) ClearTouchPoint(tp TouchPointID) error {
	return d.SetTouchPointColor(tp, color.Black)
}

func (d *Device) setTouchStripImage(img image.Image, rect *image.Rectangle) error {
	r := d.model.touchStripImageRect
	v := d.model.touchStripImageRect
	if rect != nil {
		r = *rect
		v = image.Rect(0, 0, r.Dx(), r.Dy())
	}

	data, err := genImage(img, v, d.model.touchStripImageFormat, d.model.touchStripImageTransform)
	if err != nil {
		return wrapErr(err)
	}

	return wrapErr(d.model.touchStripImageSend(d.dev, data, r))
}

func (d *Device) setTouchStripImageFromReader(r io.Reader, rect *image.Rectangle) error {
	img, _, err := image.Decode(r)
	if err != nil {
		return wrapErr(err)
	}

	return d.setTouchStripImage(img, rect)
}

func (d *Device) setTouchStripImageFromFile(name string, rect *image.Rectangle) error {
	fp, err := os.Open(name)
	if err != nil {
		return wrapErr(err)
	}
	defer fp.Close()

	return d.setTouchStripImageFromReader(fp, rect)
}

func (d *Device) setTouchStripImageFromFS(ffs fs.FS, name string, rect *image.Rectangle) error {
	fp, err := ffs.Open(name)
	if err != nil {
		return wrapErr(err)
	}
	defer fp.Close()

	return d.setTouchStripImageFromReader(fp, rect)
}

func (d *Device) validateTouchStripRectangle(rect image.Rectangle) error {
	if !rect.In(d.model.touchStripImageRect) {
		return wrapErr(&TouchStripImageRectangleError{Rect: rect})
	}
	return nil
}

// SetTouchStripImageWithRectangle draws an image.Image to the touch strip
// display available on some Elgato Stream Deck models. The image is scaled as
// needed to fit the provided rectangle.
func (d *Device) SetTouchStripImageWithRectangle(img image.Image, rect image.Rectangle) error {
	if err := d.validateOpen(); err != nil {
		return err
	}

	if err := d.validateTouchStripRectangle(rect); err != nil {
		return err
	}

	if err := d.validateTouchStrip(); err != nil {
		return err
	}

	return d.setTouchStripImage(img, &rect)
}

// SetTouchStripImage draws a given image.Image to the touch strip display
// available on some Elgato Stream Deck models. The image is scaled as needed
// to fit the whole display.
func (d *Device) SetTouchStripImage(img image.Image) error {
	if err := d.validateOpen(); err != nil {
		return err
	}

	if err := d.validateTouchStrip(); err != nil {
		return err
	}

	return d.setTouchStripImage(img, nil)
}

// SetTouchStripImageFromReaderWithRectangle draws an image from an io.Reader
// to the touch strip display available on some Elgato Stream Deck models. The
// image is decoded and scaled as needed to fit the provided rectangle.
func (d *Device) SetTouchStripImageFromReaderWithRectangle(r io.Reader, rect image.Rectangle) error {
	if err := d.validateOpen(); err != nil {
		return err
	}

	if err := d.validateTouchStrip(); err != nil {
		return err
	}

	if err := d.validateTouchStripRectangle(rect); err != nil {
		return err
	}

	if r == nil {
		return wrapErr(ErrImageInvalid)
	}

	return d.setTouchStripImageFromReader(r, &rect)
}

// SetTouchStripImageFromReader draws an image from an io.Reader to the touch
// strip display available on some Elgato Stream Deck models. The image is
// decoded and scaled as needed to fit the whole display.
func (d *Device) SetTouchStripImageFromReader(r io.Reader) error {
	if err := d.validateOpen(); err != nil {
		return err
	}

	if err := d.validateTouchStrip(); err != nil {
		return err
	}

	if r == nil {
		return wrapErr(ErrImageInvalid)
	}

	return d.setTouchStripImageFromReader(r, nil)
}

// SetTouchStripImageFromReadCloserWithRectangle draws an image from an
// io.ReadCloser to the touch strip display available on some Elgato Stream
// Deck models. The ReadCloser is automatically closed after reading. The image
// is decoded and scaled as needed to fit the provided rectangle.
func (d *Device) SetTouchStripImageFromReadCloserWithRectangle(r io.ReadCloser, rect image.Rectangle) error {
	if err := d.validateOpen(); err != nil {
		return err
	}

	if err := d.validateTouchStrip(); err != nil {
		return err
	}

	if err := d.validateTouchStripRectangle(rect); err != nil {
		return err
	}

	if r == nil {
		return wrapErr(ErrImageInvalid)
	}
	defer r.Close()

	return d.setTouchStripImageFromReader(r, &rect)
}

// SetTouchStripImageFromReadCloser draws an image from an io.ReadCloser to
// the touch strip display available on some Elgato Stream Deck models. The
// ReadCloser is automatically closed after reading. The image is decoded and
// scaled as needed to fit the whole display.
func (d *Device) SetTouchStripImageFromReadCloser(r io.ReadCloser) error {
	if err := d.validateOpen(); err != nil {
		return err
	}

	if err := d.validateTouchStrip(); err != nil {
		return err
	}

	if r == nil {
		return wrapErr(ErrImageInvalid)
	}
	defer r.Close()

	return d.setTouchStripImageFromReader(r, nil)
}

// SetTouchStripImageFromFileWithRectangle draws an image from a file to the
// touch strip display available on some Elgato Stream Deck models. The image
// is loaded, decoded and scaled as needed to fit the provided rectangle.
func (d *Device) SetTouchStripImageFromFileWithRectangle(name string, rect image.Rectangle) error {
	if err := d.validateOpen(); err != nil {
		return err
	}

	if err := d.validateTouchStrip(); err != nil {
		return err
	}

	if err := d.validateTouchStripRectangle(rect); err != nil {
		return err
	}

	return d.setTouchStripImageFromFile(name, &rect)
}

// SetTouchStripImageFromFile draws an image from a file to the touch strip
// display available on some Elgato Stream Deck models. The image is loaded,
// decoded and scaled as needed to fit the whole display.
func (d *Device) SetTouchStripImageFromFile(name string) error {
	if err := d.validateOpen(); err != nil {
		return err
	}

	if err := d.validateTouchStrip(); err != nil {
		return err
	}

	return d.setTouchStripImageFromFile(name, nil)
}

// SetTouchStripImageFromFSWithRectangle draws an image from a filesystem to
// the touch strip display available on some Elgato Stream Deck models. The
// image is loaded from a filesystem, decoded and scaled as needed to fit the
// provided rectangle.
func (d *Device) SetTouchStripImageFromFSWithRectangle(ffs fs.FS, name string, rect image.Rectangle) error {
	if err := d.validateOpen(); err != nil {
		return err
	}

	if err := d.validateTouchStrip(); err != nil {
		return err
	}

	if err := d.validateTouchStripRectangle(rect); err != nil {
		return err
	}

	return d.setTouchStripImageFromFS(ffs, name, &rect)
}

// SetTouchStripImageFromFS draws an image from a filesystem to the touch strip
// display available on some Elgato Stream Deck models. The image is loaded
// from a filesystem, decoded and scaled as needed to fit the whole display.
func (d *Device) SetTouchStripImageFromFS(ffs fs.FS, name string) error {
	if err := d.validateOpen(); err != nil {
		return err
	}

	if err := d.validateTouchStrip(); err != nil {
		return err
	}

	return d.setTouchStripImageFromFS(ffs, name, nil)
}

// SetTouchStripColorWithRectangle sets a color to the provided rectangle of
// the touch strip display available on some Elgato Stream Deck models.
func (d *Device) SetTouchStripColorWithRectangle(c color.Color, rect image.Rectangle) error {
	return d.SetTouchStripImageWithRectangle(&imageColor{
		c: c,
		b: image.Rect(0, 0, rect.Dx(), rect.Dy()),
	}, rect)
}

// SetTouchStripColor sets a color to the whole touch strip display available
// on some Elgato Stream Deck models.
func (d *Device) SetTouchStripColor(c color.Color) error {
	return d.SetTouchStripImage(&imageColor{
		c: c,
		b: d.model.touchStripImageRect,
	})
}

// ClearTouchStripWithRectangle clears the provided rectangle of the touch
// strip display available on some Elgato Stream Deck models.
func (d *Device) ClearTouchStripWithRectangle(rect image.Rectangle) error {
	return d.SetTouchStripColorWithRectangle(color.Black, rect)
}

// ClearTouchStrip clears the touch strip display available on some Elgato
// Stream Deck models.
func (d *Device) ClearTouchStrip() error {
	return d.SetTouchStripColor(color.Black)
}

// GetTouchStripImageRectangle returns an image.Rectangle representing the
// geometry of the touch strip display available on some Elgato Stream Deck
// models.
func (d *Device) GetTouchStripImageRectangle() (image.Rectangle, error) {
	if d.model.touchStripImageSend == nil {
		return image.Rectangle{}, wrapErr(ErrDeviceTouchStripNotSupported)
	}
	return d.model.touchStripImageRect, nil
}

func (d *Device) closeDisplays() error {
	if err := d.ForEachKey(d.ClearKey); err != nil {
		return err
	}

	if err := d.ForEachTouchPoint(d.ClearTouchPoint); err != nil {
		return err
	}

	if d.GetInfoBarSupported() {
		if err := d.ClearInfoBar(); err != nil {
			return err
		}
	}

	if d.GetTouchStripSupported() {
		if err := d.ClearTouchStrip(); err != nil {
			return err
		}
	}
	return nil
}
