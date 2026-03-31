// sensors.go — Core 1: lectura de sensores reales
//
// Sensor nivel   : Submersible 4-20mA, 1 metro, 5V
//
//	Shunt 120Ω en GP26 (ADC0)
//
// Sensor temp    : DS18B20 waterproof, 1-Wire
//
//	Agua → GP4 | Exterior → GP5
//
// Batería        : Waveshare UPS Module B — chip MAX17048
//
//	I2C1 en GP6 (SDA) / GP7 (SCL)
//	Dirección I2C fija: 0x36
//	Entrega: % SOC, voltaje mV, estado carga/descarga
//
// Flashear: tinygo flash -target=pico .
package main

import (
	"machine"
	"sync"
	"time"
)

// ─────────────────────────────────────────────────────────────────────────
// Pines
// ─────────────────────────────────────────────────────────────────────────

const (
	PinADCNivel    = machine.GP26 // ADC0   — sensor nivel 4-20mA
	PinDS18B20Agua = machine.GP4  // 1-Wire — DS18B20 temperatura agua
	PinDS18B20Ext  = machine.GP5  // 1-Wire — DS18B20 temperatura exterior
	PinI2C_SDA     = machine.GP6  // I2C1 SDA — MAX17048 UPS-B
	PinI2C_SCL     = machine.GP7  // I2C1 SCL — MAX17048 UPS-B
)

// ─────────────────────────────────────────────────────────────────────────
// MAX17048 — registros I2C
// ─────────────────────────────────────────────────────────────────────────
//
// Dirección: 0x36 (fija en hardware, no configurable)
//
// Registro VCELL (0x02): voltaje de celda
//   2 bytes big-endian, bits 15-4, resolución 1.25mV/unidad
//   Voltaje (mV) = (raw >> 4) * 1250 / 1000
//
// Registro SOC (0x04): state of charge = porcentaje de batería
//   2 bytes big-endian
//   Byte alto: parte entera del %
//   Byte bajo: parte fraccional (1/256 %)
//   Porcentaje = byte_alto (ignoramos fracción para mostrar entero)
//
// Registro CRATE (0x16): tasa de cambio de carga
//   2 bytes big-endian, valor con signo (int16)
//   Positivo → cargando | Negativo → descargando
//   Resolución: 0.208% por hora por unidad

const (
	max17048Addr = 0x36
	regVCELL     = 0x02
	regSOC       = 0x04
	regCRATE     = 0x16
)

// EstadoBateria describe el estado de carga del módulo UPS-B.
type EstadoBateria uint8

const (
	BatDesconocida EstadoBateria = iota
	BatCargando                  // CRATE > 0
	BatDescargando               // CRATE < 0
	BatCompleta                  // SOC >= 99% y CRATE ~ 0
)

// ─────────────────────────────────────────────────────────────────────────
// Calibración sensor de nivel 4-20mA con shunt 120Ω
// ─────────────────────────────────────────────────────────────────────────

const (
	adcNivelVacio = 596  // raw ADC con tanque vacío  (4mA  × 120Ω = 0.48V)
	adcNivelLleno = 2978 // raw ADC con tanque lleno  (20mA × 120Ω = 2.40V)
)

// ─────────────────────────────────────────────────────────────────────────
// Estado compartido entre núcleos
// ─────────────────────────────────────────────────────────────────────────

var sensorMu sync.Mutex

type SensorData struct {
	NivelAgua    int           // 0–100 %
	TempAgua     int           // °C  (-99 = error)
	TempExterior int           // °C  (-99 = error)
	BatPct       int           // 0–100 %
	BatMV        int           // voltaje en mV (ej: 3850)
	BatEstado    EstadoBateria // cargando / descargando / completa
}

var sensorState = SensorData{
	NivelAgua:    50,
	TempAgua:     22,
	TempExterior: 30,
	BatPct:       100,
	BatMV:        4200,
	BatEstado:    BatDesconocida,
}

// ReadSensors devuelve copia segura — llamar desde Core 0.
func ReadSensors() SensorData {
	sensorMu.Lock()
	d := sensorState
	sensorMu.Unlock()
	return d
}

