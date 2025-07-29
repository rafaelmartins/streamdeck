# Stream Deck Go Library

A pure Go library for interacting with Elgato Stream Deck devices. This library works on Linux, macOS, and Windows.

> [!IMPORTANT]
> **Disclaimer**: This library is **NOT** supported or endorsed by Elgato, Corsair, or any related company. Use it at your own risk!


## Features

- **Cross-platform support** - Works on Linux, macOS, and Windows
- **Pure Go implementation** - No libusb/hidapi dependency
- **Multiple device support** - Supports various Stream Deck models
- **Input event handling** - Register callbacks for input events
- **Image display** - Set custom images on keys with automatic scaling
- **Info bar support** - Control the info bar display on supported models
- **Touch point control** - Set colors for touch points on supported models
- **Device management** - Control brightness, reset, and get device information


## Supported Devices

| Model | Product ID | Keys | Touch Points | Dials | Info Bar | Touch Strip |
|-------|------------|------|--------------|-------|----------|-------------|
| Stream Deck Mini | 0x0063 | 6 | ❌ | ❌ | ❌ | ❌ |
| Stream Deck V2 | 0x006d | 15 | ❌ | ❌ | ❌ | ❌ |
| Stream Deck MK.2 | 0x0080 | 15 | ❌ | ❌ | ❌ | ❌ |
| Stream Deck Neo | 0x009a | 8 | 2 | ❌ | ✅ | ❌ |

### Upcoming Device Support

The following devices are not yet supported, but there's ongoing work to support them:

| Model | Product ID | Keys | Touch Points | Dials | Info Bar | Touch Strip |
|-------|------------|------|--------------|-------|----------|-------------|
| Stream Deck Plus | 0x0084 | 8 | ❌ | 4 | ❌ | ✅ |

