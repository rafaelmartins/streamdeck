package streamdeck

import (
	"fmt"
	"image"
	"log"
	"sync"
	"time"
)

// KeyHandlerError represents an error returned by a key handler including the
// key identifier.
type KeyHandlerError struct {
	KeyID KeyID
	Err   error
}

// Error returns a string representation of a key handler error.
func (b KeyHandlerError) Error() string {
	return fmt.Sprintf("%s [%s]", b.Err, b.KeyID)
}

// Unwrap returns the underlying key handler error.
func (b KeyHandlerError) Unwrap() error {
	return b.Err
}

// KeyHandler represents a callback function that is called when a key is
// pressed. It receives the Device and Key instances as parameters.
type KeyHandler func(d *Device, k *Key) error

// Key represents a physical key on the Elgato Stream Deck device.
type Key struct {
	id       KeyID
	handlers []KeyHandler
	input    *input
}

func (k *Key) addHandler(h KeyHandler) {
	if h == nil || k.input == nil {
		return
	}

	k.input.mtx.Lock()
	k.handlers = append(k.handlers, h)
	k.input.mtx.Unlock()
}

// WaitForRelease blocks until the key is released and returns the duration
// the key was held down. This method should be called from within a
// KeyHandler.
func (k *Key) WaitForRelease() time.Duration {
	<-k.input.channel
	return k.input.duration
}

// GetID returns the KeyID identifier for this key.
func (k *Key) GetID() KeyID {
	return k.id
}

// String returns a string representation of the Key.
func (k *Key) String() string {
	return k.id.String()
}

// KeyID represents a physical Elgato Stream Deck device key.
type KeyID byte

// String returns a string representation of the KeyID.
func (id KeyID) String() string {
	return fmt.Sprintf("KEY_%d", id)
}

// Elgato Stream Deck key identifiers. These constants represent the physical
// keys on the device, depending on the supported models.
const (
	KEY_1 KeyID = iota + 1
	KEY_2
	KEY_3
	KEY_4
	KEY_5
	KEY_6
	KEY_7
	KEY_8
	KEY_9
	KEY_10
	KEY_11
	KEY_12
	KEY_13
	KEY_14
	KEY_15
)

// TouchPointHandlerError represents an error returned by a touch point
// handler including the touch point identifier.
type TouchPointHandlerError struct {
	TouchPointID TouchPointID
	Err          error
}

// Error returns a string representation of a touch point handler error.
func (b TouchPointHandlerError) Error() string {
	return fmt.Sprintf("%s [%s]", b.Err, b.TouchPointID)
}

// Unwrap returns the underlying touch point handler error.
func (b TouchPointHandlerError) Unwrap() error {
	return b.Err
}

// TouchPointHandler represents a callback function that is called when a
// touch point is activated. It receives the Device and TouchPoint instances
// as parameters.
type TouchPointHandler func(d *Device, tp *TouchPoint) error

// TouchPoint represents a touch-sensitive area on supported Elgato Stream
// Deck devices.
type TouchPoint struct {
	id       TouchPointID
	handlers []TouchPointHandler
	input    *input
}

func (tp *TouchPoint) addHandler(h TouchPointHandler) {
	if h == nil || tp.input == nil {
		return
	}

	tp.input.mtx.Lock()
	tp.handlers = append(tp.handlers, h)
	tp.input.mtx.Unlock()
}

// WaitForRelease blocks until the touch point is released and returns the
// duration the touch point was held down. This method should be called from
// within a TouchPointHandler.
func (tp *TouchPoint) WaitForRelease() time.Duration {
	<-tp.input.channel
	return tp.input.duration
}

// GetID returns the TouchPointID identifier for this touch point.
func (tp *TouchPoint) GetID() TouchPointID {
	return tp.id
}

// String returns a string representation of the TouchPoint.
func (tp *TouchPoint) String() string {
	return tp.id.String()
}

// TouchPointID represents a physical Elgato Stream Deck device touch point.
type TouchPointID byte

// String returns a string representation of the TouchPointID.
func (id TouchPointID) String() string {
	return fmt.Sprintf("TOUCH_POINT_%d", id)
}

// Elgato Stream Deck touch point identifiers. These constants represent the
// touch points on the device, depending on the supported models.
const (
	TOUCH_POINT_1 TouchPointID = iota + 1
	TOUCH_POINT_2
)

// DialSwitchHandlerError represents an error returned by a dial switch
// handler including the dial identifier.
type DialHandlerError struct {
	DialID DialID
	Err    error
}

// Error returns a string representation of a dial handler error.
func (b DialHandlerError) Error() string {
	return fmt.Sprintf("%s [%s]", b.Err, b.DialID)
}

// Unwrap returns the underlying dial handler error.
func (b DialHandlerError) Unwrap() error {
	return b.Err
}

// DialSwitchHandler represents a callback function that is called when a
// dial switch is activated. It receives the Device and Dial instances as
// parameters.
type DialSwitchHandler func(d *Device, di *Dial) error

// DialRotateHandler represents a callback function that is called when a
// dial is rotated. It receives the Device, the Dial instance and the rotation
// delta as parameters.
type DialRotateHandler func(d *Device, di *Dial, delta int8) error

