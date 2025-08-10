// Copyright 2025 Rafael G. Martins. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package streamdeck provides support for interacting with Elgato Stream Deck
// devices connected to a computer without using vendor-provided software.
//
// It is written in pure Go and works on Linux, macOS and Windows.
package streamdeck

import (
	"errors"
	"fmt"
	"image"
	"time"

	"rafaelmartins.com/p/usbhid"
)

// Errors returned from streamdeck package may be tested against these errors
// with errors.Is.
var (
	ErrDeviceEnumerationFailed      = usbhid.ErrDeviceEnumerationFailed
	ErrDeviceFailedToClose          = usbhid.ErrDeviceFailedToClose
	ErrDeviceFailedToOpen           = usbhid.ErrDeviceFailedToOpen
	ErrDeviceInfoBarNotSupported    = errors.New("device hardware does not includes an info bar")
	ErrDeviceIsClosed               = usbhid.ErrDeviceIsClosed
	ErrDeviceIsOpen                 = usbhid.ErrDeviceIsOpen
	ErrDeviceLocked                 = usbhid.ErrDeviceLocked
	ErrDeviceTouchPointNotSupported = errors.New("device hardware does not includes touch points")
	ErrDeviceTouchStripNotSupported = errors.New("device hardware does not includes a touch strip")
	ErrDialHandlerInvalid           = errors.New("dial handler is not valid")
	ErrDialInvalid                  = errors.New("dial is not valid")
	ErrGetFeatureReportFailed       = usbhid.ErrGetFeatureReportFailed
	ErrGetInputReportFailed         = usbhid.ErrGetInputReportFailed
	ErrImageInvalid                 = errors.New("image is not valid")
	ErrKeyHandlerInvalid            = errors.New("key handler is not valid")
	ErrKeyInvalid                   = errors.New("key is not valid")
	ErrMoreThanOneDeviceFound       = usbhid.ErrMoreThanOneDeviceFound
	ErrNoDeviceFound                = usbhid.ErrNoDeviceFound
	ErrReportBufferOverflow         = usbhid.ErrReportBufferOverflow
	ErrSetFeatureReportFailed       = usbhid.ErrSetFeatureReportFailed
	ErrSetOutputReportFailed        = usbhid.ErrSetOutputReportFailed
	ErrTouchPointHandlerInvalid     = errors.New("touch point handler is not valid")
	ErrTouchPointInvalid            = errors.New("touch point is not valid")
	ErrTouchStripHandlerInvalid     = errors.New("touch strip handler is not valid")
)

// Device represents an Elgato Stream Deck device and provides methods to
// interact with it, including setting key images, handling input events, and
// controlling device settings.
type Device struct {
	dev             *usbhid.Device
	model           *model
	inputs          []*input
	dialInputs      []*input
	touchStripInput *input
	keyStates       []byte
	dialStates      []byte
	listen          chan struct{}
	open            bool
}

func wrapErr(err error) error {
	if err != nil {
		return fmt.Errorf("streamdeck: %w", err)
	}
	return nil
}

// Enumerate lists the supported Elgato Stream Deck devices connected to the
// computer.
func Enumerate() ([]*Device, error) {
	devices, err := usbhid.Enumerate(enumerateFunc)
	if err != nil {
		return nil, wrapErr(err)
	}

	rv := []*Device{}
	for _, dev := range devices {
		model, err := getModel(dev)
		if err != nil {
			return nil, wrapErr(err)
		}
		rv = append(rv, &Device{
			dev:   dev,
			model: model,
		})
	}
	return rv, nil
}

// GetDevice returns an Elgato Stream Deck device found connected to the
// machine that matches the provided serial number. If serial number is empty
// and only one device is connected, this device is returned, otherwise an
// error is returned.
func GetDevice(serialNumber string) (*Device, error) {
	devices, err := usbhid.Enumerate(enumerateFunc)
	if err != nil {
		return nil, wrapErr(err)
	}
	if len(devices) == 0 {
		return nil, fmt.Errorf("streamdeck: %w [%q]", ErrNoDeviceFound, serialNumber)
	}

	if serialNumber == "" {
		if len(devices) == 1 {
			model, err := getModel(devices[0])
			if err != nil {
				return nil, wrapErr(err)
			}
			return &Device{
				dev:   devices[0],
				model: model,
			}, nil
		}

		sn := []string{}
		for _, usbDev := range devices {
			sn = append(sn, usbDev.SerialNumber())
		}
		return nil, fmt.Errorf("streamdeck: %w %q", ErrMoreThanOneDeviceFound, sn)
	}

	for _, dev := range devices {
		if dev.SerialNumber() == serialNumber {
			model, err := getModel(dev)
			if err != nil {
				return nil, wrapErr(err)
			}
			return &Device{
				dev:   dev,
				model: model,
			}, nil
		}
	}

	return nil, fmt.Errorf("streamdeck: %w [%q]", ErrNoDeviceFound, serialNumber)
}

