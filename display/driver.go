package display

import (
	"fmt"
	"image"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"sync"

	"periph.io/x/conn/v3/gpio"
	"periph.io/x/conn/v3/gpio/gpioreg"
	"periph.io/x/conn/v3/physic"
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

	// The SSD1680 controller's RAM addresses this panel natively as
	// 176 (X, byte-packed) x 264 (Y, gate lines) — the opposite of the
	// logical Width x Height used everywhere else in this package.
	// imageToBuf() rotates into this layout before writing to RAM.
	nativeWidth  = 176
	nativeHeight = 264
)

// Waveshare27 drives the Waveshare 2.7" e-Paper HAT over SPI.
//
// Pin mapping (BCM numbering, matching the HAT's default wiring):
//
//	RST  = GPIO17  (pin 11)
//	DC   = GPIO25  (pin 22)
//	CS   = GPIO8   (SPI0 CE0, pin 24)
//	BUSY = GPIO24  (pin 18)
//	PWR  = GPIO18  (pin 12)  -- gates the panel's power rail; must be
//	                            driven high before reset/init, or the
//	                            panel never actually powers on even
//	                            though every SPI/GPIO write succeeds.
//	CLK  = GPIO11  (SPI0 SCLK, pin 23)
//	DIN  = GPIO10  (SPI0 MOSI, pin 19)
type Waveshare27 struct {
	mu   sync.Mutex
	spi  spi.PortCloser
	conn spi.Conn
	dc   gpio.PinOut
	rst  gpio.PinOut
	pwr  gpio.PinOut
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

	conn, err := port.Connect(4*physic.MegaHertz, spi.Mode0, 8)
	if err != nil {
		port.Close()
		return nil, fmt.Errorf("spi connect: %w", err)
	}

	dc := gpioreg.ByName("GPIO25")
	rst := gpioreg.ByName("GPIO17")
	pwr := gpioreg.ByName("GPIO18")
	busy := gpioreg.ByName("GPIO24")

	if dc == nil || rst == nil || pwr == nil || busy == nil {
		port.Close()
		return nil, fmt.Errorf("gpio pins not found (dc=%v rst=%v pwr=%v busy=%v)", dc, rst, pwr, busy)
	}

	d := &Waveshare27{
		spi:  port,
		conn: conn,
		dc:   dc,
		rst:  rst,
		pwr:  pwr,
		busy: busy.(gpio.PinIn),
	}

	// Power on the panel before touching reset/init — without this the
	// board silently does nothing: every SPI/GPIO call still succeeds,
	// but the panel itself never receives power.
	d.pwr.Out(gpio.High)
	time.Sleep(20 * time.Millisecond)

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
	time.Sleep(200 * time.Millisecond)
	d.rst.Out(gpio.Low)
	time.Sleep(2 * time.Millisecond)
	d.rst.Out(gpio.High)
	time.Sleep(200 * time.Millisecond)
}

func (d *Waveshare27) sendCommand(cmd byte) {
	d.dc.Out(gpio.Low)
	if err := d.conn.Tx([]byte{cmd}, nil); err != nil {
		log.Printf("display: spi command 0x%02X failed: %v", cmd, err)
	}
}

// spiChunkSize keeps each transfer under the Linux spidev default max
// transfer size (4096 bytes). A full-frame write (5808 bytes) exceeds
// that in one call, and periph.io does not chunk large transfers for
// you — it either fails the whole Tx() or (worse) silently truncates,
// leaving the panel's RAM untouched while the rest of the command
// sequence proceeds as if nothing were wrong.
const spiChunkSize = 4096

func (d *Waveshare27) sendData(data ...byte) {
	d.dc.Out(gpio.High)
	for len(data) > 0 {
		n := spiChunkSize
		if n > len(data) {
			n = len(data)
		}
		if err := d.conn.Tx(data[:n], nil); err != nil {
			log.Printf("display: spi data write failed: %v", err)
			return
		}
		data = data[n:]
	}
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

	// Set RAM Y address start/end position: 0..263 (native height, fixed
	// by the panel's OTP config — do not derive this from Width/Height).
	d.sendCommand(0x45)
	d.sendData(0x00, 0x00, byte((nativeHeight-1)&0xFF), byte(((nativeHeight-1)>>8)&0xFF))

	// Set RAM Y address counter to 0
	d.sendCommand(0x4F)
	d.sendData(0x00, 0x00)

	d.sendCommand(0x11) // data entry mode: X increment, Y increment
	d.sendData(0x03)

	d.waitBusy()
}

func (d *Waveshare27) Show(img image.Image) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	buf := imageToBuf(img)

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
	d.mu.Lock()
	defer d.mu.Unlock()

	buf := make([]byte, nativeWidth/8*nativeHeight)
	for i := range buf {
		buf[i] = 0xFF // all white
	}

	d.sendCommand(0x24)
	d.sendData(buf...)

	d.sendCommand(0x22)
	d.sendData(0xF7)
	d.sendCommand(0x20)
	d.waitBusy()

	return nil
}

// imageToBuf converts a Width x Height (264x176, landscape) image into the
// 1-bit packed buffer the SSD1680 controller expects, rotating it into the
// panel's native 176x264 RAM layout in the process. White pixel = 1, black
// pixel = 0. Each byte packs 8 pixels along the native X axis.
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

	buf := make([]byte, nativeWidth/8*nativeHeight)
	for i := range buf {
		buf[i] = 0xFF // default white
	}

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, _ := img.At(bounds.Min.X+x, bounds.Min.Y+y).RGBA()
			// Simple threshold: if dark, set pixel black
			lum := (r*299 + g*587 + b*114) / 1000
			if lum < 0x8000 {
				nativeX := y
				nativeY := nativeHeight - 1 - x
				byteIdx := (nativeX + nativeY*nativeWidth) / 8
				bitIdx := 7 - uint(nativeX%8)
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
