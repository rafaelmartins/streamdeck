package main

import (
	"context"
	"embed"
	"image"
	"image/color"
	"log"
	"math"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/image/colornames"
	"rafaelmartins.com/p/streamdeck"
)

//go:embed *.png
var iconFS embed.FS

func main() {
	sn := ""
	if len(os.Args) > 1 {
		sn = os.Args[1]
	}

	device, err := streamdeck.GetDevice(sn)
	if err != nil {
		log.Fatalf("error: failed to get device: %v", err)
	}

	log.Println("Stream Deck Image Examples")
	log.Println("This example shows different ways to set images on keys")
	log.Println("Press Ctrl+C to exit")

	if err := device.Open(); err != nil {
		log.Fatalf("error: failed to open device: %v", err)
	}
	defer func() {
		if err := device.Close(); err != nil {
			log.Printf("error: failed to close device: %v", err)
		}
	}()

	log.Printf("Setting up %d keys with different image types", device.GetKeyCount())

	setupImageExamples(device)

	if err := device.ForEachKey(func(key streamdeck.KeyID) error {
		return device.AddKeyHandler(key, func(d *streamdeck.Device, k *streamdeck.Key) error {
			key := k.GetID()
			log.Printf("Key %s pressed!", key)

			// flash white briefly
			if err := d.SetKeyColor(key, color.White); err != nil {
				return err
			}

			duration := k.WaitForRelease()
			log.Printf("Key %s held for %v", key, duration)

			// restore original image
			return restoreKeyImage(d, key)
		})
	}); err != nil {
		log.Printf("error: %v", err)
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

	log.Println("Image examples ready! Press any key...")

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
				log.Printf("Input error: %v", err)
			}
		}
	}
}

func setupImageKey(device *streamdeck.Device, key streamdeck.KeyID, withLog bool) error {
	printf := func(format string, v ...any) {
		if withLog {
			log.Printf(format, v...)
		}
	}

	rect, err := device.GetKeyImageRectangle()
	if err != nil {
		return err
	}

	switch key {
	case streamdeck.KEY_1:
		printf("Key %d: Setting solid red color", key)
		return device.SetKeyColor(key, colornames.Red)

	case streamdeck.KEY_2:
		printf("Key %d: Setting gradient image", key)
		return device.SetKeyImage(key, createGradient(rect, colornames.Red, colornames.Blue))

	case streamdeck.KEY_3:
		printf("Key %d: Setting embedded icon", key)
		if err = device.SetKeyImageFromFS(key, iconFS, "play.png"); err != nil {
			printf("error: failed to load embedded icon, using pattern: %v", err)
			return device.SetKeyImage(key, createCheckerboard(rect, colornames.Yellow, colornames.Orange))
		}

	case streamdeck.KEY_4:
		printf("Key %d: Setting checkerboard pattern", key)
		return device.SetKeyImage(key, createCheckerboard(rect, colornames.Lime, colornames.Green))

	case streamdeck.KEY_5:
		printf("Key %d: Setting circle pattern", key)
		return device.SetKeyImage(key, createCircle(rect, colornames.Purple, colornames.Mediumpurple))

	case streamdeck.KEY_6:
		printf("Key %d: Setting stripes pattern", key)
		return device.SetKeyImage(key, createStripes(rect, colornames.Cyan, colornames.Teal))

	case streamdeck.KEY_7:
		printf("Key %d: Setting spiral pattern", key)
		return device.SetKeyImage(key, createSpiral(rect, colornames.Darkorange, colornames.Brown))

	case streamdeck.KEY_8:
		printf("Key %d: Setting diamond pattern", key)
		return device.SetKeyImage(key, createDiamond(rect, colornames.Lightgreen, colornames.Darkgreen))

	default:
		printf("Key %d: Setting random color", key)
		randomColor := color.RGBA{
			R: uint8(key * 37),
			G: uint8(key * 73),
			B: uint8(key * 109),
			A: 255,
		}
		return device.SetKeyColor(key, randomColor)
	}
	return nil
}

