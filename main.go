// Code generated for Waveshare Pico-ePaper-3.7 + RP2040 (TinyGo)
// Flashear: tinygo flash -target=pico .
// HO2Metric — Monitor de tanque de agua con relés y botones
package main

import (
	"fmt"
	"machine"
	"strconv"
	"time"
)

// ── Pines ─────────────────────────────────────────────────────────────────
const (
	PinRele1    = machine.GP16
	PinRele2    = machine.GP17
	PinBtnModo  = machine.GP18 // Pulsador AUTO/MANUAL
	PinBtnBomba = machine.GP19 // Pulsador activacion de la bomba (solo en MANUAL)

	ReleActivoHigh = false
)

// ── Umbrales ──────────────────────────────────────────────────────────────
const (
	UmbralEncendido = 20 // % — AUTO: enciende bomba por debajo de este valor
	UmbralApagado   = 90 // % — ambos modos: apaga y bloquea al llegar aquí
)

// ── Estado de la aplicación ───────────────────────────────────────────────
var (
	TempAgua     = 22
	TempExterior = 30
	BatPct       = 23
	BatMV        = 4200
	BatEstadoAct = BatDesconocida
	NivelAgua    = 50
	CapacidadMax = 1000
	ModoAuto     = true
	BombaOn      = false
)

// ── Helpers ───────────────────────────────────────────────────────────────
func itoa(n int) string { return strconv.Itoa(n) }

func releOn(pin machine.Pin) {
	if ReleActivoHigh {
		pin.High()
	} else {
		pin.Low()
	}
}
func releOff(pin machine.Pin) {
	if ReleActivoHigh {
		pin.Low()
	} else {
		pin.High()
	}
}

// ── Dibujo: pantalla completa ─────────────────────────────────────────────
func drawPantalla(epd *EPD) {
	epd.fillScreen(ColorWhite)

	drawHeader(epd)
	// panel izquierdo con datos principales
	drawDatos(epd, 2)

	// Separadores verticales entre columnas
	epd.vline(260, 35, 270, ColorLightGray)
	epd.vline(292, 35, 270, ColorLightGray)

	// Tanque derecha
	drawTanque(epd, 300, 36, NivelAgua)

	// Logo
	epd.drawImage(345, 222, RLC, 100, 47)

}

// ── Configura la interrupción de wake-up en el botón de bomba :─
// Llamar una vez en main() después de configurar el pin.
// FallingEdge = flanco de bajada (pull-up → presión → GND)
func configurarWakeup(btn machine.Pin) {
	btn.SetInterrupt(machine.PinFalling, func(p machine.Pin) {
		// No hacer nada pesado aquí, solo señalizar
		select {
		case wakeupChan <- struct{}{}:
		default: // ya había una señal pendiente, ignorar
		}
	})
}

