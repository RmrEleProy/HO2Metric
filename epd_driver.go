// Waveshare Pico-ePaper-3.7 + RP2040 (TinyGo)
// Controlador: SSD1677 — 4 grises (2 bits/px), 280×480 px
// Flashear: tinygo flash -target=pico .
package main

import (
	"machine"
	"time"
)

// ── Dimensiones físicas del panel ─────────────────────────────────────────
const (
	EPD_WIDTH  = 280
	EPD_HEIGHT = 480
)

// ── Paleta de 4 grises (2 bits/px) ───────────────────────────────────────
const (
	ColorBlack     uint8 = 0x00
	ColorDarkGray  uint8 = 0x02
	ColorLightGray uint8 = 0x01
	ColorWhite     uint8 = 0x03
)

// ── Rotación (sentido horario) ────────────────────────────────────────────
// Inspirado en el driver epd4in2 de TinyGo drivers.
type Rotation uint8

const (
	Rotation0   Rotation = iota // 0°   — portrait  280×480
	Rotation90                  // 90°  — landscape 480×280  ← valor por defecto del proyecto
	Rotation180                 // 180° — portrait  280×480 invertido
	Rotation270                 // 270° — landscape 480×280 invertido
)

// ── LUT 4 grises (105 bytes, específica del SSD1677 / Pico-ePaper-3.7) ───
var lut4GrayGC = [105]byte{
	0x2A, 0x06, 0x15, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x28, 0x06, 0x14, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x20, 0x06, 0x10, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x14, 0x06, 0x28, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x02, 0x02, 0x0A, 0x00, 0x00, 0x00, 0x08, 0x08, 0x02,
	0x00, 0x02, 0x02, 0x0A, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x22, 0x22, 0x22, 0x22, 0x22,
}

// ── Estructura del driver ─────────────────────────────────────────────────
type EPD struct {
	spi      *machine.SPI
	rst      machine.Pin
	dc       machine.Pin
	cs       machine.Pin
	busy     machine.Pin
	buf      []byte   // buffer lógico: canvas de dibujo (ancho lógico × alto lógico)
	rotBuf   []byte   // buffer de rotación: siempre 280×480 (físico del panel)
	rotation Rotation // rotación activa
}

// ── Constructor ───────────────────────────────────────────────────────────
// Pines por defecto del Waveshare Pico-ePaper-3.7.
// Para cambiar pines usá newEPDWithPins().
func newEPD() *EPD {
	return newEPDWithPins(
		machine.SPI1,
		machine.GP10, // SCK
		machine.GP11, // SDO (MOSI)
		machine.GP28, // SDI (MISO, no usado en escritura)
		machine.GP9,  // CS
		machine.GP8,  // DC
		machine.GP12, // RST
		machine.GP13, // BUSY
		Rotation90,   // landscape por defecto
	)
}

// newEPDWithPins permite configurar pines y rotación inicial de forma explícita.
// Útil si montás la pantalla en otra orientación o usás otro pinout.
func newEPDWithPins(
	spiPort *machine.SPI,
	sckPin, sdoPin, sdiPin machine.Pin,
	csPin, dcPin, rstPin, busyPin machine.Pin,
	rot Rotation,
) *EPD {
	e := &EPD{
		rst:      rstPin,
		dc:       dcPin,
		cs:       csPin,
		busy:     busyPin,
		rotation: rot,
		// rotBuf es siempre el tamaño físico del panel (280×480, 2 bits/px)
		rotBuf: make([]byte, EPD_WIDTH*EPD_HEIGHT/4),
	}

	// El buffer lógico depende de la rotación:
	// landscape (90/270) → canvas 480×280; portrait (0/180) → canvas 280×480
	lw, lh := e.logicalSize()
	e.buf = make([]byte, lw*lh/4)

	e.rst.Configure(machine.PinConfig{Mode: machine.PinOutput})
	e.dc.Configure(machine.PinConfig{Mode: machine.PinOutput})
	e.cs.Configure(machine.PinConfig{Mode: machine.PinOutput})
	e.busy.Configure(machine.PinConfig{Mode: machine.PinInput})

	// Estado inicial de pines (CS/DC/RST en alto)
	e.cs.High()
	e.dc.High()
	e.rst.High()

	e.spi = spiPort
	e.spi.Configure(machine.SPIConfig{
		Frequency: 4_000_000,
		SCK:       sckPin,
		SDO:       sdoPin,
		SDI:       sdiPin,
		Mode:      0,
	})

	time.Sleep(500 * time.Millisecond)
	e.init4Gray()
	e.clearHW()
	time.Sleep(250 * time.Millisecond)
	return e
}

// ── Cambiar rotación en caliente ──────────────────────────────────────────
// Después de llamar a SetRotation hay que redibujar el buffer completo
// porque el tamaño lógico puede haber cambiado.
func (e *EPD) SetRotation(rot Rotation) {
	e.rotation = rot
	lw, lh := e.logicalSize()
	needed := lw * lh / 4
	if len(e.buf) != needed {
		e.buf = make([]byte, needed)
	}
}

