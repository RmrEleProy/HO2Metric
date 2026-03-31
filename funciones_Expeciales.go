package main

import (
	"fmt"
)

/*
############################################
#  dibuja la cabecera de la interface      #
#  título, estado de batería y voltaje     #
############################################
*/

func drawHeader(epd *EPD) {
	epd.fillRect(0, 0, 480, 26, ColorBlack)

	// Título (zona izquierda)
	epd.drawTextCentered(0, 0, 368, 26, "HO2Metric", Font20, ColorWhite)

	// Separador
	epd.vline(368, 2, 22, ColorDarkGray)

	// Ícono batería
	drawBateria(epd, 370, 5, 52, 18, BatPct)

}

func batEstadoStr() (string, uint8) {
	switch BatEstadoAct {
	case BatCargando:
		return "+carg", ColorWhite
	case BatCompleta:
		return "llena", ColorLightGray
	case BatDescargando:
		return "-desc", ColorLightGray
	default:
		return "bat?", ColorLightGray
	}
}

/*
###########################################
#  Funcion para dibujar el tanque de agua #
###########################################
*/

func drawTanque(epd *EPD, ox, oy, nivel int) {
	const (
		tanX = 8
		tanW = 140
		tanY = 10
		tanH = 150
		r    = 8
	)
	tx := ox + tanX
	ty := oy + tanY

	// Tapa superior
	epd.fillRoundRect(tx, ty-6, tanW, 12, 6, ColorDarkGray)
	epd.roundRect(tx, ty-6, tanW, 12, 6, ColorBlack)

	// Cuerpo vacío
	epd.roundRect(tx, ty, tanW, tanH, r, ColorBlack)

	// Relleno de agua
	altoAgua := (nivel * (tanH - 4)) / 100
	yAgua := ty + tanH - 2 - altoAgua

	var colorAgua uint8
	switch {
	case nivel <= 20:
		colorAgua = ColorLightGray
	case nivel <= 80:
		colorAgua = ColorDarkGray
	default:
		colorAgua = ColorBlack
	}

	if altoAgua > 0 {
		epd.fillRoundRect(tx+2, yAgua, tanW-4, altoAgua, r-2, colorAgua)
	}

	// Ondas en superficie
	if altoAgua > 3 {
		for wx := tx + 2; wx < tx+tanW-6; wx += 8 {
			epd.hline(wx, yAgua, 4, ColorBlack)
			if wx+4 < tx+tanW-6 {
				epd.hline(wx+4, yAgua+1, 4, ColorBlack)
			}
		}
	}

	// Marcas de nivel (lado derecho)
	marcaX := tx + tanW + 2
	for pct := 0; pct <= 100; pct += 25 {
		my := ty + tanH - 2 - (pct*(tanH-4))/100
		epd.hline(marcaX, my, 4, ColorDarkGray)
		if pct == 0 || pct == 50 || pct == 100 {
			epd.drawText(marcaX+6, my-4, itoa(pct), Font8, ColorDarkGray)
		}
	}

	// Texto % y litros centrado en el cuerpo
	litros := (CapacidadMax * nivel) / 100
	textY := ty + tanH/2 - 10
	if textY < ty+4 {
		textY = ty + 4
	}
	epd.drawTextCentered(tx, textY, tanW, 20, itoa(nivel)+"%", Font24, ColorLightGray)
	epd.drawTextCentered(tx, textY+24, tanW, 14, itoa(litros)+"L", Font24, ColorLightGray)

	// Base / tapa inferior
	epd.fillRoundRect(tx, ty+tanH-4, tanW, 8, 4, ColorDarkGray)
	epd.roundRect(tx, ty+tanH-4, tanW, 8, 4, ColorBlack)

	// Válvula de salida
	epd.fillRect(tx+tanW/2-10, ty+tanH+4, 20, 6, ColorDarkGray)
	epd.fillRect(tx+tanW/2-6, ty+tanH+10, 12, 8, ColorBlack)
}