func writeSensors(d SensorData) {
	sensorMu.Lock()
	sensorState = d
	sensorMu.Unlock()
}

// ─────────────────────────────────────────────────────────────────────────
// StartSensorCore — lanzar Core 1
// ─────────────────────────────────────────────────────────────────────────

func StartSensorCore() {
	initSensorPins()
	go sensorLoop()
}

func initSensorPins() {
	// ADC nivel
	machine.InitADC()
	adcNivel := machine.ADC{Pin: PinADCNivel}
	adcNivel.Configure(machine.ADCConfig{})

	// DS18B20
	PinDS18B20Agua.Configure(machine.PinConfig{Mode: machine.PinInputPullup})
	PinDS18B20Ext.Configure(machine.PinConfig{Mode: machine.PinInputPullup})

	// I2C1 para MAX17048
	machine.I2C1.Configure(machine.I2CConfig{
		Frequency: 400_000, // 400 kHz — fast mode, soportado por MAX17048
		SDA:       PinI2C_SDA,
		SCL:       PinI2C_SCL,
	})

	// Dar tiempo al MAX17048 para iniciar (tarda ~500ms tras power-on)
	time.Sleep(600 * time.Millisecond)
}

// ─────────────────────────────────────────────────────────────────────────
// Loop de Core 1
// ─────────────────────────────────────────────────────────────────────────
//
// Tiempos aproximados por iteración:
//   ADC nivel         ~20ms  (16 muestras × 1ms)
//   DS18B20 agua      ~800ms (conversión 12 bits)
//   DS18B20 exterior  ~800ms (conversión 12 bits)
//   MAX17048          ~5ms   (3 lecturas I2C)
//   Total             ~1.6 s por ciclo completo