// IsOpen checks if the Elgato Stream Deck device is open and available for
// usage.
func (d *Device) IsOpen() bool {
	return d.open && d.dev.IsOpen()
}

// Open opens the Elgato Stream Deck device for usage.
func (d *Device) Open() error {
	if d.IsOpen() {
		return wrapErr(ErrDeviceIsOpen)
	}

	if err := d.dev.Open(true); err != nil {
		return wrapErr(err)
	}

	d.open = true
	d.listen = make(chan struct{})
	return nil
}

func (d *Device) validateOpen() error {
	if !d.IsOpen() {
		return wrapErr(ErrDeviceIsClosed)
	}
	return nil
}

// Close closes the Elgato Stream Deck device.
func (d *Device) Close() error {
	if err := d.validateOpen(); err != nil {
		return err
	}

	if err := d.closeDisplays(); err != nil {
		return wrapErr(err)
	}

	if d.listen != nil {
		close(d.listen)
		d.listen = nil
	}

	if err := d.dev.Close(); err != nil {
		return err
	}

	d.open = false
	return nil
}

func (d *Device) validateKey(key KeyID) error {
	if key < KEY_1 || key >= KEY_1+KeyID(d.model.keyCount) {
		return fmt.Errorf("%w: %s", ErrKeyInvalid, key)
	}
	return nil
}

func (d *Device) validateTouchPoint(tp TouchPointID) error {
	if d.model.touchPointColorSend == nil || d.model.touchPointCount == 0 {
		return wrapErr(ErrDeviceTouchPointNotSupported)
	}

	if tp < TOUCH_POINT_1 || tp >= TOUCH_POINT_1+TouchPointID(d.model.touchPointCount) {
		return fmt.Errorf("%w: %s", ErrTouchPointInvalid, tp)
	}
	return nil
}

func (d *Device) validateInfoBar() error {
	if d.model.infoBarImageSend == nil {
		return wrapErr(ErrDeviceInfoBarNotSupported)
	}
	return nil
}

func (d *Device) validateDial(di DialID) error {
	if di < DIAL_1 || di >= DIAL_1+DialID(d.model.dialCount) {
		return fmt.Errorf("%w: %s", ErrDialInvalid, di)
	}
	return nil
}

func (d *Device) validateTouchStrip() error {
	if d.model.touchStripImageSend == nil {
		return wrapErr(ErrDeviceTouchStripNotSupported)
	}
	return nil
}

