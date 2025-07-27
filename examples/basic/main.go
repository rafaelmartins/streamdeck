package main

import (
	"context"
	"fmt"
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

	log.Println("Stream Deck Basic Example")
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

	log.Printf("Connected to: %s", device.GetModelName())
	log.Printf("Serial Number: %s", device.GetSerialNumber())
	log.Printf("Available keys: %d", device.GetKeyCount())

	colors := []color.Color{
		colornames.Red,
		colornames.Green,
		colornames.Blue,
		colornames.Yellow,
		colornames.Magenta,
		colornames.Cyan,
	}

	if err := device.ForEachKey(func(key streamdeck.KeyID) error {
		if k := byte(key - streamdeck.KEY_1); k >= byte(len(colors)) {
			return nil
		}

		if err := device.SetKeyColor(key, colors[byte(key)-1]); err != nil {
			return fmt.Errorf("failed to set color for key %s: %v", key, err)
		}

		log.Printf("Set key %s to color", key)

		return device.AddKeyHandler(key, func(d *streamdeck.Device, k *streamdeck.Key) error {
			key := k.GetID()
			log.Printf("Key %s pressed!", key)

			// flash the key by setting it to white briefly
			if err := d.SetKeyColor(key, color.White); err != nil {
				return err
			}

			duration := k.WaitForRelease()
			log.Printf("Key %s was held for %v", key, duration)

			// restore original color
			return d.SetKeyColor(key, colors[key-streamdeck.KEY_1])
		})
	}); err != nil {
		log.Printf("error: %v", err)
	}

	if err := device.SetBrightness(75); err != nil {
		log.Printf("error: failed to set brightness: %v", err)
	} else {
		log.Println("Set brightness to 75%")
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

	log.Println("Device ready! Press any coloured key on the Stream Deck...")

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

		case <-time.After(100 * time.Millisecond):
		}
	}
}