Supporting additional models would require hardware access for the library maintainer. Adding support based purely on data reverse-engineered from other libraries is not an option. If you have the means to support adding more devices, please [contact the maintainer](https://rafaelmartins.com/) for additional information.


## Motivation

This work started as an internal library for the [mister-macropads](https://rafaelmartins.com/p/mister-macropads) project, which was never published there. The API worked out so nicely that I decided to make it a standalone public library.

The design is heavily inspired by the client library for my open hardware macropad [octokeyz](https://rafaelmartins.com/p/octokeyz) ([rafaelmartins.com/p/octokeyz](https://pkg.go.dev/rafaelmartins.com/p/octokeyz)), and it relies on my pure Go USB HID library ([rafaelmartins.com/p/usbhid](https://pkg.go.dev/rafaelmartins.com/p/usbhid)).

Being pure Go makes it easier to cross-compile for restricted environments, like the MiSTer FPGA Linux-based operating system. The library also strives to implement and consume the interfaces defined by Go standard libraries for improved compatibility.

This library does not have any tooling to generate images; users are encouraged to generate any `image.Image` and the library will make it fit the desired viewport. There are `Get*ImageRectangle()` functions to recover the geometry of the viewport if users want to generate images the right size and avoid scaling.


## Installation

```bash
go get rafaelmartins.com/p/streamdeck
```


## Quick Start

```go
package main

import (
	"image/color"
	"log"

	"rafaelmartins.com/p/streamdeck"
)

func main() {
	// get the first available stream deck device
	devices, err := streamdeck.Enumerate()
	if err != nil {
		log.Fatal(err)
	}
	if len(devices) == 0 {
		log.Fatal("No Stream Deck devices found")
	}
	device := devices[0]

	// open the device
	if err := device.Open(); err != nil {
		log.Fatal(err)
	}
	defer device.Close()

	// set key 1 to red
	red := color.RGBA{255, 0, 0, 255}
	if err := device.SetKeyColor(streamdeck.KEY_1, red); err != nil {
		log.Print(err)
		return
	}

	// add a key handler
	if err := device.AddKeyHandler(streamdeck.KEY_1, func(d *streamdeck.Device, k *streamdeck.Key) error {
		log.Printf("Key %s pressed!", k)
		duration := k.WaitForRelease()
		log.Printf("Key %s released! %s", k, duration)
		return nil
	}); err != nil {
		log.Print(err)
		return
	}

	// listen for input events
	if err := device.Listen(nil); err != nil {
		log.Print(err)
		return
	}
}
```


## Usage Examples

### Setting Key Images

```go
// from a file
err := device.SetKeyImageFromFile(streamdeck.KEY_1, "icon.png")

// from an embedded filesystem
//go:embed icons
var iconFS embed.FS
err := device.SetKeyImageFromFS(streamdeck.KEY_1, iconFS, "play.png")

// from a solid color
blue := color.RGBA{0, 0, 255, 255}
err := device.SetKeyColor(streamdeck.KEY_1, blue)

// clear a key (set to black)
err := device.ClearKey(streamdeck.KEY_1)
```

### Input Event Handling

```go
// simple key handler
device.AddKeyHandler(streamdeck.KEY_1, func(d *streamdeck.Device, k *streamdeck.Key) error {
	log.Printf("Key %s pressed", k)
	return nil
})

// handler with press duration
device.AddKeyHandler(streamdeck.KEY_2, func(d *streamdeck.Device, k *streamdeck.Key) error {
	duration := k.WaitForRelease()
	log.Printf("Key %s held for %v", k, duration)
	return nil
})

// touch point handler (if supported)
device.AddTouchPointHandler(streamdeck.TOUCH_POINT_1, func(d *streamdeck.Device, tp *streamdeck.TouchPoint) error {
	log.Printf("Touch point %s pressed", tp)
	duration := tp.WaitForRelease()
	log.Printf("Touch point %s held for %v", tp, duration)
	return nil
})
```

### Info Bar Control (if supported)

```go
// check if info bar is supported
if device.GetInfoBarSupported() {
	// set info bar from file
	err := device.SetInfoBarImageFromFile("info.png")

	// or set solid color
	err := device.SetInfoBarColor(color.RGBA{255, 0, 0, 255})

	// or clear info bar
	err := device.ClearInfoBar()
}
```

### Touch Point Control (if supported)

```go
// set touch point colors
red := color.RGBA{255, 0, 0, 255}
green := color.RGBA{0, 255, 0, 255}

device.SetTouchPointColor(streamdeck.TOUCH_POINT_1, red)
device.SetTouchPointColor(streamdeck.TOUCH_POINT_2, green)

// clear touch point (set to black)
device.ClearTouchPoint(streamdeck.TOUCH_POINT_1)
```

### Device Information

```go
// get device details
modelID := device.GetModelID()
modelName := device.GetModelName()
serialNumber := device.GetSerialNumber()
keyCount := device.GetKeyCount()
touchPointCount := device.GetTouchPointCount()

fmt.Printf("Model ID: %s\n", modelID)
fmt.Printf("Model Name: %s\n", modelName)
fmt.Printf("Serial: %s\n", serialNumber)
fmt.Printf("Keys: %d\n", keyCount)
fmt.Printf("Touch Points: %d\n", touchPointCount)

// get firmware version (requires open device)
if device.IsOpen() {
	firmwareVersion, err := device.GetFirmwareVersion()
	if err == nil {
		fmt.Printf("Firmware: %s\n", firmwareVersion)
	}
}
```

### Brightness Control

```go
// set brightness to 50%
err := device.SetBrightness(50)
```

### Working with Multiple Keys

```go
// iterate over all keys
err := device.ForEachKey(func(key streamdeck.KeyID) error {
	// set each key to a different color
	hue := float64(key-streamdeck.KEY_1) * 60.0 // different hues
	color := hsvaToRGBA(hue, 1.0, 1.0, 1.0)
	return device.SetKeyColor(key, color)
})

// iterate over all touch points
err := device.ForEachTouchPoint(func(tp streamdeck.TouchPointID) error {
	return device.SetTouchPointColor(tp, color.RGBA{255, 255, 255, 255})
})
```


## API Reference

Please check [pkg.go.dev/rafaelmartins.com/p/streamdeck](https://pkg.go.dev/rafaelmartins.com/p/streamdeck) for complete API documentation.


## Examples

See the [examples](examples/) directory for complete working examples:

- **[Basic Usage](examples/basic/main.go)** - Simple input handling and image setting with complete error handling
- **[Advanced Features](examples/advanced/main.go)** - Info bar, touch points, long press detection, and dynamic effects
- **[Image Examples](examples/images/main.go)** - Different ways to set images including embedded files, patterns, and generated graphics
- **[Device Information](examples/device-info/main.go)** - Device enumeration, capability detection, and information retrieval
- **[Multi-Device](examples/multi-device/main.go)** - Working with multiple Stream Deck devices simultaneously with synchronized effects

### Running Examples

Each example is a complete, standalone program. To run them:

```bash
# Basic usage example (pass a serial number as argument if multiple devices connected)
go run examples/basic/main.go

# Advanced features (pass a serial number as argument if multiple devices connected. requires a device with info bar/touch points for full demo)
go run examples/advanced/main.go

# Image examples with patterns and embedded icons (pass a serial number as argument if multiple devices connected)
go run examples/images/main.go

# Device information and enumeration
go run examples/device-info/main.go

# Multi-device synchronization (requires multiple devices for full demo)
go run examples/multi-device/main.go
```


## License

This library is distributed under a BSD 3-Clause license.