// AddKeyHandler registers a KeyHandler callback to be called whenever the
// given key is pressed.
func (d *Device) AddKeyHandler(key KeyID, fn KeyHandler) error {
	if err := d.validateKey(key); err != nil {
		return err
	}

	if fn == nil {
		return wrapErr(ErrKeyHandlerInvalid)
	}

	if d.inputs == nil {
		d.inputs = newInputs(d, d.model.keyCount, d.model.touchPointCount)
	}

	for _, in := range d.inputs {
		if in.key != nil && in.key.id == key {
			in.key.addHandler(fn)
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrKeyInvalid, key)
}

// AddTouchPointHandler registers a TouchPointHandler callback to be called
// whenever the given touch point is pressed.
func (d *Device) AddTouchPointHandler(tp TouchPointID, fn TouchPointHandler) error {
	if err := d.validateTouchPoint(tp); err != nil {
		return err
	}

	if fn == nil {
		return wrapErr(ErrTouchPointHandlerInvalid)
	}

	if d.inputs == nil {
		d.inputs = newInputs(d, d.model.keyCount, d.model.touchPointCount)
	}

	for _, in := range d.inputs {
		if in.tp != nil && in.tp.id == tp {
			in.tp.addHandler(fn)
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrTouchPointInvalid, tp)
}

// AddDialSwitchHandler registers a DialSwitchHandler callback to be called
// whenever the given dial is pressed.
func (d *Device) AddDialSwitchHandler(di DialID, fn DialSwitchHandler) error {
	if err := d.validateDial(di); err != nil {
		return err
	}

	if fn == nil {
		return wrapErr(ErrDialHandlerInvalid)
	}

	if d.dialInputs == nil {
		d.dialInputs = newDialInputs(d, d.model.dialCount)
	}

	for _, in := range d.dialInputs {
		if in.dial != nil && in.dial.id == di {
			in.dial.addSwitchHandler(fn)
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrDialInvalid, di)
}

// AddDialRotateHandler registers a DialSwitchHandler callback to be called
// whenever the given dial is rotated.
func (d *Device) AddDialRotateHandler(di DialID, fn DialRotateHandler) error {
	if err := d.validateDial(di); err != nil {
		return err
	}

	if fn == nil {
		return wrapErr(ErrDialHandlerInvalid)
	}

	if d.dialInputs == nil {
		d.dialInputs = newDialInputs(d, d.model.dialCount)
	}

	for _, in := range d.dialInputs {
		if in.dial != nil && in.dial.id == di {
			in.dial.addRotateHandler(fn)
			return nil
		}
	}
	return fmt.Errorf("%w: %s", ErrDialInvalid, di)
}

// AddTouchStripTouchHandler registers a TouchStripTouchHandler callback to be
// called whenever the touch strip is touched.
func (d *Device) AddTouchStripTouchHandler(fn TouchStripTouchHandler) error {
	if err := d.validateTouchStrip(); err != nil {
		return err
	}

	if fn == nil {
		return wrapErr(ErrTouchStripHandlerInvalid)
	}

	if d.touchStripInput == nil {
		d.touchStripInput = newTouchStripInput(d)
	}

	d.touchStripInput.touchStrip.addTouchHandler(fn)
	return nil
}

// AddTouchStripSwipeHandler registers a TouchStripSwipeHandler callback to be
// called whenever the touch strip is swiped.
func (d *Device) AddTouchStripSwipeHandler(fn TouchStripSwipeHandler) error {
	if err := d.validateTouchStrip(); err != nil {
		return err
	}

	if fn == nil {
		return wrapErr(ErrTouchStripHandlerInvalid)
	}

	if d.touchStripInput == nil {
		d.touchStripInput = newTouchStripInput(d)
	}

	d.touchStripInput.touchStrip.addSwipeHandler(fn)
	return nil
}

// Listen listens to input events from the Elgato Stream Deck device and calls
// handler callbacks as required.
//
// errCh is an error channel to receive errors from the input handlers. If set
// to a nil channel, errors are sent to standard logger. Errors are sent
// non-blocking.
func (d *Device) Listen(errCh chan error) error {
	if err := d.validateOpen(); err != nil {
		return err
	}

	if i := int(d.model.keyCount + d.model.touchPointCount); len(d.keyStates) != i {
		d.keyStates = make([]byte, i)
	}
	if len(d.dialStates) != int(d.model.dialCount) {
		d.dialStates = make([]byte, d.model.dialCount)
	}

	for {
		select {
		case <-d.listen:
			return nil
		default:
			if d.listen == nil {
				return nil
			}
		}

		id, buf, err := d.dev.GetInputReport()
		if err != nil {
			return wrapErr(err)
		}
		if id != 1 {
			return fmt.Errorf("streamdeck: got unexpected report id: %d", id)
		}

		if buf[0] == 2 && d.model.touchStripImageSend != nil {
			if d.touchStripInput == nil {
				continue
			}

			t := TouchStripTouchType(0)

			switch buf[3] {
			case 1:
				t = TOUCH_STRIP_TOUCH_TYPE_SHORT
				fallthrough

			case 2:
				if t == 0 {
					t = TOUCH_STRIP_TOUCH_TYPE_LONG
				}

				if len(buf) < 9 {
					continue
				}

				d.touchStripInput.touch(t, image.Point{
					X: int(buf[6])<<8 | int(buf[5]),
					Y: int(buf[8])<<8 | int(buf[7]),
				}, errCh)

			case 3:
				if len(buf) < 13 {
					continue
				}

				d.touchStripInput.swipe(image.Point{
					X: int(buf[6])<<8 | int(buf[5]),
					Y: int(buf[8])<<8 | int(buf[7]),
				}, image.Point{
					X: int(buf[10])<<8 | int(buf[9]),
					Y: int(buf[12])<<8 | int(buf[11]),
				}, errCh)
			}
			continue
		}

		if buf[0] == 3 && d.model.dialCount > 0 {
			states := buf[d.model.dialStart : d.model.dialStart+d.model.dialCount]
			switch buf[3] {
			case 0:
				t := time.Now()
				for i, st := range states {
					if st == d.dialStates[i] {
						continue
					}
					if i >= len(d.dialInputs) {
						continue
					}

					inp := d.dialInputs[i]
					if st > 0 {
						inp.press(t, errCh)
					} else {
						inp.release(t)
					}
				}
				d.dialStates = states
				continue

			case 1:
				for i, st := range states {
					if i >= len(d.dialInputs) {
						continue
					}
					if st != 0 {
						d.dialInputs[i].rotate(int8(st), errCh)
					}
				}
			}
			continue
		}

		states := buf[d.model.keyStart : d.model.keyStart+d.model.keyCount]
		if d.model.touchPointCount > 0 {
			states = append(states, buf[d.model.touchPointStart:d.model.touchPointStart+d.model.touchPointCount]...)
		}

		t := time.Now()
		for i, st := range states {
			if st == d.keyStates[i] {
				continue
			}
			if i >= len(d.inputs) {
				continue
			}

			inp := d.inputs[i]
			if st > 0 {
				inp.press(t, errCh)
			} else {
				inp.release(t)
			}
		}
		d.keyStates = states
	}
}

// GetModelName returns the Elgato Stream Deck device model name.
func (d *Device) GetModelName() string {
	return d.dev.Product()
}

// GetModelID returns a string identifier of the Elgato Stream Deck device
// model.
func (d *Device) GetModelID() string {
	return d.model.id
}

// GetSerialNumber returns the serial number of the Elgato Stream Deck device.
func (d *Device) GetSerialNumber() string {
	return d.dev.SerialNumber()
}

// GetKeyCount returns the number of keys available on the Elgato Stream Deck
// device.
func (d *Device) GetKeyCount() byte {
	return d.model.keyCount
}

// GetTouchPointCount returns the number of touch points available on the
// Elgato Stream Deck device, if supported.
func (d *Device) GetTouchPointCount() byte {
	return d.model.touchPointCount
}

// GetDialCount returns the number of dials available on the Elgato Stream Deck
// device.
func (d *Device) GetDialCount() byte {
	return d.model.dialCount
}

// GetInfoBarSupported returns a boolean reporting if the Elgato Stream Deck
// device includes an info bar display.
func (d *Device) GetInfoBarSupported() bool {
	return d.model.infoBarImageSend != nil
}

// GetTouchStripSupported returns a boolean reporting if the Elgato Stream Deck
// device includes an touch strip display.
func (d *Device) GetTouchStripSupported() bool {
	return d.model.touchStripImageSend != nil
}

// GetFirmwareVersion returns the firmware version of the Elgato Stream Deck
// device.
func (d *Device) GetFirmwareVersion() (string, error) {
	if err := d.validateOpen(); err != nil {
		return "", err
	}

	rv, err := d.model.firmwareVersion(d.dev)
	if err != nil {
		return "", wrapErr(err)
	}
	return rv, nil
}

// Reset resets the Elgato Stream Deck device.
//
// Please note that this will close the connection, because this is similar to
// power cycling the device. This function won't try to reconnect.
func (d *Device) Reset() error {
	if err := d.validateOpen(); err != nil {
		return err
	}

	if err := d.model.reset(d.dev); err != nil {
		return wrapErr(err)
	}

	return d.dev.Close()
}

// SetBrightness sets the Elgato Stream Deck device brightness, in percent.
func (d *Device) SetBrightness(perc byte) error {
	if err := d.validateOpen(); err != nil {
		return err
	}

	if perc > 100 {
		perc = 100
	}
	return wrapErr(d.model.brightness(d.dev, perc))
}

// ForEachKey calls the provided callback function for each key available on
// the Elgato Stream Deck device, passing the KeyID as an argument.
func (d *Device) ForEachKey(cb func(k KeyID) error) error {
	if cb == nil {
		return errors.New("streamdeck: ForEachKey callback is nil")
	}

	for key := KEY_1; key < KEY_1+KeyID(d.model.keyCount); key++ {
		if err := cb(key); err != nil {
			return err
		}
	}
	return nil
}

// ForEachTouchPoint calls the provided callback function for each touch point
// available on the Elgato Stream Deck device, passing the TouchPointID as an
// argument.
func (d *Device) ForEachTouchPoint(cb func(tp TouchPointID) error) error {
	if cb == nil {
		return errors.New("streamdeck: ForEachTouchPoint callback is nil")
	}

	for tp := TOUCH_POINT_1; tp < TOUCH_POINT_1+TouchPointID(d.model.touchPointCount); tp++ {
		if err := cb(tp); err != nil {
			return err
		}
	}
	return nil
}

// ForEachDial calls the provided callback function for each dial
// available on the Elgato Stream Deck device, passing the DialID as an
// argument.
func (d *Device) ForEachDial(cb func(di DialID) error) error {
	if cb == nil {
		return errors.New("streamdeck: ForEachDial callback is nil")
	}

	for di := DIAL_1; di < DIAL_1+DialID(d.model.dialCount); di++ {
		if err := cb(di); err != nil {
			return err
		}
	}
	return nil
}
