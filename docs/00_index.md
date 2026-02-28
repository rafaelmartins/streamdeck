---
menu: Main
---
**A pure Go library for interacting with Elgato Stream Deck devices on Linux, macOS, and Windows.**

## Overview

streamdeck is a Go library that provides direct access to Elgato Stream Deck hardware without requiring vendor-provided software. It communicates with devices over USB HID using the [usbhid](https://pkg.go.dev/rafaelmartins.com/p/usbhid) library, with no dependency on libusb or hidapi.

The library supports multiple Stream Deck models, each with different capabilities including LCD keys, touch points, rotary dials, info bar displays, and touch strips. It handles device discovery, input event callbacks, image display with automatic scaling, and device management operations like brightness control and reset.

The API design is inspired by the client library for the [octokeyz](@@/p/octokeyz/) open hardware macropad. Being pure Go makes cross-compilation straightforward for restricted environments, such as the MiSTer FPGA Linux-based operating system.

> [!CAUTION]
> **Disclaimer:** This library is not supported or endorsed by Elgato, Corsair, or any related company.

## Key highlights

- **Pure Go** -- no libusb or hidapi dependency, simplifying cross-compilation
- **Cross-platform** -- works on Linux, macOS, and Windows
- **Multiple device support** -- handles Stream Deck Mini, V2, MK.2, Plus, and Neo
- **Callback-based input** -- register handlers for keys, touch points, dials, and touch strips
- **Automatic image scaling** -- accepts any `image.Image` and scales it to fit the target display
- **Standard library interfaces** -- image sources can be `io.Reader`, `io.ReadCloser`, `fs.FS`, or file paths
- **BSD 3-Clause license** -- permissive open source licensing

## Supported devices

| Model | Product ID | Keys | Touch Points | Dials | Info Bar | Touch Strip |
|-------|------------|------|--------------|-------|----------|-------------|
| Stream Deck Mini | `0x0063` | 6 | -- | -- | -- | -- |
| Stream Deck V2 | `0x006d` | 15 | -- | -- | -- | -- |
| Stream Deck MK.2 | `0x0080` | 15 | -- | -- | -- | -- |
| Stream Deck Plus | `0x0084` | 8 | -- | 4 | -- | 800x100 px |
| Stream Deck Neo | `0x009a` | 8 | 2 | -- | 248x58 px | -- |

Stream Deck V2 (`0x006d`) is treated as an alias for MK.2 (`0x0080`) and uses the same protocol.

Supporting additional models requires hardware access for the library maintainer. If you have the means to help, please [contact the maintainer](https://rafaelmartins.com/).

## Usage

```go
package main

import (
	"image/color"
	"log"

	"rafaelmartins.com/p/streamdeck"
)

func main() {
	device, err := streamdeck.GetDevice("")
	if err != nil {
		log.Fatal(err)
	}

	if err := device.Open(); err != nil {
		log.Fatal(err)
	}
	defer device.Close()

	red := color.RGBA{255, 0, 0, 255}
	if err := device.SetKeyColor(streamdeck.KEY_1, red); err != nil {
		log.Print(err)
		return
	}

	if err := device.AddKeyHandler(streamdeck.KEY_1, func(d *streamdeck.Device, k *streamdeck.Key) error {
		log.Printf("Key %s pressed!", k)
		duration := k.WaitForRelease()
		log.Printf("Key %s released after %s", k, duration)
		return nil
	}); err != nil {
		log.Print(err)
		return
	}

	if err := device.Listen(nil); err != nil {
		log.Print(err)
	}
}
```

## Explore further

- [Development guide](10_development-guide.md) -- integration, API overview, and architecture
- [API documentation](https://pkg.go.dev/rafaelmartins.com/p/streamdeck) -- complete API reference on pkg.go.dev
- [Source code](https://github.com/rafaelmartins/streamdeck) -- GitHub repository
- [usbhid](https://pkg.go.dev/rafaelmartins.com/p/usbhid) -- underlying pure Go USB HID library
- [octokeyz](@@/p/octokeyz/) -- related open hardware macropad project