/*
############################################
# Funcion que dibuja el panel izquierdo    #
# con datos principales y estado de bomba  #
############################################
*/

func drawDatos(epd *EPD, cx int) {
	const cw = 250
	cardW := (cw - 4) / 2

	// ── Temp Agua + Temp Exterior en fila ────────────────────────────────

	epd.roundRect(cx, 40, cardW, 36, 4, ColorDarkGray)
	epd.drawText(cx+4, 45, "AGUA", Font8, ColorDarkGray)
	epd.drawTextCentered(cx, 54, cardW, 18, itoa(TempAgua)+"C", Font16, ColorBlack)

	epd.roundRect(cx+cardW+4, 40, cardW, 36, 4, ColorDarkGray)
	epd.drawText(cx+cardW+8, 45, "EXT.", Font8, ColorDarkGray)
	epd.drawTextCentered(cx+cardW+4, 54, cardW, 18, itoa(TempExterior)+"C", Font16, ColorBlack)

	//-------estados de la bateria en fila (IGNORED)----------------------
	epd.roundRect(cx, 80, cardW, 36, 4, ColorDarkGray)
	epd.drawText(cx+4, 85, "BATERIA", Font8, ColorDarkGray)
	estadoStr, estadoColor := batEstadoStr()
	epd.drawTextCentered(cx, 94, cardW, 18, estadoStr, Font16, estadoColor)

	epd.roundRect(cx+cardW+4, 80, cardW, 36, 4, ColorDarkGray)
	epd.drawText(cx+cardW+8, 85, "VOLT.", Font8, ColorDarkGray)
	voltStr := itoa(BatMV/1000) + "." + itoa((BatMV%1000)/10) + "V"
	epd.drawTextCentered(cx+cardW+4, 94, cardW, 18, voltStr, Font16, ColorBlack)

	// ── Barra de nivel ────────────────────────────────────────────────────
	// epd.drawText(cx, 92, "NIVEL", Font12, ColorDarkGray)
	// barW := cw - 32
	// barFill := (barW * NivelAgua) / 100
	// epd.rect(cx, 102, barW, 10, ColorDarkGray)
	// if barFill > 0 {
	// 	epd.fillRect(cx+1, 102, barFill-1, 8, ColorBlack)
	// }
	// epd.drawText(cx+barW+4, 102, itoa(NivelAgua)+"%", Font12, ColorBlack)

	// ── Card litros (fondo negro) ─────────────────────────────────────────
	litros := (CapacidadMax * NivelAgua) / 100
	epd.fillRoundRect(cx, 126, cw, 26, 4, ColorBlack)
	epd.drawText(cx+4, 130, "LITROS", Font8, ColorWhite)
	epd.drawTextCentered(cx, 138, cw, 14,
		itoa(litros)+"/"+itoa(CapacidadMax)+" L", Font12, ColorWhite)

	// ── Card Modo ─────────────────────────────────────────────────────────
	fmt.Println("vrificacion del modo")
	modoStr := "MANUAL"
	if ModoAuto {
		modoStr = "AUTO"
		fmt.Println("cambio de modo a auto")
	}
	if ModoAuto {
		epd.fillRoundRect(cx, 160, cw, 24, 4, ColorDarkGray)
		epd.drawTextCentered(cx, 160, cw, 24, "MODO: "+modoStr, Font16, ColorWhite)
	} else {
		fmt.Println("cambio de modo a manual: variable modo:", modoStr)
		epd.roundRect(cx, 160, cw, 24, 4, ColorDarkGray)
		epd.drawTextCentered(cx, 160, cw, 24, "MODO: "+modoStr, Font16, ColorBlack)
	}

	// ── Card Estado Bomba ─────────────────────────────────────────────────
	switch {
	case BombaOn:
		epd.fillRoundRect(cx, 200, cw, 24, 4, ColorBlack)
		epd.drawTextCentered(cx, 200, cw, 24, "BOMBA: ON", Font16, ColorWhite)
	case NivelAgua >= UmbralApagado:
		epd.fillRoundRect(cx, 200, cw, 24, 4, ColorLightGray)
		epd.drawTextCentered(cx, 200, cw, 24, "TANQUE LLENO", Font16, ColorBlack)
	default:
		epd.roundRect(cx, 200, cw, 24, 4, ColorDarkGray)
		epd.drawTextCentered(cx, 200, cw, 24, "BOMBA: OFF", Font16, ColorBlack)
	}

	// ── Línea de estado inferior ──────────────────────────────────────────
	epd.rect(cx, 238, cw, 20, ColorLightGray)
	epd.drawTextCenteredH(cx, 240, cw, "ENC<"+itoa(UmbralEncendido)+"%  APG>"+itoa(UmbralApagado)+"%",
		Font16, ColorDarkGray)
}

