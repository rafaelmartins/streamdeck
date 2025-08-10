package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/image/colornames"
	"rafaelmartins.com/p/streamdeck"
)

func main() {
	sn := ""
	if len(os.Args) > 1 {
		sn = os.Args[1]
	}

	device, err := streamdeck.GetDevice(sn)
	if err != nil {
		log.Fatalf("error: failed to get device: %v", err)
	}

	log.Println("Stream Deck Advanced Example")
	log.Println("This example demonstrates advanced features like info bar, touch strips and touch points")
	log.Println("Press Ctrl+C to exit")

	if err := device.Open(); err != nil {
		log.Fatalf("error: failed to open device: %v", err)
	}
	defer func() {
		log.Println("Closing device...")
		if err := device.Close(); err != nil {
			log.Printf("error: failed to close device: %v", err)
		}
	}()

	log.Printf("Device: %s (%s)", device.GetModelName(), device.GetModelID())
	log.Printf("Keys: %d, Touch Points: %d, Dials: %d", device.GetKeyCount(), device.GetTouchPointCount(), device.GetDialCount())

	if rect, err := device.GetInfoBarImageRectangle(); err != nil {
		if errors.Is(err, streamdeck.ErrDeviceInfoBarNotSupported) {
			log.Println("Info bar not supported on this device")
		} else {
			log.Printf("error: %v", err)
		}
	} else {
		log.Printf("Info bar supported, dimensions: %dx%d", rect.Dx(), rect.Dy())

		// create a gradient image for the info bar
		img := createGradientImage(rect, colornames.Blueviolet, colornames.Orangered)
		if err := device.SetInfoBarImage(img); err != nil {
			log.Printf("error: failed to create a gradient for the info bar: %v", err)
		} else {
			log.Println("Set info bar gradient")
		}
	}

	if rect, err := device.GetTouchStripImageRectangle(); err != nil {
		if errors.Is(err, streamdeck.ErrDeviceTouchStripNotSupported) {
			log.Println("Touch strip not supported on this device")
		} else {
			log.Printf("error: %v", err)
		}
	} else {
		log.Printf("Touch strip supported, dimensions: %dx%d", rect.Dx(), rect.Dy())

		// create a gradient image for the touch strip
		img := createGradientImage(rect, colornames.Blueviolet, colornames.Orangered)
		if err := device.SetTouchStripImage(img); err != nil {
			log.Printf("error: failed to create a gradient for the touch strip: %v", err)
		} else {
			log.Println("Set touch strip gradient")
		}

		if err := device.AddTouchStripTouchHandler(func(d *streamdeck.Device, typ streamdeck.TouchStripTouchType, p image.Point) error {
			log.Printf("Touch strip activated: (%s: %s)", typ, p)
			return nil
		}); err != nil {
			log.Printf("error: %v", err)
		}

		if err := device.AddTouchStripSwipeHandler(func(d *streamdeck.Device, origin image.Point, destination image.Point) error {
			log.Printf("Touch strip swiped: (%s -> %s)", origin, destination)
			return nil
		}); err != nil {
			log.Printf("error: %v", err)
		}
	}

	if count := device.GetTouchPointCount(); count == 0 {
		log.Println("Touch points not supported on this device")
	} else {
		log.Println("Setting up touch points...")

		colors := []color.Color{
			colornames.Orange,
			colornames.Purple,
		}

		if err := device.ForEachTouchPoint(func(tp streamdeck.TouchPointID) error {
			c := colors[int(tp-streamdeck.TOUCH_POINT_1)%len(colors)]

			if err := device.SetTouchPointColor(tp, c); err != nil {
				return fmt.Errorf("failed to set touch point %s color: %v", tp, err)
			} else {
				log.Printf("Set touch point %s color", tp)
			}

			return device.AddTouchPointHandler(tp, func(d *streamdeck.Device, tp *streamdeck.TouchPoint) error {
				touch := tp.GetID()
				log.Printf("Touch point %s activated!", touch)

				// flash the touch point
				if err := d.SetTouchPointColor(touch, color.White); err != nil {
					return err
				}

				duration := tp.WaitForRelease()
				log.Printf("Touch point %s held for %v", touch, duration)

				// restore original color
				return d.SetTouchPointColor(touch, colors[int(touch-streamdeck.TOUCH_POINT_1)%len(colors)])
			})
		}); err != nil {
			log.Printf("error: %v", err)
		}
	}

	if count := device.GetDialCount(); count == 0 {
		log.Println("Dials not supported on this device")
	} else {
		log.Println("Setting up dials...")

		if err := device.ForEachDial(func(d streamdeck.DialID) error {
			if err := device.AddDialRotateHandler(d, func(d *streamdeck.Device, di *streamdeck.Dial, delta int8) error {
				log.Printf("Dial %s rotated: %d", di, delta)
				return nil
			}); err != nil {
				return err
			}

			return device.AddDialSwitchHandler(d, func(d *streamdeck.Device, di *streamdeck.Dial) error {
				log.Printf("Dial %s pressed!", di)
				duration := di.WaitForRelease()
				log.Printf("Dial %s was held for %v", di, duration)
				return nil
			})
		}); err != nil {
			log.Printf("error: %v", err)
		}
	}

	setupKeys(device)

	// set brightness to maximum for better visibility
	if err := device.SetBrightness(100); err != nil {
		log.Printf("error: failed to set brightness: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	errChan := make(chan error, 1)
	go func() {
		if err := device.Listen(errChan); err != nil {
			errChan <- err
		}
	}()

	log.Println("Advanced features ready! Try operating keys, touch points, dials...")

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-sigChan:
			log.Println("Received interrupt signal")
			cancel()
			return

		case err := <-errChan:
			if err != nil {
				log.Printf("error: input error: %v", err)
			}

		case <-ticker.C:
			updateInfoBar(device)
			updateTouchStrip(device)
		}
	}
}

