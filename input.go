package streamdeck

import (
	"fmt"
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
// touch points on the device, numbered from 1 to 2 depending on the
// supported models.
const (
	TOUCH_POINT_1 TouchPointID = iota + 1
	TOUCH_POINT_2
)

type input struct {
	mtx      sync.Mutex
	device   *Device
	channel  chan bool
	pressed  time.Time
	released time.Time
	duration time.Duration
	key      *Key
	tp       *TouchPoint
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