/*
############################################
# Funcion que dibuja el ícono de batería   #
############################################
*/

func drawBateria(epd *EPD, x, y, w, h, pct int) {
	epd.fillRect(x, y, w-6, h, ColorWhite)
	epd.fillRect(x+1, y+1, w-8, h-2, ColorWhite)
	poloH := h / 2
	epd.fillRect(x+w-6, y+(h-poloH)/2, 6, poloH, ColorWhite)
	fillW := ((w - 10) * pct) / 100
	if fillW > 0 {
		epd.fillRect(x+2, y+2, fillW, h-4, ColorDarkGray)
	}
	epd.drawText(x+w+6, y+(h-poloH)/2, itoa(pct)+"%", Font16, ColorWhite)
}

// ── Lógica de control de relés ────────────────────────────────────────────
//
// AUTOMATICO:
//
//	nivel < 20%  → enciende bomba
//	nivel >= 90% → apaga bomba
//	20-89%       → histéresis, mantiene estado actual
//
// MANUAL:
//
//	nivel >= 90% → apaga bomba forzosamente (protección)
//	nivel < 90%  → respeta BombaOn (el usuario decide con GP20)
//
// Devuelve true si BombaOn cambió.
func actualizarReles() bool {
	antes := BombaOn

	if ModoAuto {
		if NivelAgua < UmbralEncendido {
			BombaOn = true
		} else if NivelAgua >= UmbralApagado {
			BombaOn = false
		}
	} else {
		if NivelAgua >= UmbralApagado {
			BombaOn = false
		}
	}

	if BombaOn {
		releOn(PinRele1)
		releOn(PinRele2)
	} else {
		releOff(PinRele1)
		releOff(PinRele2)
	}

	return BombaOn != antes
}

func drawPantallaAlerta(epd *EPD, alerta TipoAlerta) {
	epd.fillScreen(ColorWhite)
	drawHeader(epd)

	var titulo, detalle string
	switch alerta {
	case AlertaNivelBajoT:
		titulo = "NIVEL BAJO"
		detalle = itoa(NivelAgua) + "% — bomba encendida"
	case AlertaBateriaT:
		titulo = "BATERIA BAJA"
		detalle = itoa(BatPct) + "% — " + itoa(BatMV/1000) + "." + itoa((BatMV%1000)/10) + "V"
	case AlertaTempAltaT:
		titulo = "TEMP. ALTA"
		detalle = itoa(TempAgua) + "°C en el agua"
	case AlertaSensorErr:
		titulo = "ERROR SENSOR"
		detalle = "revisar conexiones"
	case AlertaTanqueLleno:
		titulo = "TANQUE LLENO"
		detalle = itoa(NivelAgua) + "% — bomba apagada"
	}

	// Fondo de alerta (rectángulo oscuro centrado)
	epd.fillRoundRect(20, 80, 440, 120, 8, ColorBlack)
	epd.drawTextCentered(20, 90, 440, 30, titulo, Font24, ColorWhite)
	epd.drawTextCentered(20, 130, 440, 24, detalle, Font16, ColorLightGray)
	epd.drawTextCentered(20, 170, 440, 20, "Presiona BOMBA para confirmar", Font12, ColorWhite)

	// Tanque a la derecha con nivel actual
	drawTanque(epd, 300, 36, NivelAgua)
}