// Dial represents a rotative encoder with switch available on some Elgato
// Stream Deck devices.
type Dial struct {
	id             DialID
	switchHandlers []DialSwitchHandler
	rotateHandlers []DialRotateHandler
	input          *input
}

func (d *Dial) addSwitchHandler(h DialSwitchHandler) {
	if h == nil || d.input == nil {
		return
	}

	d.input.mtx.Lock()
	d.switchHandlers = append(d.switchHandlers, h)
	d.input.mtx.Unlock()
}

func (d *Dial) addRotateHandler(h DialRotateHandler) {
	if h == nil || d.input == nil {
		return
	}

	d.input.mtx.Lock()
	d.rotateHandlers = append(d.rotateHandlers, h)
	d.input.mtx.Unlock()
}

// WaitForRelease blocks until the dial switch is released and returns the
// duration the dial switch was held closed. This method should be called from
// within a DialSwitchHandler.
func (d *Dial) WaitForRelease() time.Duration {
	<-d.input.channel
	return d.input.duration
}

// GetID returns the DialID identifier for this dial.
func (d *Dial) GetID() DialID {
	return d.id
}

// String returns a string representation of the Dial.
func (d *Dial) String() string {
	return d.id.String()
}

// DialID represents a physical Elgato Stream Deck device dial.
type DialID byte

// String returns a string representation of the DialID.
func (id DialID) String() string {
	return fmt.Sprintf("DIAL_%d", id)
}

// Elgato Stream Deck dial identifiers. These constants represent the
// dials on the device depending on the supported models.
const (
	DIAL_1 DialID = iota + 1
	DIAL_2
	DIAL_3
	DIAL_4
)

// TouchStripTouchType represents a touch strip touch type
type TouchStripTouchType byte

// String returns a string representation of the TouchStripTouchType.
func (t TouchStripTouchType) String() string {
	switch t {
	case TOUCH_STRIP_TOUCH_TYPE_SHORT:
		return "TOUCH_STRIP_TOUCH_TYPE_SHORT"
	case TOUCH_STRIP_TOUCH_TYPE_LONG:
		return "TOUCH_STRIP_TOUCH_TYPE_LONG"
	default:
		return ""
	}
}

// Elgato Stream Deck touch strip touch types. These constants represent the
// duration while the touch strip was touched.
const (
	TOUCH_STRIP_TOUCH_TYPE_SHORT TouchStripTouchType = iota + 1
	TOUCH_STRIP_TOUCH_TYPE_LONG
)

// TouchStripTouchHandlerError represents an error returned by a touch strip
// touch handler including the touch type and touched point.
type TouchStripTouchHandlerError struct {
	Type  TouchStripTouchType
	Point image.Point
	Err   error
}

// Error returns a string representation of a touch strip touch handler error.
func (b TouchStripTouchHandlerError) Error() string {
	return fmt.Sprintf("%s [%s: %s]", b.Err, b.Type, b.Point)
}

// Unwrap returns the underlying touch strip touch handler error.
func (b TouchStripTouchHandlerError) Unwrap() error {
	return b.Err
}

// TouchStripSwipeHandlerError represents an error returned by a touch strip
// swipe handler including the swipe origin and destination points.
type TouchStripSwipeHandlerError struct {
	Origin      image.Point
	Destination image.Point
	Err         error
}

// Error returns a string representation of a touch strip swipe handler error.
func (b TouchStripSwipeHandlerError) Error() string {
	return fmt.Sprintf("%s [%s %s]", b.Err, b.Origin, b.Destination)
}

// Unwrap returns the underlying touch strip swipe handler error.
func (b TouchStripSwipeHandlerError) Unwrap() error {
	return b.Err
}

// TouchStripTouchHandler represents a callback function that is called when a
// touch strip is touched. It receives the Device instance, the touch strip
// touch type and point as parameters.
type TouchStripTouchHandler func(d *Device, t TouchStripTouchType, p image.Point) error

// TouchStripSwipeHandler represents a callback function that is called when a
// touch strip is swiped. It receives the Device instance, the origin point and
// the destination point as parameters.
type TouchStripSwipeHandler func(d *Device, origin image.Point, destination image.Point) error

type touchStrip struct {
	touchHandlers []TouchStripTouchHandler
	swipeHandlers []TouchStripSwipeHandler
	input         *input
}

func (t *touchStrip) addTouchHandler(h TouchStripTouchHandler) {
	if h == nil || t.input == nil {
		return
	}

	t.input.mtx.Lock()
	t.touchHandlers = append(t.touchHandlers, h)
	t.input.mtx.Unlock()
}

func (t *touchStrip) addSwipeHandler(h TouchStripSwipeHandler) {
	if h == nil || t.input == nil {
		return
	}

	t.input.mtx.Lock()
	t.swipeHandlers = append(t.swipeHandlers, h)
	t.input.mtx.Unlock()
}

type input struct {
	mtx        sync.Mutex
	device     *Device
	channel    chan bool
	pressed    time.Time
	released   time.Time
	duration   time.Duration
	key        *Key
	tp         *TouchPoint
	dial       *Dial
	touchStrip *touchStrip
}