func setupKeys(device *streamdeck.Device) {
	log.Println("Setting up keys with different behaviors...")

	if err := device.ForEachKey(func(key streamdeck.KeyID) error {
		keyColor := getKeyColor(key)
		if err := device.SetKeyColor(key, keyColor); err != nil {
			return fmt.Errorf("failed to set %s color: %w", key, err)
		}

		return device.AddKeyHandler(key, func(d *streamdeck.Device, k *streamdeck.Key) error {
			key := k.GetID()
			log.Printf("Key %s pressed!", key)

			switch key {
			case streamdeck.KEY_1:
				// quick flash
				return flashKey(d, key, 200*time.Millisecond)

			case streamdeck.KEY_2:
				// long press detection
				return handleLongPress(d, key, k)

			case streamdeck.KEY_3:
				// color cycle
				return cycleKeyColor(d, key, k)

			default:
				// standard flash
				return flashKey(d, key, 500*time.Millisecond)
			}
		})
	}); err != nil {
		log.Printf("error: %v", err)
	}
}

func getKeyColor(key streamdeck.KeyID) color.Color {
	colors := []color.Color{
		colornames.Red,
		colornames.Lime,
		colornames.Blue,
		colornames.Yellow,
		colornames.Magenta,
		colornames.Cyan,
		colornames.Orange,
		colornames.Purple,
	}
	return colors[int(key-streamdeck.KEY_1)%len(colors)]
}

func flashKey(device *streamdeck.Device, key streamdeck.KeyID, duration time.Duration) error {
	// flash white
	if err := device.SetKeyColor(key, color.White); err != nil {
		return err
	}

	time.Sleep(duration)

	// restore original color
	return device.SetKeyColor(key, getKeyColor(key))
}

func handleLongPress(device *streamdeck.Device, key streamdeck.KeyID, k *streamdeck.Key) error {
	log.Printf("Key %s: waiting for release to measure duration...", key)

	// set to yellow to indicate we're measuring
	if err := device.SetKeyColor(key, colornames.Yellow); err != nil {
		return err
	}

	duration := k.WaitForRelease()

	var c color.Color
	if duration > 2*time.Second {
		log.Printf("Key %s: LONG press detected (%v)", key, duration)
		c = colornames.Red
	} else {
		log.Printf("Key %s: Short press (%v)", key, duration)
		c = colornames.Purple
	}

	// show result color briefly
	if err := device.SetKeyColor(key, c); err != nil {
		return err
	}

	time.Sleep(time.Second)

	// restore original color
	return device.SetKeyColor(key, getKeyColor(key))
}

func cycleKeyColor(device *streamdeck.Device, key streamdeck.KeyID, k *streamdeck.Key) error {
	colors := []color.Color{
		colornames.Red,
		colornames.Orange,
		colornames.Yellow,
		colornames.Green,
		colornames.Cyan,
		colornames.Blue,
		colornames.Purple,
		colornames.Fuchsia,
	}

	// channel to signal the goroutine to stop
	done := make(chan struct{})

	// cycle through colors while pressed
	go func() {
		ticker := time.NewTicker(300 * time.Millisecond)
		defer ticker.Stop()

		i := 0
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				if err := device.SetKeyColor(key, colors[i%len(colors)]); err != nil {
					log.Printf("error: failed while cycling color: %v", err)
					return
				}
				i++
			}
		}
	}()

	duration := k.WaitForRelease()
	close(done)
	log.Printf("Key %s: Color cycled for %v", key, duration)

	// restore original color
	return device.SetKeyColor(key, getKeyColor(key))
}

func createGradientImage(rect image.Rectangle, startColor color.RGBA, endColor color.RGBA) image.Image {
	img := image.NewRGBA(rect)

	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			// calculate gradient position (0.0 to 1.0)
			t := float64(x-rect.Min.X) / float64(rect.Dx())

			// interpolate colors
			r := float64(startColor.R)*(1-t) + float64(endColor.R)*t
			g := float64(startColor.G)*(1-t) + float64(endColor.G)*t
			b := float64(startColor.B)*(1-t) + float64(endColor.B)*t

			img.Set(x, y, color.RGBA{
				R: uint8(r),
				G: uint8(g),
				B: uint8(b),
				A: 255,
			})
		}
	}

	return img
}

func updateInfoBar(device *streamdeck.Device) {
	if !device.GetInfoBarSupported() {
		return
	}

	// create a simple color representation of time
	now := time.Now()
	timeColor := color.RGBA{
		R: uint8(50 + (now.Second() * 3)),
		G: uint8(100),
		B: uint8(200 - (now.Second() * 2)),
		A: 255,
	}

	if err := device.SetInfoBarColor(timeColor); err != nil {
		log.Printf("error: failed to set info bar color: %v", err)
	}
}

func updateTouchStrip(device *streamdeck.Device) {
	if !device.GetTouchStripSupported() {
		return
	}

	// create a simple color representation of time
	now := time.Now()
	timeColor := color.RGBA{
		R: uint8(50 + (now.Second() * 3)),
		G: uint8(100),
		B: uint8(200 - (now.Second() * 2)),
		A: 255,
	}

	if err := device.SetTouchStripColor(timeColor); err != nil {
		log.Printf("error: failed to set touch strip color: %v", err)
	}
}