func sensorLoop() {
	adcNivel := machine.ADC{Pin: PinADCNivel}

	for {
		var d SensorData

		d.NivelAgua = leerNivel4_20mA(adcNivel)
		d.TempAgua = leerDS18B20(PinDS18B20Agua)
		d.TempExterior = leerDS18B20(PinDS18B20Ext)

		pct, mv, estado := leerMAX17048()
		d.BatPct = pct
		d.BatMV = mv
		d.BatEstado = estado

		writeSensors(d)

		time.Sleep(200 * time.Millisecond)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// MAX17048 — lectura I2C
// ─────────────────────────────────────────────────────────────────────────

// i2cReadReg16 lee 2 bytes de un registro del MAX17048 (big-endian).
func i2cReadReg16(reg byte) (uint16, error) {
	// Escribir dirección del registro
	err := machine.I2C1.Tx(max17048Addr, []byte{reg}, nil)
	if err != nil {
		return 0, err
	}
	// Leer 2 bytes
	buf := make([]byte, 2)
	err = machine.I2C1.Tx(max17048Addr, nil, buf)
	if err != nil {
		return 0, err
	}
	return uint16(buf[0])<<8 | uint16(buf[1]), nil
}

// leerMAX17048 devuelve (porcentaje, voltajeMV, estado).
// En caso de error I2C devuelve los últimos valores conocidos o defaults.
func leerMAX17048() (pct int, mv int, estado EstadoBateria) {
	// ── Voltaje (registro VCELL) ──────────────────────────────────────
	// raw bits 15-4, resolución 1.25mV por unidad
	rawV, err := i2cReadReg16(regVCELL)
	if err != nil {
		return sensorState.BatPct, sensorState.BatMV, sensorState.BatEstado
	}
	// Voltaje en mV: (raw >> 4) * 1.25mV = (raw >> 4) * 5 / 4
	mv = int(rawV>>4) * 5 / 4

	// ── SOC — porcentaje (registro SOC) ───────────────────────────────
	// Byte alto = parte entera del %, byte bajo = fracción /256
	rawSOC, err := i2cReadReg16(regSOC)
	if err != nil {
		return sensorState.BatPct, mv, sensorState.BatEstado
	}
	pct = int(rawSOC >> 8) // byte alto = porcentaje entero
	if pct > 100 {
		pct = 100
	}

	// ── CRATE — tasa de cambio (registro CRATE) ───────────────────────
	// int16 con signo: positivo = cargando, negativo = descargando
	// Resolución 0.208%/hora por unidad
	rawCR, err := i2cReadReg16(regCRATE)
	if err != nil {
		estado = BatDesconocida
		return pct, mv, estado
	}
	crate := int16(rawCR)

	switch {
	case pct >= 99 && crate >= -2 && crate <= 2:
		estado = BatCompleta
	case crate > 0:
		estado = BatCargando
	default:
		estado = BatDescargando
	}

	return pct, mv, estado
}

// ─────────────────────────────────────────────────────────────────────────
// Sensor de nivel 4-20mA — ADC
// ─────────────────────────────────────────────────────────────────────────

func leerNivel4_20mA(adc machine.ADC) int {
	var suma uint32
	for i := 0; i < 16; i++ {
		suma += uint32(adc.Get())
		time.Sleep(1 * time.Millisecond)
	}
	raw := int(suma / 16)

	if raw < adcNivelVacio {
		raw = adcNivelVacio
	}
	if raw > adcNivelLleno {
		raw = adcNivelLleno
	}

	return (raw - adcNivelVacio) * 100 / (adcNivelLleno - adcNivelVacio)
}

// ─────────────────────────────────────────────────────────────────────────
// DS18B20 — protocolo 1-Wire
// ─────────────────────────────────────────────────────────────────────────

func owReset(pin machine.Pin) bool {
	pin.Configure(machine.PinConfig{Mode: machine.PinOutput})
	pin.Low()
	time.Sleep(480 * time.Microsecond)
	pin.Configure(machine.PinConfig{Mode: machine.PinInputPullup})
	time.Sleep(70 * time.Microsecond)
	present := !pin.Get()
	time.Sleep(410 * time.Microsecond)
	return present
}

func owWriteBit(pin machine.Pin, bit bool) {
	pin.Configure(machine.PinConfig{Mode: machine.PinOutput})
	pin.Low()
	if bit {
		time.Sleep(1 * time.Microsecond)
		pin.Configure(machine.PinConfig{Mode: machine.PinInputPullup})
		time.Sleep(60 * time.Microsecond)
	} else {
		time.Sleep(60 * time.Microsecond)
		pin.Configure(machine.PinConfig{Mode: machine.PinInputPullup})
		time.Sleep(1 * time.Microsecond)
	}
}

func owReadBit(pin machine.Pin) bool {
	pin.Configure(machine.PinConfig{Mode: machine.PinOutput})
	pin.Low()
	time.Sleep(1 * time.Microsecond)
	pin.Configure(machine.PinConfig{Mode: machine.PinInputPullup})
	time.Sleep(14 * time.Microsecond)
	bit := pin.Get()
	time.Sleep(45 * time.Microsecond)
	return bit
}

func owWriteByte(pin machine.Pin, b byte) {
	for i := 0; i < 8; i++ {
		owWriteBit(pin, b&0x01 != 0)
		b >>= 1
	}
}

func owReadByte(pin machine.Pin) byte {
	var b byte
	for i := 0; i < 8; i++ {
		if owReadBit(pin) {
			b |= 1 << uint(i)
		}
	}
	return b
}

func leerDS18B20(pin machine.Pin) int {
	if !owReset(pin) {
		return -99
	}
	owWriteByte(pin, 0xCC) // SKIP ROM
	owWriteByte(pin, 0x44) // CONVERT T

	pin.Configure(machine.PinConfig{Mode: machine.PinInputPullup})
	deadline := time.Now().Add(1 * time.Second)
	for !pin.Get() {
		if time.Now().After(deadline) {
			return -99
		}
		time.Sleep(10 * time.Millisecond)
	}

	if !owReset(pin) {
		return -99
	}
	owWriteByte(pin, 0xCC) // SKIP ROM
	owWriteByte(pin, 0xBE) // READ SCRATCHPAD

	lsb := owReadByte(pin)
	msb := owReadByte(pin)
	for i := 0; i < 7; i++ {
		owReadByte(pin)
	}

	raw := int16(uint16(msb)<<8 | uint16(lsb))
	tempC := int(raw) / 16

	if tempC < -55 || tempC > 125 {
		return -99
	}
	return tempC
}