// ── Main ──────────────────────────────────────────────────────────────────
func main() {
	// LED de estado
	led := machine.LED
	led.Configure(machine.PinConfig{Mode: machine.PinOutput})
	led.High()

	// Relés — apagados al inicio
	rele1 := PinRele1
	rele2 := PinRele2
	rele1.Configure(machine.PinConfig{Mode: machine.PinOutput})
	rele2.Configure(machine.PinConfig{Mode: machine.PinOutput})
	releOff(rele1)
	releOff(rele2)

	// Botones con pull-up interno
	// Conexión: GP18/GP19 → botón → GND  (sin resistencias externas)
	btnModo := PinBtnModo
	btnBomba := PinBtnBomba
	btnModo.Configure(machine.PinConfig{Mode: machine.PinInputPullup})
	btnBomba.Configure(machine.PinConfig{Mode: machine.PinInputPullup})

	//colocar boton que encuinde la bomba como wakeup source para salir de sleep al presionarlo
	configurarWakeup(btnBomba)

	// ── Lanzar Core 1 con los sensores ────────────────────────────────
	//!!IMPORTANTE: lanzar Core 1 cuabdo tengas los sensores de nivel y temperatura funcionando, para evitar bloqueos por fallas en la lectura.
	//!! ajustar: las constantes adcNivelVacio y adcNivelLleno son valores calculados teóricos. Cuando tengas el sensor conectado, medí el valor real con el tanque vacío y lleno usando un println(adc.Get()) y actualizá esos dos números para una calibración exacta.
	StartSensorCore()

	// Pantalla
	epd := newEPD()
	led.Low()

	// Evaluar relés con el estado inicial antes de dibujar
	actualizarReles()

	// Dibujo y refresco inicial
	drawPantalla(epd)
	for i := 0; i < 6; i++ {
		led.High()
		time.Sleep(100 * time.Millisecond)
		led.Low()
		time.Sleep(100 * time.Millisecond)
	}
	epd.show()

	// Estado anterior de botones para detectar flanco de bajada
	btnModoAnterior := true
	btnBombaAnterior := true

	// Estado anterior para detectar si algo cambió y hay que redibujar
	prevModo := ModoAuto
	prevBomba := BombaOn
	prevNivel := NivelAgua
	prevBatPct := BatPct
	prevBatMV := BatMV
	prevBatEst := BatEstadoAct

	// ── Loop principal ─────────────────────────────────────────────────────
	time.Sleep(2 * time.Second)
	fmt.Println("va a comenzar el ciclo") // pausa para que el usuario vea la pantalla inicial antes de que empiece a actualizarse
	for {

		// ── Leer sensores desde Core 1 (copia segura con mutex) ───────
		//TODO: ACTIVAR ESTO CUANDO SE TENGAN LOS SENSORES REALES FUNCIONANDO, PARA QUE LOS DATOS SE ACTUALICEN REALMENTE. POR AHORA ESTÁ COMENTADO PARA EVITAR ERRORES DE LECTURA QUE BLOQUEEN EL PROGRAMA.
		datos := ReadSensors()
		NivelAgua = datos.NivelAgua
		TempAgua = datos.TempAgua
		TempExterior = datos.TempExterior
		BatPct = datos.BatPct
		BatMV = datos.BatMV
		BatEstadoAct = datos.BatEstado

		// ── Botón GP18: alternar AUTO / MANUAL ────────────────────────
		btnModoActual := btnModo.Get()
		if !btnModoActual && btnModoAnterior {
			time.Sleep(20 * time.Millisecond) // antirrebote
			if !btnModo.Get() {
				registrarActividad()
				if estadoSistema == EstadoActivo {

					ModoAuto = !ModoAuto
				}
			}
		}
		btnModoAnterior = btnModoActual

		// ── Botón GP19: encender/apagar bomba (solo en MANUAL) ────────
		btnBombaActual := btnBomba.Get()
		if !btnBombaActual && btnBombaAnterior {
			time.Sleep(20 * time.Millisecond) // antirrebote
			if !btnBomba.Get() {
				registrarActividad()
				if estadoSistema == EstadoActivo && !ModoAuto {
					BombaOn = !BombaOn
				}
				if estadoSistema == EstadoAlerta {
					//Boton de bomba en alerta = confirma / ignorar alerta. Si el usuario presiona el botón, se asume que está consciente de la situación y quiere ignorar la alerta (por ejemplo, nivel bajo pero sabe que es porque está usando agua). Si no presiona el botón, se asume que no está consciente o no puede atender la situación, y se mantiene la alerta activa.
					estadoSistema = EstadoActivo
					alertaActual = AlertaNinguna
				}
				// la protección de 90% la aplica actualizarReles()
			}
		}
		btnBombaAnterior = btnBombaActual

		// ── Aplicar lógica de relés ────────────────────────────────────
		actualizarReles()
		/*
		   ######################################
		   #   MAQUINA DE ESTADOS DEL SISTEMA   #
		   ######################################
		*/
		alerta, hayAlerta := checkAlertas(datos)
		switch estadoSistema {
		case EstadoActivo:
			if hayAlerta && alerta == AlertaTanqueLleno {
				// Tanque lleno → entrar en sleep (no es urgente)
				estadoSistema = EstadoSleep
				epd.SleepClean()
				fmt.Println("→ SLEEP: tanque lleno")
			} else if verificarTimeout() && !hayAlerta {
				// Inactividad → sleep
				estadoSistema = EstadoSleep
				epd.SleepClean()
				fmt.Println("→ SLEEP: timeout")
			} else {
				// Redibujar si algo cambió
				if NivelAgua != prevNivel || ModoAuto != prevModo || BombaOn != prevBomba {
					prevNivel = NivelAgua
					prevModo = ModoAuto
					prevBomba = BombaOn
					drawPantalla(epd)
					epd.show()
				}
			}
		case EstadoSleep:
			if hayAlerta && alerta != AlertaTanqueLleno {
				// Condición crítica → despertar y mostrar alerta
				alertaActual = alerta
				estadoSistema = EstadoAlerta
				epd = newEPD() // reset hardware completo para despertar la pantalla
				drawPantallaAlerta(epd, alerta)
				epd.show()
				fmt.Println("→ ALERTA:", alerta)
				break
			}
			select {
			case <-wakeupChan:
				// Botón presionado → despertar
				registrarActividad()
				estadoSistema = EstadoActivo
				epd = newEPD() // reset hardware pantalla
				drawPantalla(epd)
				epd.show()
				fmt.Println("→ ACTIVO: wake-up por botón")
			case <-time.After(10 * time.Second):
				// Cada 10s salir del select para verificar alertas
				// (el Core 1 sigue leyendo sensores, esto solo re-chequea)
			}
			// En sleep: solo el sensor loop de Core 1 sigue corriendo

		case EstadoAlerta:
			// Si la condición se resolvió sola → volver a activo
			if !hayAlerta {
				estadoSistema = EstadoActivo
				alertaActual = AlertaNinguna
				registrarActividad()
				drawPantalla(epd)
				epd.show()
			}
		}
		//TODO verificar esta parte antes de flashear el rp pico,
		// ── Redibujar pantalla si algo cambió ──────────────────────────
		batCambio := BatPct != prevBatPct || BatMV != prevBatMV || BatEstadoAct != prevBatEst
		if NivelAgua != prevNivel || ModoAuto != prevModo ||
			BombaOn != prevBomba || batCambio {
			prevNivel = NivelAgua
			prevModo = ModoAuto
			prevBomba = BombaOn
			fmt.Println("cumplio la espresion")
			prevBatPct = BatPct
			prevBatMV = BatMV
			prevBatEst = BatEstadoAct
			drawPantalla(epd)
			epd.show() // full refresh 4 grises (~4 segundos)
			fmt.Println("se redibujó la pantalla")
		}
		fmt.Println("ciclo completo, esperando el próximo...")
		time.Sleep(50 * time.Millisecond) // polling rápido para no perder pulsaciones

	}
}
