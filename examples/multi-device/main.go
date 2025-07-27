package main

import (
	"context"
	"fmt"
	"image/color"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"golang.org/x/image/colornames"
	"rafaelmartins.com/p/streamdeck"
)

var baseColors = [][]color.Color{

	// red theme
	{
		color.RGBA{255, 100, 100, 255},
		color.RGBA{255, 150, 150, 255},
		color.RGBA{255, 50, 50, 255},
		color.RGBA{200, 50, 50, 255},
	},

	// blue theme
	{
		color.RGBA{100, 100, 255, 255},
		color.RGBA{150, 150, 255, 255},
		color.RGBA{50, 50, 255, 255},
		color.RGBA{50, 50, 200, 255},
	},

	// green theme
	{
		color.RGBA{100, 255, 100, 255},
		color.RGBA{150, 255, 150, 255},
		color.RGBA{50, 255, 50, 255},
		color.RGBA{50, 200, 50, 255},
	},

	// purple theme
	{
		color.RGBA{200, 100, 255, 255},
		color.RGBA{220, 150, 255, 255},
		color.RGBA{180, 50, 255, 255},
		color.RGBA{150, 50, 200, 255},
	},

	// orange theme
	{
		color.RGBA{255, 165, 100, 255},
		color.RGBA{255, 200, 150, 255},
		color.RGBA{255, 140, 50, 255},
		color.RGBA{200, 100, 50, 255},
	},
}

type DeviceManager struct {
	devices []*streamdeck.Device
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

func NewDeviceManager() *DeviceManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &DeviceManager{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (dm *DeviceManager) Initialize() error {
	devices, err := streamdeck.Enumerate()
	if err != nil {
		return fmt.Errorf("failed to enumerate devices: %w", err)
	}

	if len(devices) == 0 {
		return fmt.Errorf("no Stream Deck devices found")
	}

	log.Printf("Found %d Stream Deck device(s)", len(devices))

	for i, device := range devices {
		if err := device.Open(); err != nil {
			log.Printf("Failed to open device %d: %v", i, err)
			continue
		}

		log.Printf("Device %d: %s (%s) with %d keys", i, device.GetModelName(), device.GetSerialNumber(), device.GetKeyCount())
		dm.devices = append(dm.devices, device)
	}

	if len(dm.devices) == 0 {
		return fmt.Errorf("failed to open any devices")
	}
	return nil
}

func (dm *DeviceManager) SetupDevices() error {
	for i, device := range dm.devices {
		if err := dm.setupDevice(device, i); err != nil {
			return fmt.Errorf("failed to setup device %d: %v", i, err)
		}
	}
	return nil
}

func (dm *DeviceManager) setupDevice(device *streamdeck.Device, deviceIndex int) error {
	if err := device.SetBrightness(75); err != nil {
		return fmt.Errorf("failed to set brightness: %w", err)
	}

	colors := baseColors[deviceIndex%len(baseColors)]

	return device.ForEachKey(func(key streamdeck.KeyID) error {
		keyColor := colors[int(key-streamdeck.KEY_1)%(len(colors))]

		if err := device.SetKeyColor(key, keyColor); err != nil {
			return fmt.Errorf("failed to set color for key %s on device %d: %v", key, deviceIndex, err)
		}

		if err := device.AddKeyHandler(key, func(d *streamdeck.Device, k *streamdeck.Key) error {
			key := k.GetID()
			log.Printf("Device %d, Key %s pressed!", deviceIndex, key)

			// flash the key white
			if err := d.SetKeyColor(key, color.White); err != nil {
				return err
			}

			// also flash a key on other devices for synchronization effect
			dm.flashOtherDevices(deviceIndex, key)

			duration := k.WaitForRelease()
			log.Printf("Device %d, Key %s held for %v", deviceIndex, key, duration)

			// restore original color
			return d.SetKeyColor(key, colors[int(key-1)%len(colors)])
		}); err != nil {
			return fmt.Errorf("failed to add handler for key %s on device %d: %v", key, deviceIndex, err)
		}
		return nil
	})
}

func (dm *DeviceManager) flashOtherDevices(devSrc int, key streamdeck.KeyID) {
	for i, device := range dm.devices {
		if i == devSrc {
			continue
		}

		go func(dev *streamdeck.Device, idx int) {
			if byte(key) <= dev.GetKeyCount() {
				// flash briefly in white
				if err := dev.SetKeyColor(key, color.White); err != nil {
					log.Printf("error: failed to flash key %s on device %d: %v", key, idx, err)
					return
				}

				time.Sleep(300 * time.Millisecond)

				// restore original color based on device theme
				colors := baseColors[idx%len(baseColors)]
				if err := dev.SetKeyColor(key, colors[int(key-1)%len(colors)]); err != nil {
					log.Printf("error: failed to restore key %s color on device %d: %v", key, idx, err)
				}
			}
		}(device, i)
	}
}

func (dm *DeviceManager) Listen() {
	for i, device := range dm.devices {
		dm.wg.Add(1)

		go func(dev *streamdeck.Device, deviceIndex int) {
			defer dm.wg.Done()

			errCh := make(chan error, 1)
			go func() {
				if err := dev.Listen(errCh); err != nil {
					select {
					case errCh <- err:
					default:
					}
				}
			}()

			for {
				select {
				case <-dm.ctx.Done():
					log.Printf("Device %d listener stopping", deviceIndex)
					return

				case err := <-errCh:
					if err != nil {
						log.Printf("error: device %d: %v", deviceIndex, err)
					}
				}
			}
		}(device, i)
	}
}

func (dm *DeviceManager) CreateSynchronizedEffect() {
	dm.wg.Add(1)

	go func() {
		defer dm.wg.Done()

		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-dm.ctx.Done():
				return

			case <-ticker.C:
				dm.runWaveEffect()
			}
		}
	}()
}

