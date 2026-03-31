package main

import "time"

// ── Estados del sistema ───────────────────────────────────────────────────
type SistemaEstado uint8

const (
	EstadoActivo SistemaEstado = iota
	EstadoSleep                // pantalla OFF, sensores corriendo
	EstadoAlerta               // pantalla ON con mensaje urgente
)

// ── Configuración de umbrales de alerta ───────────────────────────────────
const (
	AlertaNivelBajo   = 30              // % — enciende pantalla si baja de aquí
	AlertaBateriaBaja = 20              // % — enciende pantalla si batería baja de aquí
	AlertaTempAlta    = 50              // °C — temperatura de agua peligrosa
	TimeoutSleep      = 5 * time.Minute // inactividad → sleep
)

var (
	estadoSistema   = EstadoActivo
	ultimaActividad = time.Now()
	wakeupChan      = make(chan struct{}, 1)
)

// ── Tipo de alerta para saber qué mensaje mostrar ─────────────────────────
type TipoAlerta uint8

const (
	AlertaNinguna     TipoAlerta = iota
	AlertaNivelBajoT             // NivelAgua < 30%
	AlertaBateriaT               // BatPct < 20%
	AlertaTempAltaT              // TempAgua > 50°C
	AlertaSensorErr              // TempAgua == -99
	AlertaTanqueLleno            // NivelAgua >= UmbralApagado
)

var alertaActual = AlertaNinguna

// ── Evalúa si hay que salir de SLEEP por una condición crítica ────────────
// Llamar desde el loop principal (Core 0) en cada iteración.
func checkAlertas(datos SensorData) (TipoAlerta, bool) {
	// Prioridad: sensor roto > temp alta > batería > nivel bajo > tanque lleno
	switch {
	case datos.TempAgua == -99 || datos.TempExterior == -99:
		return AlertaSensorErr, true
	case datos.TempAgua > AlertaTempAlta:
		return AlertaTempAltaT, true
	case datos.BatPct < AlertaBateriaBaja:
		return AlertaBateriaT, true
	case datos.NivelAgua < AlertaNivelBajo:
		return AlertaNivelBajoT, true
	case datos.NivelAgua >= UmbralApagado:
		return AlertaTanqueLleno, true
	}
	return AlertaNinguna, false
}

// ── Registra actividad (llamar al detectar pulsación de botón) ────────────
func registrarActividad() {
	ultimaActividad = time.Now()
	if estadoSistema == EstadoSleep {
		estadoSistema = EstadoActivo
	}
}

// ── Verifica si expiró el timeout de inactividad ──────────────────────────
func verificarTimeout() bool {
	return time.Since(ultimaActividad) > TimeoutSleep
}
