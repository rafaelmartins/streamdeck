package main

import (
	"errors"
	"fmt"
	"log"

	"golang.org/x/image/colornames"
	"rafaelmartins.com/p/streamdeck"
)

func main() {
	log.Println("Stream Deck Device Information Example")

	devices, err := streamdeck.Enumerate()
	if err != nil {
		log.Fatalf("error: failed to enumerate devices: %v", err)
	}

	if len(devices) == 0 {
		log.Println("error: no Stream Deck devices found")
		return
	}

	fmt.Printf("Found %d Stream Deck device(s):\n\n", len(devices))

	for i, device := range devices {
		fmt.Printf("Device %d:\n", i+1)
		fmt.Println("---------")

		if err := device.Open(); err != nil {
			fmt.Printf("  Error opening device: %v\n\n", err)
			continue
		}

		fmt.Printf("  Model Name: %s\n", device.GetModelName())
		fmt.Printf("  Model ID: %s\n", device.GetModelID())
		fmt.Printf("  Serial Number: %s\n", device.GetSerialNumber())

		if firmwareVersion, err := device.GetFirmwareVersion(); err != nil {
			fmt.Printf("  Firmware Version: Error - %v\n", err)
		} else {
			fmt.Printf("  Firmware Version: %s\n", firmwareVersion)
		}

		fmt.Printf("  Key Count: %d\n", device.GetKeyCount())
		fmt.Printf("  Touch Point Count: %d\n", device.GetTouchPointCount())
		fmt.Printf("  Dial Count: %d\n", device.GetDialCount())

		if rect, err := device.GetKeyImageRectangle(); err != nil {
			fmt.Printf("  Key Image Size: Error - %v\n", err)
		} else {
			fmt.Printf("  Key Image Size: %dx%d\n", rect.Dx(), rect.Dy())
		}

		if rect, err := device.GetInfoBarImageRectangle(); err != nil {
			if errors.Is(err, streamdeck.ErrDeviceInfoBarNotSupported) {
				fmt.Println("  Info Bar: Not supported")
			} else {
				fmt.Printf("  Info Bar: Error - %v\n", err)
			}
		} else {
			fmt.Printf("  Info Bar: Supported (%dx%d)\n", rect.Dx(), rect.Dy())
		}

		if rect, err := device.GetTouchStripImageRectangle(); err != nil {
			if errors.Is(err, streamdeck.ErrDeviceTouchStripNotSupported) {
				fmt.Println("  Touch Strip: Not supported")
			} else {
				fmt.Printf("  Touch Strip: Error - %v\n", err)
			}
		} else {
			fmt.Printf("  Touch Strip: Supported (%dx%d)\n", rect.Dx(), rect.Dy())
		}

		fmt.Println("  Testing basic functionality...")
		if err := testBasicFunctionality(device); err != nil {
			fmt.Printf("  Test Result: Failed - %v\n", err)
		} else {
			fmt.Println("  Test Result: Passed")
		}

		if err := device.Close(); err != nil {
			fmt.Printf("  Error closing device: %v\n", err)
		}
		fmt.Println()
	}

	fmt.Println("\nTesting device selection:")
	fmt.Println("-------------------------")

	sn := devices[0].GetSerialNumber()
	fmt.Printf("Device serial number: %s\n", sn)

	device, err := streamdeck.GetDevice(sn)
	if err != nil {
		fmt.Printf("Failed to get device by serial number: %v\n", err)
	} else {
		fmt.Printf("Successfully got device by serial number: %s\n", sn)

		if err := device.Open(); err != nil {
			fmt.Printf("Failed to open device by serial: %v\n", err)
		} else {
			fmt.Println("Successfully opened device by serial number")
			device.Close()
		}
	}

	if len(devices) == 1 {
		device, err := streamdeck.GetDevice("")
		if err != nil {
			fmt.Printf("Failed to get device with empty serial: %v\n", err)
		} else {
			fmt.Println("Successfully got device with empty serial (single device)")
			if err := device.Open(); err != nil {
				fmt.Printf("Failed to open single device: %v\n", err)
			} else {
				fmt.Println("Successfully opened device with empty serial")
				device.Close()
			}
		}
	} else {
		fmt.Printf("Multiple devices found (%d), GetDevice(\"\") should fail\n", len(devices))
		if _, err := streamdeck.GetDevice(""); err != nil {
			if errors.Is(err, streamdeck.ErrMoreThanOneDeviceFound) {
				fmt.Println("Correctly returned ErrMoreThanOneDeviceFound for empty serial with multiple devices")
			} else {
				fmt.Printf("Error - Unexpected error for empty serial with multiple devices: %v\n", err)
			}
		} else {
			fmt.Println("Error - Should have failed for empty serial with multiple devices")
		}
	}

	fmt.Println("\nTesting error cases:")
	fmt.Println("--------------------")

	if _, err = streamdeck.GetDevice("non-existent-serial"); err != nil {
		if errors.Is(err, streamdeck.ErrNoDeviceFound) {
			fmt.Println("Correctly returned ErrNoDeviceFound for non-existent serial")
		} else {
			fmt.Printf("Error - Unexpected error for non-existent serial: %v\n", err)
		}
	} else {
		fmt.Println("Error - Should have failed for non-existent serial")
	}
}

func testBasicFunctionality(device *streamdeck.Device) error {
	if err := device.SetBrightness(50); err != nil {
		return fmt.Errorf("brightness control failed: %w", err)
	}

	if device.GetKeyCount() > 0 {
		if err := device.SetKeyColor(streamdeck.KEY_1, colornames.Red); err != nil {
			return fmt.Errorf("key color setting failed: %w", err)
		}
		if err := device.ClearKey(streamdeck.KEY_1); err != nil {
			return fmt.Errorf("key clearing failed: %w", err)
		}
	}

	if device.GetTouchPointCount() > 0 {
		if err := device.SetTouchPointColor(streamdeck.TOUCH_POINT_1, colornames.Lime); err != nil {
			return fmt.Errorf("touch point color setting failed: %w", err)
		}
		if err := device.ClearTouchPoint(streamdeck.TOUCH_POINT_1); err != nil {
			return fmt.Errorf("touch point clearing failed: %w", err)
		}
	}

	if device.GetInfoBarSupported() {
		if err := device.SetInfoBarColor(colornames.Blue); err != nil {
			return fmt.Errorf("info bar color setting failed: %w", err)
		}
		if err := device.ClearInfoBar(); err != nil {
			return fmt.Errorf("info bar clearing failed: %w", err)
		}
	}

	if device.GetTouchStripSupported() {
		if err := device.SetTouchStripColor(colornames.Blue); err != nil {
			return fmt.Errorf("touch strip color setting failed: %w", err)
		}
		if err := device.ClearTouchStrip(); err != nil {
			return fmt.Errorf("touch strip clearing failed: %w", err)
		}
	}
	return nil
}
