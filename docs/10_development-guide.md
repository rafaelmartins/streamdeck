# Development guide

The streamdeck library provides a callback-based API for interacting with Elgato Stream Deck devices. This guide covers installation, device lifecycle, input handling, display control, and the library's architecture.

For detailed function signatures and return values, see the [API documentation](https://pkg.go.dev/rafaelmartins.com/p/streamdeck).

## Integration

### Installation

```bash
go get rafaelmartins.com/p/streamdeck
```

Requires Go 1.24 or later.

### Dependencies

| Module | Purpose |
|--------|---------|
| `rafaelmartins.com/p/usbhid` | Pure Go USB HID communication |
| `golang.org/x/image` | BMP encoding and image scaling/transformation |

No C libraries or CGO are required.

## API overview

### Device discovery

Two functions handle device discovery:

- `Enumerate()` -- returns a slice of all connected Stream Deck devices
- `GetDevice(serialNumber)` -- returns a single device by serial number, or the only connected device if the serial number is empty

```go
// Get the only connected device (errors if multiple are connected)
device, err := streamdeck.GetDevice("")
if err != nil {
    log.Fatal(err)
}

// Or enumerate all devices
devices, err := streamdeck.Enumerate()
if err != nil {
    log.Fatal(err)
}
for _, dev := range devices {
    log.Printf("Found: %s (serial: %s)", dev.GetModelName(), dev.GetSerialNumber())
}
```

### Device lifecycle

A device must be opened before use and closed when done. `Close()` automatically clears all displays before closing the USB connection.

```go
if err := device.Open(); err != nil {
    log.Fatal(err)
}
defer device.Close()
```

> [!NOTE]
> `Reset()` performs a hardware reset equivalent to power cycling the device. It closes the USB connection and does not attempt to reconnect.

### Device information

After obtaining a device (even before opening), these methods provide information about its capabilities:

| Method | Return type | Description |
|--------|-------------|-------------|
| `GetModelName()` | `string` | Product name reported by USB descriptor |
| `GetModelID()` | `string` | Internal model identifier (`mini`, `mk2`, `plus`, `neo`) |
| `GetSerialNumber()` | `string` | Device serial number |
| `GetKeyCount()` | `byte` | Number of LCD keys |
| `GetTouchPointCount()` | `byte` | Number of touch points (0 if unsupported) |
| `GetDialCount()` | `byte` | Number of rotary dials (0 if unsupported) |
| `GetInfoBarSupported()` | `bool` | Whether the device has an info bar display |
| `GetTouchStripSupported()` | `bool` | Whether the device has a touch strip display |
| `GetFirmwareVersion()` | `string, error` | Firmware version string (requires open device) |

### Input handling

Input handling uses a callback model. Register handlers for specific inputs, then call `Listen()` to start the event loop. `Listen()` blocks until the device is closed or an error occurs.

#### Key handlers

Register a `KeyHandler` for any key from `KEY_1` through `KEY_15` (depending on the device model). The handler receives the `Device` and `Key` instances. Call `WaitForRelease()` inside the handler to block until the key is released and get the press duration.

```go
device.AddKeyHandler(streamdeck.KEY_1, func(d *streamdeck.Device, k *streamdeck.Key) error {
    log.Printf("Key %s pressed", k)
    duration := k.WaitForRelease()
    log.Printf("Key %s released after %s", k, duration)
    return nil
})
```

Multiple handlers can be registered for the same key. Each handler runs in its own goroutine.

#### Touch point handlers

Available on Stream Deck Neo. Register a `TouchPointHandler` for `TOUCH_POINT_1` or `TOUCH_POINT_2`. The API mirrors key handlers, including `WaitForRelease()`.

```go
device.AddTouchPointHandler(streamdeck.TOUCH_POINT_1, func(d *streamdeck.Device, tp *streamdeck.TouchPoint) error {
    log.Printf("Touch point %s activated", tp)
    duration := tp.WaitForRelease()
    log.Printf("Touch point %s released after %s", tp, duration)
    return nil
})
```

#### Dial handlers

Available on Stream Deck Plus. Dials support two types of handlers:

- `DialSwitchHandler` -- called when a dial is pressed (supports `WaitForRelease()`)
- `DialRotateHandler` -- called when a dial is rotated, with an `int8` delta value

```go
device.AddDialSwitchHandler(streamdeck.DIAL_1, func(d *streamdeck.Device, di *streamdeck.Dial) error {
    log.Printf("Dial %s pressed", di)
    duration := di.WaitForRelease()
    log.Printf("Dial %s released after %s", di, duration)
    return nil
})

device.AddDialRotateHandler(streamdeck.DIAL_1, func(d *streamdeck.Device, di *streamdeck.Dial, delta int8) error {
    log.Printf("Dial %s rotated: %d", di, delta)
    return nil
})
```

#### Touch strip handlers

Available on Stream Deck Plus. Touch strips support two types of handlers:

- `TouchStripTouchHandler` -- called on touch events (short or long), receives touch type and `image.Point` coordinates
- `TouchStripSwipeHandler` -- called on swipe events, receives origin and destination `image.Point` coordinates

```go
device.AddTouchStripTouchHandler(func(d *streamdeck.Device, t streamdeck.TouchStripTouchType, p image.Point) error {
    log.Printf("Touch strip touched at %s (type: %s)", p, t)
    return nil
})

device.AddTouchStripSwipeHandler(func(d *streamdeck.Device, origin image.Point, dest image.Point) error {
    log.Printf("Touch strip swiped from %s to %s", origin, dest)
    return nil
})
```

#### Event loop

`Listen()` starts the event loop. It accepts an optional error channel for receiving handler errors. If `nil`, errors are logged to the standard logger.

```go
// Log errors to standard logger
if err := device.Listen(nil); err != nil {
    log.Fatal(err)
}

// Or use an error channel for custom error handling
errCh := make(chan error, 10)
go func() {
    for err := range errCh {
        log.Printf("Handler error: %v", err)
    }
}()
if err := device.Listen(errCh); err != nil {
    log.Fatal(err)
}
```

### Display control

#### Key images

Each key has an LCD display. The library accepts images from multiple sources and automatically scales them to fit. Use `GetKeyImageRectangle()` to obtain the display geometry if generating images at the exact target size is desired.

| Method | Image source |
|--------|-------------|
| `SetKeyImage(key, img)` | `image.Image` |
| `SetKeyImageFromReader(key, r)` | `io.Reader` |
| `SetKeyImageFromReadCloser(key, r)` | `io.ReadCloser` (auto-closed) |
| `SetKeyImageFromFile(key, path)` | File path |
| `SetKeyImageFromFS(key, fsys, path)` | `fs.FS` filesystem |
| `SetKeyColor(key, color)` | Solid `color.Color` |
| `ClearKey(key)` | Clears to black |

The library decodes PNG, JPEG, GIF, and BMP input images. Internally, images are encoded as JPEG (or BMP for Stream Deck Mini) before being sent to the device.

```go
// Set a solid color
device.SetKeyColor(streamdeck.KEY_1, color.RGBA{0, 255, 0, 255})

// Set from an image.Image
device.SetKeyImage(streamdeck.KEY_2, myImage)

// Set from an embedded filesystem
//go:embed assets
var assets embed.FS
device.SetKeyImageFromFS(streamdeck.KEY_3, assets, "assets/icon.png")
```

#### Info bar display

Available on Stream Deck Neo (248x58 pixels). The same set of image source methods applies, prefixed with `SetInfoBar*` instead of `SetKey*`. Use `GetInfoBarImageRectangle()` to obtain the display geometry.

| Method | Image source |
|--------|-------------|
| `SetInfoBarImage(img)` | `image.Image` |
| `SetInfoBarImageFromReader(r)` | `io.Reader` |
| `SetInfoBarImageFromReadCloser(r)` | `io.ReadCloser` (auto-closed) |
| `SetInfoBarImageFromFile(path)` | File path |
| `SetInfoBarImageFromFS(fsys, path)` | `fs.FS` filesystem |
| `SetInfoBarColor(color)` | Solid `color.Color` |
| `ClearInfoBar()` | Clears to black |

#### Touch point colors

Available on Stream Deck Neo. Touch points only support solid colors.

```go
device.SetTouchPointColor(streamdeck.TOUCH_POINT_1, color.RGBA{255, 0, 0, 255})
device.ClearTouchPoint(streamdeck.TOUCH_POINT_2)
```

#### Touch strip display

Available on Stream Deck Plus (800x100 pixels). Supports full-strip images and partial updates via rectangle-constrained variants. Use `GetTouchStripImageRectangle()` to obtain the display geometry.

| Method | Image source |
|--------|-------------|
| `SetTouchStripImage(img)` | Full strip from `image.Image` |
| `SetTouchStripImageWithRectangle(img, rect)` | Partial update within `image.Rectangle` |
| `SetTouchStripColor(color)` | Solid color, full strip |
| `SetTouchStripColorWithRectangle(color, rect)` | Solid color within rectangle |
| `ClearTouchStrip()` | Clears full strip to black |
| `ClearTouchStripWithRectangle(rect)` | Clears rectangle to black |

File, reader, read-closer, and filesystem variants are also available for both full and rectangle-constrained images.

### Brightness and reset

```go
// Set brightness (0-100 percent)
device.SetBrightness(75)

// Reset the device (closes connection)
device.Reset()
```

### Iteration helpers

`ForEachKey`, `ForEachTouchPoint`, and `ForEachDial` iterate over all available inputs on the device, passing the respective ID to a callback. These are useful for applying the same operation to all inputs.

```go
device.ForEachKey(func(k streamdeck.KeyID) error {
    return device.SetKeyColor(k, color.RGBA{0, 0, 255, 255})
})
```

## Architecture

### Source files

| File | Purpose |
|------|---------|
| `device.go` | Device struct, discovery, lifecycle, input dispatching, device info, settings |
| `input.go` | Input types (Key, TouchPoint, Dial, TouchStrip), handler registration, event processing |
| `image.go` | Image encoding, scaling, transformation, display methods for keys/info bar/touch strip |
| `models.go` | Model definitions, per-device protocol details, USB product IDs |

### Execution model

The library uses a synchronous event loop in `Listen()` that reads USB HID input reports. When an input state change is detected, all registered handlers for that input are dispatched in separate goroutines. Press/release tracking uses channels: `WaitForRelease()` blocks on a channel that is signaled when the input transitions from pressed to released.

Handler errors are forwarded to either a user-provided error channel or the standard logger. Error delivery is non-blocking to prevent handler deadlocks.

### Image processing

Images go through a pipeline before reaching the device:

1. **Scaling** -- source images are scaled to fit the target rectangle using bilinear interpolation, maintaining aspect ratio
2. **Transformation** -- model-specific flips and rotations are applied (e.g., Stream Deck Mini requires 90-degree rotation and horizontal flip)
3. **Encoding** -- the final image is encoded as JPEG (quality 100) or BMP depending on the device model
4. **Chunking** -- encoded data is split into USB HID output report-sized chunks with protocol-specific headers

## Examples

The [examples](https://github.com/rafaelmartins/streamdeck/tree/main/examples) directory contains complete working programs:

- **basic** -- simple input handling and image setting
- **advanced** -- info bar, touch points, dials, touch strip, long press detection
- **images** -- embedded files, generated patterns, and programmatic graphics
- **device-info** -- device enumeration and capability detection
- **multi-device** -- working with multiple devices simultaneously

Run any example with:

```bash
go run examples/basic/main.go
```

Pass a serial number as an argument if multiple devices are connected.