func (dm *DeviceManager) runWaveEffect() {
	log.Println("Running synchronized wave effect...")

	var dev *streamdeck.Device
	for _, device := range dm.devices {
		if dev == nil || device.GetKeyCount() > dev.GetKeyCount() {
			dev = device
		}
	}
	if dev == nil {
		log.Println("error: no devices available for wave effect")
		return
	}

	dev.ForEachKey(func(key streamdeck.KeyID) error {
		for _, device := range dm.devices {
			if byte(key) <= device.GetKeyCount() {
				if err := device.SetKeyColor(key, colornames.Yellow); err != nil {
					log.Printf("error: failed to set yellow color for wave effect: %v", err)
				}
			}
		}

		time.Sleep(100 * time.Millisecond)

		for i, device := range dm.devices {
			if byte(key) <= device.GetKeyCount() {
				colors := baseColors[i%len(baseColors)]
				if err := device.SetKeyColor(key, colors[int(key-1)%len(colors)]); err != nil {
					log.Printf("error: failed to restore color after wave effect: %v", err)
				}
			}
		}
		return nil
	})
}

func (dm *DeviceManager) Shutdown() {
	log.Println("Shutting down device manager...")

	dm.cancel()
	dm.wg.Wait()

	for i, device := range dm.devices {
		if err := device.Close(); err != nil {
			log.Printf("error: failed to close device %d: %v", i, err)
		} else {
			log.Printf("Device %d closed successfully", i)
		}
	}
}

func main() {
	log.Println("Stream Deck Multi-Device Example")
	log.Println("This example demonstrates working with multiple Stream Deck devices")
	log.Println("Press any key on any device to see synchronized effects")
	log.Println("Press Ctrl+C to exit")

	dm := NewDeviceManager()
	if err := dm.Initialize(); err != nil {
		log.Fatalf("error: failed to initialize devices: %v", err)
	}
	defer dm.Shutdown()

	if err := dm.SetupDevices(); err != nil {
		log.Fatalf("error: failed to setup devices: %v", err)
	}

	dm.Listen()

	dm.CreateSynchronizedEffect()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	log.Printf("Multi-device setup complete with %d devices!", len(dm.devices))
	log.Println("Try pressing keys to see cross-device synchronization!")

	<-sigChan
	log.Println("Received interrupt signal, shutting down...")
}
