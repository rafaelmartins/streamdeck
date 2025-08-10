// Copyright 2025 Rafael G. Martins. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package streamdeck

import (
	"bytes"
	"fmt"
	"image"
	"image/color"

	"rafaelmartins.com/p/usbhid"
)

const elgatoVendorID uint16 = 0x0fd9

type model struct {
	id                       string
	keyStart                 byte
	keyCount                 byte
	keyImageRect             image.Rectangle
	keyImageFormat           imageFormat
	keyImageTransform        imageTransform
	keyImageSend             func(dev *usbhid.Device, key KeyID, imgData []byte) error
	infoBarImageRect         image.Rectangle
	infoBarImageFormat       imageFormat
	infoBarImageTransform    imageTransform
	infoBarImageSend         func(dev *usbhid.Device, imgData []byte) error
	touchPointStart          byte
	touchPointCount          byte
	touchPointColorSend      func(dev *usbhid.Device, tp TouchPointID, c color.Color) error
	dialStart                byte
	dialCount                byte
	touchStripImageRect      image.Rectangle
	touchStripImageFormat    imageFormat
	touchStripImageTransform imageTransform
	touchStripImageSend      func(dev *usbhid.Device, imgData []byte, rect image.Rectangle) error
	reset                    func(dev *usbhid.Device) error
	brightness               func(dev *usbhid.Device, perc byte) error
	firmwareVersion          func(dev *usbhid.Device) (string, error)
}