func newInputs(d *Device, numKeys byte, numTouchPoints byte) []*input {
	rv := []*input{}
	for i := KEY_1; i < KeyID(numKeys+1); i++ {
		in := &input{
			device: d,
			key: &Key{
				id: i,
			},
		}
		in.key.input = in
		rv = append(rv, in)
	}
	for i := TOUCH_POINT_1; i < TOUCH_POINT_1+TouchPointID(numTouchPoints); i++ {
		in := &input{
			device: d,
			tp: &TouchPoint{
				id: i,
			},
		}
		in.tp.input = in
		rv = append(rv, in)
	}
	return rv
}

func newDialInputs(d *Device, numDials byte) []*input {
	rv := []*input{}
	for i := DIAL_1; i < DialID(numDials+1); i++ {
		in := &input{
			device: d,
			dial: &Dial{
				id: i,
			},
		}
		in.dial.input = in
		rv = append(rv, in)
	}
	return rv
}

func newTouchStripInput(d *Device) *input {
	rv := &input{
		device:     d,
		touchStrip: &touchStrip{},
	}
	rv.touchStrip.input = rv
	return rv
}

func (in *input) press(t time.Time, errCh chan error) {
	in.mtx.Lock()
	defer in.mtx.Unlock()

	in.channel = make(chan bool)
	in.pressed = t
	in.released = time.Time{}
	in.duration = 0

	if in.key != nil {
		for _, h := range in.key.handlers {
			go func(in *input, hnd KeyHandler) {
				if err := hnd(in.device, in.key); err != nil {
					e := KeyHandlerError{
						KeyID: in.key.id,
						Err:   err,
					}

					if errCh != nil {
						select {
						case errCh <- e:
						default:
						}
					} else {
						log.Printf("error: %s", e)
					}
				}
			}(in, h)
		}
	}

	if in.tp != nil {
		for _, h := range in.tp.handlers {
			go func(in *input, hnd TouchPointHandler) {
				if err := hnd(in.device, in.tp); err != nil {
					e := TouchPointHandlerError{
						TouchPointID: in.tp.id,
						Err:          err,
					}

					if errCh != nil {
						select {
						case errCh <- e:
						default:
						}
					} else {
						log.Printf("error: %s", e)
					}
				}
			}(in, h)
		}
	}

	if in.dial != nil {
		for _, h := range in.dial.switchHandlers {
			go func(in *input, hnd DialSwitchHandler) {
				if err := hnd(in.device, in.dial); err != nil {
					e := DialHandlerError{
						DialID: in.dial.id,
						Err:    err,
					}

					if errCh != nil {
						select {
						case errCh <- e:
						default:
						}
					} else {
						log.Printf("error: %s", e)
					}
				}
			}(in, h)
		}
	}
}

func (in *input) release(t time.Time) {
	in.mtx.Lock()
	defer in.mtx.Unlock()

	// currently released
	if !in.released.IsZero() {
		return
	}

	in.released = t
	in.duration = in.released.Sub(in.pressed)
	in.pressed = time.Time{}
	close(in.channel)
}

func (in *input) rotate(delta int8, errCh chan error) {
	in.mtx.Lock()
	defer in.mtx.Unlock()

	if in.dial == nil {
		return
	}

	for _, h := range in.dial.rotateHandlers {
		go func(in *input, hnd DialRotateHandler) {
			if err := hnd(in.device, in.dial, delta); err != nil {
				e := DialHandlerError{
					DialID: in.dial.id,
					Err:    err,
				}

				if errCh != nil {
					select {
					case errCh <- e:
					default:
					}
				} else {
					log.Printf("error: %s", e)
				}
			}
		}(in, h)
	}
}

func (in *input) touch(t TouchStripTouchType, p image.Point, errCh chan error) {
	in.mtx.Lock()
	defer in.mtx.Unlock()

	if in.touchStrip == nil {
		return
	}

	for _, h := range in.touchStrip.touchHandlers {
		go func(in *input, hnd TouchStripTouchHandler) {
			if err := hnd(in.device, t, p); err != nil {
				e := TouchStripTouchHandlerError{
					Type:  t,
					Point: p,
					Err:   err,
				}

				if errCh != nil {
					select {
					case errCh <- e:
					default:
					}
				} else {
					log.Printf("error: %s", e)
				}
			}
		}(in, h)
	}
}

func (in *input) swipe(origin image.Point, destination image.Point, errCh chan error) {
	in.mtx.Lock()
	defer in.mtx.Unlock()

	if in.touchStrip == nil {
		return
	}

	for _, h := range in.touchStrip.swipeHandlers {
		go func(in *input, hnd TouchStripSwipeHandler) {
			if err := hnd(in.device, origin, destination); err != nil {
				e := TouchStripSwipeHandlerError{
					Origin:      origin,
					Destination: destination,
					Err:         err,
				}

				if errCh != nil {
					select {
					case errCh <- e:
					default:
					}
				} else {
					log.Printf("error: %s", e)
				}
			}
		}(in, h)
	}
}