// GetRotation devuelve la rotación activa.
func (e *EPD) GetRotation() Rotation { return e.rotation }

// logicalSize devuelve el ancho y alto del canvas de dibujo según la rotación.
// En landscape el canvas es 480×280; en portrait es 280×480.
func (e *EPD) logicalSize() (w, h int) {
	switch e.rotation {
	case Rotation90, Rotation270:
		return EPD_HEIGHT, EPD_WIDTH // 480×280
	default:
		return EPD_WIDTH, EPD_HEIGHT // 280×480
	}
}

// ── Comunicación SPI ──────────────────────────────────────────────────────
func (e *EPD) cmd(b byte) { e.dc.Low(); e.cs.Low(); e.spi.Transfer(b); e.cs.High() }
func (e *EPD) dat(b byte) { e.dc.High(); e.cs.Low(); e.spi.Transfer(b); e.cs.High() }

// waitIdle espera a que el panel termine de procesar.
// Timeout de ~5 s (500 × 10 ms) para evitar bloqueos permanentes.
func (e *EPD) waitIdle() {
	time.Sleep(10 * time.Millisecond)
	for i := 0; i < 500 && e.busy.Get(); i++ {
		time.Sleep(10 * time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
}

// ── Secuencia de reset hardware ───────────────────────────────────────────
func (e *EPD) reset() {
	e.rst.High()
	time.Sleep(20 * time.Millisecond)
	e.rst.Low()
	time.Sleep(200 * time.Microsecond)
	e.rst.High()
	time.Sleep(20 * time.Millisecond)
}

// ── LUT de 4 grises ───────────────────────────────────────────────────────
func (e *EPD) loadLUT() {
	e.cmd(0x32)
	for i := 0; i < 105; i++ {
		e.dat(lut4GrayGC[i])
	}
}

// ── Inicialización del panel en modo 4 grises ─────────────────────────────
func (e *EPD) init4Gray() {
	e.reset()
	e.cmd(0x12) // SWRESET
	time.Sleep(300 * time.Millisecond)
	e.cmd(0x46)
	e.dat(0xF7)
	e.waitIdle() // Auto Write Red RAM
	e.cmd(0x47)
	e.dat(0xF7)
	e.waitIdle() // Auto Write B/W RAM
	e.cmd(0x01)
	e.dat(0xDF)
	e.dat(0x01)
	e.dat(0x00) // Driver Output Control
	e.cmd(0x03)
	e.dat(0x00) // Gate Driving Voltage
	e.cmd(0x04)
	e.dat(0x41)
	e.dat(0xA8)
	e.dat(0x32) // Source Driving Voltage
	e.cmd(0x11)
	e.dat(0x03) // Data Entry Mode
	e.cmd(0x3C)
	e.dat(0x03) // Border Waveform
	e.cmd(0x0C)
	e.dat(0xAE)
	e.dat(0xC7)
	e.dat(0xC3)
	e.dat(0xC0)
	e.dat(0xC0) // Booster
	e.cmd(0x18)
	e.dat(0x80) // Temperature Sensor
	e.cmd(0x2C)
	e.dat(0x44) // Write VCOM
	e.cmd(0x37)
	for i := 0; i < 10; i++ {
		e.dat(0x00)
	}
	e.cmd(0x44)
	e.dat(0x00)
	e.dat(0x00)
	e.dat(0x17)
	e.dat(0x01) // RAM X range
	e.cmd(0x45)
	e.dat(0x00)
	e.dat(0x00)
	e.dat(0xDF)
	e.dat(0x01) // RAM Y range
	e.cmd(0x22)
	e.dat(0xCF)
}

// ── Limpieza hardware (borra el panel a blanco) ───────────────────────────
func (e *EPD) clearHW() {
	wide := EPD_WIDTH / 8
	e.cmd(0x49)
	e.dat(0x00)
	e.cmd(0x4E)
	e.dat(0x00)
	e.dat(0x00)
	e.cmd(0x4F)
	e.dat(0x00)
	e.dat(0x00)
	e.cmd(0x24)
	for j := 0; j < EPD_HEIGHT; j++ {
		for i := 0; i < wide; i++ {
			e.dat(0xFF)
		}
	}
	e.cmd(0x4E)
	e.dat(0x00)
	e.dat(0x00)
	e.cmd(0x4F)
	e.dat(0x00)
	e.dat(0x00)
	e.cmd(0x26)
	for j := 0; j < EPD_HEIGHT; j++ {
		for i := 0; i < wide; i++ {
			e.dat(0xFF)
		}
	}
	e.loadLUT()
	e.cmd(0x22)
	e.dat(0xC7)
	e.cmd(0x20)
	e.waitIdle()
}

// ── ClearBuffer: borra solo el buffer en RAM (sin tocar el panel) ─────────
// Útil para preparar un nuevo frame sin hacer un full refresh todavía.
func (e *EPD) ClearBuffer() {
	// White en 2 bits/px = 0x03 por píxel → 4 píxeles por byte = 0xFF
	for i := range e.buf {
		e.buf[i] = 0xFF
	}
}

// ── Envío del buffer físico al panel (4 grises) ───────────────────────────
// image debe ser el buffer físico 280×480 en formato 2 bits/px.
func (e *EPD) display(image []byte) {
	bufSize := EPD_WIDTH * EPD_HEIGHT / 8
	e.cmd(0x49)
	e.dat(0x00)
	e.cmd(0x4E)
	e.dat(0x00)
	e.dat(0x00)
	e.cmd(0x4F)
	e.dat(0x00)
	e.dat(0x00)
	e.cmd(0x24)
	for i := 0; i < bufSize; i++ {
		var t3 byte
		for j := 0; j < 2; j++ {
			t1 := image[i*2+j]
			for k := 0; k < 2; k++ {
				t2 := t1 & 0x03
				if t2 == 0x03 || t2 == 0x02 {
					t3 |= 0x01
				}
				t3 <<= 1
				t1 >>= 2
				t2 = t1 & 0x03
				if t2 == 0x03 || t2 == 0x02 {
					t3 |= 0x01
				}
				if j != 1 || k != 1 {
					t3 <<= 1
				}
				t1 >>= 2
			}
		}
		e.dat(t3)
	}
	e.cmd(0x4E)
	e.dat(0x00)
	e.dat(0x00)
	e.cmd(0x4F)
	e.dat(0x00)
	e.dat(0x00)
	e.cmd(0x26)
	for i := 0; i < bufSize; i++ {
		var t3 byte
		for j := 0; j < 2; j++ {
			t1 := image[i*2+j]
			for k := 0; k < 2; k++ {
				t2 := t1 & 0x03
				if t2 == 0x03 || t2 == 0x01 {
					t3 |= 0x01
				}
				t3 <<= 1
				t1 >>= 2
				t2 = t1 & 0x03
				if t2 == 0x03 || t2 == 0x01 {
					t3 |= 0x01
				}
				if j != 1 || k != 1 {
					t3 <<= 1
				}
				t1 >>= 2
			}
		}
		e.dat(t3)
	}
	e.loadLUT()
	e.cmd(0x22)
	e.dat(0xC7)
	e.cmd(0x20)
	e.waitIdle()
}

// ── show: aplica la rotación y envía al panel ─────────────────────────────
// Transforma e.buf (canvas lógico) → e.rotBuf (físico 280×480) según rotation,
// luego llama a display().
func (e *EPD) show() {
	for i := range e.rotBuf {
		e.rotBuf[i] = 0
	}

	lw, lh := e.logicalSize()

	switch e.rotation {
	case Rotation0:
		// Sin rotación: el canvas ya es 280×480 = igual al físico.
		copy(e.rotBuf, e.buf)

	case Rotation90:
		// 90° CW: canvas 480×280 → físico 280×480
		// Mapeado: px(lx, ly) en canvas → físico(EPD_WIDTH-1-ly, lx)
		for ly := 0; ly < lh; ly++ {
			for lx := 0; lx < lw; lx++ {
				px := e.getpx(e.buf, lw, lx, ly)
				e.setpx(e.rotBuf, EPD_WIDTH, EPD_WIDTH-1-ly, lx, px)
			}
		}

	case Rotation180:
		// 180°: canvas 280×480, física invertida.
		for ly := 0; ly < lh; ly++ {
			for lx := 0; lx < lw; lx++ {
				px := e.getpx(e.buf, lw, lx, ly)
				e.setpx(e.rotBuf, EPD_WIDTH, EPD_WIDTH-1-lx, EPD_HEIGHT-1-ly, px)
			}
		}

	case Rotation270:
		// 270° CW (= 90° CCW): canvas 480×280 → físico 280×480
		for ly := 0; ly < lh; ly++ {
			for lx := 0; lx < lw; lx++ {
				px := e.getpx(e.buf, lw, lx, ly)
				e.setpx(e.rotBuf, EPD_WIDTH, ly, EPD_HEIGHT-1-lx, px)
			}
		}
	}

	e.display(e.rotBuf)
}

// ── SleepClean: apagado seguro del panel ───
// limpia el panel a blanco antes de entrar en deep sleep.
// Secuencia completa: apaga voltajes → espera idle → modo deep sleep.
// --Para despertar el panel hay que hacer un reset hardware completo (newEPD).
func (e *EPD) SleepClean() {
	e.ClearBuffer()                    // pone e.buf todo en blanco (0xFF)
	e.show()                           // envía el blanco al panel con rotación correcta
	time.Sleep(500 * time.Millisecond) // deja que el refresh termine
	// Apagar voltaje VCOM
	e.cmd(0x02) // Power OFF
	e.waitIdle()
	// Entrar en deep sleep (0xA5 = checksum de seguridad del SSD1677)
	e.cmd(0x10)
	e.dat(0x03)
}