var models = map[uint16]*model{
	0x0063: {
		id:                "mini",
		keyStart:          0,
		keyCount:          6,
		keyImageRect:      image.Rect(0, 0, 80, 80),
		keyImageFormat:    imageFormatBMP,
		keyImageTransform: imageTransformRotate90 | imageTransformFlipHorizontal,
		keyImageSend: func(dev *usbhid.Device, key KeyID, imgData []byte) error {
			hdr := make([]byte, 15)
			hdr[0] = 1
			hdr[4] = 1 + byte(key-KEY_1)
			return imageSend(dev, 2, hdr, imgData, func(hdr []byte, page, last byte, size uint16) {
				hdr[1] = page
				hdr[3] = last
			})
		},
		reset: func(dev *usbhid.Device) error {
			pl := make([]byte, dev.GetFeatureReportLength())
			pl[0] = 0x63
			return dev.SetFeatureReport(11, pl)
		},
		brightness: func(dev *usbhid.Device, perc byte) error {
			pl := make([]byte, dev.GetFeatureReportLength())
			pl[0] = 0x55
			pl[1] = 0xaa
			pl[2] = 0xd1
			pl[3] = 0x01
			pl[4] = perc
			return dev.SetFeatureReport(5, pl)
		},
		firmwareVersion: func(dev *usbhid.Device) (string, error) {
			buf, err := dev.GetFeatureReport(4)
			if err != nil {
				return "", err
			}
			b, _, _ := bytes.Cut(buf[4:], []byte{0})
			return string(b), nil
		},
	},
	0x0080: {
		id:                "mk2",
		keyStart:          3,
		keyCount:          15,
		keyImageRect:      image.Rect(0, 0, 72, 72),
		keyImageFormat:    imageFormatJPEG,
		keyImageTransform: imageTransformFlipHorizontal | imageTransformFlipVertical,
		keyImageSend: func(dev *usbhid.Device, key KeyID, imgData []byte) error {
			hdr := make([]byte, 7)
			hdr[0] = 7
			hdr[1] = byte(key - KEY_1)
			return imageSend(dev, 2, hdr, imgData, func(hdr []byte, page, last byte, size uint16) {
				hdr[2] = last
				hdr[3] = byte(size)
				hdr[4] = byte(size >> 8)
				hdr[5] = byte(page)
			})
		},
		reset: func(dev *usbhid.Device) error {
			pl := make([]byte, dev.GetFeatureReportLength())
			pl[0] = 0x02
			return dev.SetFeatureReport(3, pl)
		},
		brightness: func(dev *usbhid.Device, perc byte) error {
			pl := make([]byte, dev.GetFeatureReportLength())
			pl[0] = 0x08
			pl[1] = perc
			return dev.SetFeatureReport(3, pl)
		},
		firmwareVersion: func(dev *usbhid.Device) (string, error) {
			buf, err := dev.GetFeatureReport(5)
			if err != nil {
				return "", err
			}
			b, _, _ := bytes.Cut(buf[5:], []byte{0})
			return string(b), nil
		},
	},
	0x0084: {
		id:                "plus",
		keyStart:          3,
		keyCount:          8,
		keyImageRect:      image.Rect(0, 0, 120, 120),
		keyImageFormat:    imageFormatJPEG,
		keyImageTransform: 0,
		keyImageSend: func(dev *usbhid.Device, key KeyID, imgData []byte) error {
			hdr := make([]byte, 7)
			hdr[0] = 7
			hdr[1] = byte(key - KEY_1)
			return imageSend(dev, 2, hdr, imgData, func(hdr []byte, page, last byte, size uint16) {
				hdr[2] = last
				hdr[3] = byte(size)
				hdr[4] = byte(size >> 8)
				hdr[5] = byte(page)
			})
		},
		dialStart:                4,
		dialCount:                4,
		touchStripImageRect:      image.Rect(0, 0, 800, 100),
		touchStripImageFormat:    imageFormatJPEG,
		touchStripImageTransform: 0,
		touchStripImageSend: func(dev *usbhid.Device, imgData []byte, rect image.Rectangle) error {
			hdr := make([]byte, 15)
			hdr[0] = 12
			hdr[1] = byte(rect.Min.X)
			hdr[2] = byte(rect.Min.X >> 8)
			hdr[3] = byte(rect.Min.Y)
			hdr[4] = byte(rect.Min.Y >> 8)
			hdr[5] = byte(rect.Dx())
			hdr[6] = byte(rect.Dx() >> 8)
			hdr[7] = byte(rect.Dy())
			hdr[8] = byte(rect.Dy() >> 8)
			return imageSend(dev, 2, hdr, imgData, func(hdr []byte, page, last byte, size uint16) {
				hdr[9] = last
				hdr[10] = page
				hdr[11] = 0
				hdr[12] = byte(size)
				hdr[13] = byte(size >> 8)
			})
		},
		reset: func(dev *usbhid.Device) error {
			pl := make([]byte, dev.GetFeatureReportLength())
			pl[0] = 0x02
			return dev.SetFeatureReport(3, pl)
		},
		brightness: func(dev *usbhid.Device, perc byte) error {
			pl := make([]byte, dev.GetFeatureReportLength())
			pl[0] = 0x08
			pl[1] = perc
			return dev.SetFeatureReport(3, pl)
		},
		firmwareVersion: func(dev *usbhid.Device) (string, error) {
			buf, err := dev.GetFeatureReport(5)
			if err != nil {
				return "", err
			}
			b, _, _ := bytes.Cut(buf[5:], []byte{0})
			return string(b), nil
		},
	},
	0x009a: {
		id:                "neo",
		keyStart:          3,
		keyCount:          8,
		keyImageRect:      image.Rect(0, 0, 96, 96),
		keyImageFormat:    imageFormatJPEG,
		keyImageTransform: imageTransformFlipHorizontal | imageTransformFlipVertical,
		keyImageSend: func(dev *usbhid.Device, key KeyID, imgData []byte) error {
			hdr := make([]byte, 7)
			hdr[0] = 7
			hdr[1] = byte(key - KEY_1)
			return imageSend(dev, 2, hdr, imgData, func(hdr []byte, page, last byte, size uint16) {
				hdr[2] = last
				hdr[3] = byte(size)
				hdr[4] = byte(size >> 8)
				hdr[5] = byte(page)
			})
		},
		infoBarImageRect:      image.Rect(0, 0, 248, 58),
		infoBarImageFormat:    imageFormatJPEG,
		infoBarImageTransform: imageTransformFlipHorizontal | imageTransformFlipVertical,
		infoBarImageSend: func(dev *usbhid.Device, imgData []byte) error {
			hdr := make([]byte, 7)
			hdr[0] = 11
			return imageSend(dev, 2, hdr, imgData, func(hdr []byte, page, last byte, size uint16) {
				hdr[2] = last
				hdr[3] = byte(size)
				hdr[4] = byte(size >> 8)
				hdr[5] = byte(page)
			})
		},
		touchPointStart: 11,
		touchPointCount: 2,
		touchPointColorSend: func(dev *usbhid.Device, tp TouchPointID, c color.Color) error {
			r, g, b, _ := c.RGBA()
			pl := make([]byte, dev.GetFeatureReportLength())
			pl[0] = 0x06
			pl[1] = byte(tp - TOUCH_POINT_1 + 8 /* keyCount */)
			pl[2] = byte(r)
			pl[3] = byte(g)
			pl[4] = byte(b)
			return dev.SetFeatureReport(3, pl)
		},
		reset: func(dev *usbhid.Device) error {
			pl := make([]byte, dev.GetFeatureReportLength())
			pl[0] = 0x02
			return dev.SetFeatureReport(3, pl)
		},
		brightness: func(dev *usbhid.Device, perc byte) error {
			pl := make([]byte, dev.GetFeatureReportLength())
			pl[0] = 0x08
			pl[1] = perc
			return dev.SetFeatureReport(3, pl)
		},
		firmwareVersion: func(dev *usbhid.Device) (string, error) {
			buf, err := dev.GetFeatureReport(5)
			if err != nil {
				return "", err
			}
			b, _, _ := bytes.Cut(buf[5:], []byte{0})
			return string(b), nil
		},
	},
}

var modelAliases = map[uint16]uint16{
	0x006d: 0x0080,
}

func getModel(dev *usbhid.Device) (*model, error) {
	if dev.VendorId() != elgatoVendorID {
		return nil, fmt.Errorf("%w: not an Elgato device: %04x", ErrDeviceEnumerationFailed, dev.VendorId())
	}

	id := dev.ProductId()
	if ma, found := modelAliases[id]; found {
		id = ma
	}

	md, found := models[id]
	if !found {
		return nil, fmt.Errorf("%w: device not supported: %04x:%04x", ErrDeviceEnumerationFailed, dev.VendorId(), dev.ProductId())
	}
	return md, nil
}

func enumerateFunc(dev *usbhid.Device) bool {
	if dev.VendorId() != elgatoVendorID {
		return false
	}
	id := dev.ProductId()
	if ma, found := modelAliases[id]; found {
		id = ma
	}
	_, found := models[id]
	return found
}