func setupImageExamples(device *streamdeck.Device) {
	if err := device.ForEachKey(func(key streamdeck.KeyID) error {
		return setupImageKey(device, key, true)
	}); err != nil {
		log.Printf("error: failed to set image for key: %v", err)
	}
}

func restoreKeyImage(device *streamdeck.Device, key streamdeck.KeyID) error {
	return setupImageKey(device, key, false)
}

func createGradient(rect image.Rectangle, color1, color2 color.RGBA) image.Image {
	img := image.NewRGBA(rect)

	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			t := float64(x-rect.Min.X) / float64(rect.Dx())

			r := float64(color1.R)*(1-t) + float64(color2.R)*t
			g := float64(color1.G)*(1-t) + float64(color2.G)*t
			b := float64(color1.B)*(1-t) + float64(color2.B)*t

			img.Set(x, y, color.RGBA{uint8(r), uint8(g), uint8(b), 255})
		}
	}

	return img
}

func createCheckerboard(rect image.Rectangle, color1, color2 color.RGBA) image.Image {
	img := image.NewRGBA(rect)
	squareSize := rect.Dx() / 8

	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if ((x/squareSize)+(y/squareSize))%2 == 0 {
				img.Set(x, y, color1)
			} else {
				img.Set(x, y, color2)
			}
		}
	}

	return img
}

func createCircle(rect image.Rectangle, fillColor, bgColor color.RGBA) image.Image {
	img := image.NewRGBA(rect)

	centerX := rect.Dx() / 2
	centerY := rect.Dy() / 2
	radius := float64(min(centerX, centerY)) * 0.8

	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			dx := float64(x - centerX)
			dy := float64(y - centerY)
			distance := math.Sqrt(dx*dx + dy*dy)

			if distance <= radius {
				img.Set(x, y, fillColor)
			} else {
				img.Set(x, y, bgColor)
			}
		}
	}

	return img
}

func createStripes(rect image.Rectangle, color1, color2 color.RGBA) image.Image {
	img := image.NewRGBA(rect)
	stripeWidth := rect.Dx() / 8

	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			if (x/stripeWidth)%2 == 0 {
				img.Set(x, y, color1)
			} else {
				img.Set(x, y, color2)
			}
		}
	}

	return img
}

func createSpiral(rect image.Rectangle, color1, color2 color.RGBA) image.Image {
	img := image.NewRGBA(rect)

	centerX := float64(rect.Dx()) / 2
	centerY := float64(rect.Dy()) / 2
	maxRadius := math.Min(centerX, centerY)

	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			dx := float64(x) - centerX
			dy := float64(y) - centerY

			angle := math.Atan2(dy, dx)
			if angle < 0 {
				angle += 2 * math.Pi
			}

			distance := math.Sqrt(dx*dx + dy*dy)

			// simple Archimedean spiral: r = a * θ
			turns := 3.0
			spiralSpacing := maxRadius / (turns * 2 * math.Pi)
			armThickness := maxRadius * 0.06

			// check if point is on any spiral arm
			isOnSpiral := false
			for i := 0; i < int(turns); i++ {
				expectedRadius := spiralSpacing * (angle + float64(i)*2*math.Pi)
				if expectedRadius <= maxRadius && math.Abs(distance-expectedRadius) < armThickness {
					isOnSpiral = true
					break
				}
			}

			if isOnSpiral {
				img.Set(x, y, color1)
			} else {
				img.Set(x, y, color2)
			}
		}
	}

	return img
}

func createDiamond(rect image.Rectangle, fillColor, bgColor color.RGBA) image.Image {
	img := image.NewRGBA(rect)

	centerX := rect.Dx() / 2
	centerY := rect.Dy() / 2
	size := min(centerX, centerY)

	for y := rect.Min.Y; y < rect.Max.Y; y++ {
		for x := rect.Min.X; x < rect.Max.X; x++ {
			dx := int(math.Abs(float64(x - centerX)))
			dy := int(math.Abs(float64(y - centerY)))

			if dx+dy <= size {
				img.Set(x, y, fillColor)
			} else {
				img.Set(x, y, bgColor)
			}
		}
	}

	return img
}
