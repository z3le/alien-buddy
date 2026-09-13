package display

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/spi"
	"periph.io/x/conn/v3/spi/spireg"
	"periph.io/x/host/v3"
)

// Display is the interface everything talks to.
type Display interface {
	Show(img image.Image) error
	Clear() error
}

// ============================================
// Waveshare 2.7" e-Paper (264×176) SPI driver
// ============================================

const (
	Width  = 264
	Height = 176
)

// Waveshare27 drives the Waveshare 2.7" e-Paper HAT over SPI.
//
// Pin mapping (BCM numbering, matching the HAT's default wiring):
//
//	RST  = GPIO17  (pin 11)
//	DC   = GPIO25  (pin 22)
//	CS   = GPIO8   (SPI0 CE0, pin 24)
//	BUSY = GPIO24  (pin 18)
//	CLK  = GPIO11  (SPI0 SCLK, pin 23)
//	DIN  = GPIO10  (SPI0 MOSI, pin 19)
type Waveshare27 struct {
	spi  spi.PortCloser
	conn spi.Conn
	dc   gpio.PinOut
	rst  gpio.PinOut
	busy gpio.PinIn
}

func NewWaveshare27() (*Waveshare27, error) {
	if _, err := host.Init(); err != nil {
		return nil, fmt.Errorf("periph init: %w", err)
	}

	port, err := spireg.Open("SPI0.0")
	if err != nil {
		return nil, fmt.Errorf("spi open: %w", err)
	}

	conn, err := port.Connect(4_000_000, spi.Mode0, 8)
	if err != nil {
		port.Close()
		return nil, fmt.Errorf("spi connect: %w", err)
	}

	dc := gpioreg.ByName("GPIO25")
	rst := gpioreg.ByName("GPIO17")
	busy := gpioreg.ByName("GPIO24")

	if dc == nil || rst == nil || busy == nil {
		port.Close()
		return nil, fmt.Errorf("gpio pins not found (dc=%v rst=%v busy=%v)", dc, rst, busy)
	}

	d := &Waveshare27{
		spi:  port,
		conn: conn,
		dc:   dc,
		rst:  rst,
		busy: busy.(gpio.PinIn),
	}

	d.reset()
	d.init()
	return d, nil
}

func (d *Waveshare27) Close() error {
	d.Clear()
	return d.spi.Close()
}

func (d *Waveshare27) reset() {
	d.rst.Out(gpio.High)
	time.Sleep(20 * time.Millisecond)
	d.rst.Out(gpio.Low)
	time.Sleep(2 * time.Millisecond)
	d.rst.Out(gpio.High)
	time.Sleep(20 * time.Millisecond)
}

func (d *Waveshare27) sendCommand(cmd byte) {
	d.dc.Out(gpio.Low)
	d.conn.Tx([]byte{cmd}, nil)
}

func (d *Waveshare27) sendData(data ...byte) {
	d.dc.Out(gpio.High)
	d.conn.Tx(data, nil)
}

func (d *Waveshare27) waitBusy() {
	for d.busy.Read() == gpio.High {
		time.Sleep(10 * time.Millisecond)
	}
}

func (d *Waveshare27) init() {
	d.waitBusy()

	d.sendCommand(0x12) // software reset
	d.waitBusy()

	d.sendCommand(0x01) // driver output control
	d.sendData(
		byte((Height-1)&0xFF),
		byte(((Height-1)>>8)&0xFF),
		0x00,
	)

	d.sendCommand(0x11) // data entry mode: X increment, Y increment
	d.sendData(0x03)

	// Set RAM X address range
	d.sendCommand(0x44)
	d.sendData(0x00, byte(Width/8-1))

	// Set RAM Y address range
	d.sendCommand(0x45)
	d.sendData(0x00, 0x00, byte((Height-1)&0xFF), byte(((Height-1)>>8)&0xFF))

	d.sendCommand(0x3C) // border waveform
	d.sendData(0x05)

	// Set RAM X counter
	d.sendCommand(0x4E)
	d.sendData(0x00)

	// Set RAM Y counter
	d.sendCommand(0x4F)
	d.sendData(0x00, 0x00)

	d.waitBusy()
}

func (d *Waveshare27) Show(img image.Image) error {
	buf := imageToBuf(img)

	// Set cursor to 0,0
	d.sendCommand(0x4E)
	d.sendData(0x00)
	d.sendCommand(0x4F)
	d.sendData(0x00, 0x00)

	// Write image data
	d.sendCommand(0x24)
	d.sendData(buf...)

	// Trigger display refresh
	d.sendCommand(0x22)
	d.sendData(0xF7)
	d.sendCommand(0x20)
	d.waitBusy()

	return nil
}

func (d *Waveshare27) Clear() error {
	buf := make([]byte, Width/8*Height)
	for i := range buf {
		buf[i] = 0xFF // all white
	}

	d.sendCommand(0x4E)
	d.sendData(0x00)
	d.sendCommand(0x4F)
	d.sendData(0x00, 0x00)

	d.sendCommand(0x24)
	d.sendData(buf...)

	d.sendCommand(0x22)
	d.sendData(0xF7)
	d.sendCommand(0x20)
	d.waitBusy()

	return nil
}

// imageToBuf converts an image to 1-bit packed bytes for the e-ink display.
// White pixel = 1, Black pixel = 0. Each byte = 8 horizontal pixels.
func imageToBuf(img image.Image) []byte {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	// Clamp to display size
	if w > Width {
		w = Width
	}
	if h > Height {
		h = Height
	}

	buf := make([]byte, Width/8*Height)
	for i := range buf {
		buf[i] = 0xFF // default white
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			// Simple threshold: if dark, set pixel black
			lum := (r*299 + g*587 + b*114) / 1000
			if lum < 0x8000 {
				byteIdx := y*(Width/8) + x/8
				bitIdx := 7 - uint(x%8)
				buf[byteIdx] &^= 1 << bitIdx
			}
		}
	}
	return buf
}

// ============================================
// Fake display — saves PNGs for development
// ============================================

type Fake struct {
	dir   string
	count atomic.Int64
}

func NewFake(dir string) *Fake {
	os.MkdirAll(dir, 0o755)
	return &Fake{dir: dir}
}

func (f *Fake) Show(img image.Image) error {
	n := f.count.Add(1)
	path := filepath.Join(f.dir, fmt.Sprintf("frame_%04d.png", n))

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		return err
	}
	fmt.Printf("[fake display] saved %s\n", path)
	return nil
}

func (f *Fake) Clear() error {
	fmt.Println("[fake display] cleared")
	return nil
}
