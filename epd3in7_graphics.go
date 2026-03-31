// Code generated for Waveshare Pico-ePaper-3.7 + RP2040 (TinyGo)
// Flashear: tinygo flash -target=pico .
package main

func (e *EPD) getpx(buf []byte, width, x, y int) uint8 {
	pos := y*width + x
	return (buf[pos/4] >> uint((pos%4)*2)) & 0x03
}
func (e *EPD) setpx(buf []byte, width, x, y int, c uint8) {
	pos := y*width + x
	sh := uint((pos % 4) * 2)
	buf[pos/4] &^= 0x03 << sh
	buf[pos/4] |= (c & 0x03) << sh
}
func (e *EPD) px(x, y int, c uint8) {
	if x >= 0 && x < 480 && y >= 0 && y < 280 {
		e.setpx(e.buf, 480, x, y, c)
	}
}
func (e *EPD) fillScreen(c uint8) {
	var p byte
	for i := 0; i < 4; i++ {
		p |= (c & 0x03) << uint(6-i*2)
	}
	for i := range e.buf {
		e.buf[i] = p
	}
}
func (e *EPD) hline(x, y, l int, c uint8) {
	for i := 0; i < l; i++ {
		e.px(x+i, y, c)
	}
}
func (e *EPD) vline(x, y, l int, c uint8) {
	for i := 0; i < l; i++ {
		e.px(x, y+i, c)
	}
}
func (e *EPD) rect(x, y, w, h int, c uint8) {
	e.hline(x, y, w, c)
	e.hline(x, y+h-1, w, c)
	e.vline(x, y, h, c)
	e.vline(x+w-1, y, h, c)
}
func (e *EPD) fillRect(x, y, w, h int, c uint8) {
	for row := y; row < y+h; row++ {
		e.hline(x, row, w, c)
	}
}
func (e *EPD) line(x0, y0, x1, y1 int, c uint8) {
	dx := x1 - x0
	if dx < 0 {
		dx = -dx
	}
	dy := y1 - y0
	if dy < 0 {
		dy = -dy
	}
	sx := 1
	if x0 > x1 {
		sx = -1
	}
	sy := 1
	if y0 > y1 {
		sy = -1
	}
	err := dx - dy
	for {
		e.px(x0, y0, c)
		if x0 == x1 && y0 == y1 {
			break
		}
		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}
func isqrt(n int) int {
	if n <= 0 {
		return 0
	}
	x := n
	for {
		x1 := (x + n/x) / 2
		if x1 >= x {
			return x
		}
		x = x1
	}
}
func (e *EPD) circle(cx, cy, r int, c uint8) {
	x, y, d := 0, r, 3-2*r
	for y >= x {
		e.px(cx+x, cy+y, c)
		e.px(cx-x, cy+y, c)
		e.px(cx+x, cy-y, c)
		e.px(cx-x, cy-y, c)
		e.px(cx+y, cy+x, c)
		e.px(cx-y, cy+x, c)
		e.px(cx+y, cy-x, c)
		e.px(cx-y, cy-x, c)
		if d < 0 {
			d += 4*x + 6
		} else {
			d += 4*(x-y) + 10
			y--
		}
		x++
	}
}
func (e *EPD) fillCircle(cx, cy, r int, c uint8) {
	for dy := -r; dy <= r; dy++ {
		dx := isqrt(r*r - dy*dy)
		e.hline(cx-dx, cy+dy, 2*dx+1, c)
	}
}
func (e *EPD) triangle(x0, y0, x1, y1, x2, y2 int, c uint8) {
	e.line(x0, y0, x1, y1, c)
	e.line(x1, y1, x2, y2, c)
	e.line(x2, y2, x0, y0, c)
}

// drawChar: MSB del byte = pixel izquierdo de la fila
func (e *EPD) drawChar(x, y int, ch byte, f Font, c uint8) {
	if ch < 32 || ch > 126 {
		ch = '?'
	}
	base := int(ch-32) * f.height * f.bytesPerRow
	for row := 0; row < f.height; row++ {
		for b := 0; b < f.bytesPerRow; b++ {
			byt := f.table[base+row*f.bytesPerRow+b]
			for bit := 0; bit < 8; bit++ {
				col := b*8 + bit
				if col >= f.width {
					break
				}
				if byt&(0x80>>uint(bit)) != 0 {
					e.px(x+col, y+row, c)
				}
			}
		}
	}
}
func (e *EPD) drawText(x, y int, s string, f Font, c uint8) {
	for i := 0; i < len(s); i++ {
		e.drawChar(x+i*(f.width+1), y, s[i], f, c)
	}
}

// ─────────────────────────────────────────────
// IMAGEN BITMAP DE CUALQUIER TAMAÑO
// ─────────────────────────────────────────────
//
// drawImage dibuja una imagen de cualquier tamaño en la posición (x, y).
//
// Formato del bitmap (imgW × imgH píxeles, 2 bits/px, LSB primero):
//   - Cada byte contiene 4 píxeles consecutivos.
//   - El píxel más a la IZQUIERDA está en los bits 1-0 (LSB).
//   - Colores: 0x00=Black 0x01=DarkGray 0x02=LightGray 0x03=White
//   - Tamaño del slice: ceil(imgW * imgH / 4) bytes.
//   - Si la imagen sobresale de la pantalla se recorta automáticamente.
func (e *EPD) drawImage(x, y int, img []byte, imgW, imgH int) {
	for row := 0; row < imgH; row++ {
		dy := y + row
		if dy < 0 || dy >= EPD_HEIGHT {
			continue
		}
		for col := 0; col < imgW; col++ {
			dx := x + col
			if dx < 0 || dx >= EPD_HEIGHT {
				continue
			}
			pos := row*imgW + col
			px := uint8((img[pos/4] >> uint((pos%4)*2)) & 0x03)
			e.px(dx, dy, px)
		}
	}
}

// drawImageMono dibuja una imagen monocroma (1 bit/px, MSB primero) en (x, y).
//
// Formato del bitmap (imgW × imgH píxeles, 1 bit/px):
//   - MSB del byte = píxel más a la izquierda de cada fila.
//   - bit=1 → color c1 (foreground), bit=0 → color c0 (background).
//   - Filas de imgW píxeles, cada fila ocupa ceil(imgW/8) bytes.
//   - Tamaño del slice: ceil(imgW/8) * imgH bytes.
func (e *EPD) drawImageMono(x, y int, img []byte, imgW, imgH int, c1, c0 uint8) {
	bpr := (imgW + 7) / 8
	for row := 0; row < imgH; row++ {
		dy := y + row
		if dy < 0 || dy >= EPD_HEIGHT {
			continue
		}
		for col := 0; col < imgW; col++ {
			dx := x + col
			if dx < 0 || dx >= EPD_WIDTH {
				continue
			}
			byt := img[row*bpr+col/8]
			if byt&(0x80>>uint(col%8)) != 0 {
				e.px(dx, dy, c1)
			} else {
				e.px(dx, dy, c0)
			}
		}
	}
}

// ─────────────────────────────────────────────
// RECTÁNGULO CON ESQUINAS REDONDEADAS
// ─────────────────────────────────────────────

// roundRect dibuja el contorno de un rectángulo con esquinas redondeadas.
// x,y = esquina sup-izq | w,h = dimensiones | r = radio esquinas | c = color
func (e *EPD) roundRect(x, y, w, h, r int, c uint8) {
	if r > w/2 {
		r = w / 2
	}
	if r > h/2 {
		r = h / 2
	}
	e.hline(x+r, y, w-2*r, c)
	e.hline(x+r, y+h-1, w-2*r, c)
	e.vline(x, y+r, h-2*r, c)
	e.vline(x+w-1, y+r, h-2*r, c)
	e.drawRoundCorners(x+r, y+r, x+w-r-1, y+h-r-1, r, c, false)
}

// fillRoundRect dibuja un rectángulo relleno con esquinas redondeadas.
func (e *EPD) fillRoundRect(x, y, w, h, r int, c uint8) {
	if r > w/2 {
		r = w / 2
	}
	if r > h/2 {
		r = h / 2
	}
	e.fillRect(x+r, y, w-2*r, h, c)
	e.fillRect(x, y+r, r, h-2*r, c)
	e.fillRect(x+w-r, y+r, r, h-2*r, c)
	e.drawRoundCorners(x+r, y+r, x+w-r-1, y+h-r-1, r, c, true)
}

// drawRoundCorners dibuja o rellena las 4 esquinas redondeadas.
// (cx1,cy1) = centro esquina sup-izq | (cx2,cy2) = centro esquina inf-der
func (e *EPD) drawRoundCorners(cx1, cy1, cx2, cy2, r int, c uint8, fill bool) {
	xi, yi, d := 0, r, 3-2*r
	for yi >= xi {
		if fill {
			e.hline(cx1-yi, cy1-xi, yi, c)
			e.hline(cx2+1, cy1-xi, yi, c)
			e.hline(cx1-yi, cy2+xi, yi, c)
			e.hline(cx2+1, cy2+xi, yi, c)
			if xi != 0 {
				e.hline(cx1-xi, cy1-yi, xi, c)
				e.hline(cx2+1, cy1-yi, xi, c)
				e.hline(cx1-xi, cy2+yi, xi, c)
				e.hline(cx2+1, cy2+yi, xi, c)
			}
		} else {
			e.px(cx2+xi, cy1-yi, c)
			e.px(cx2+yi, cy1-xi, c)
			e.px(cx1-xi, cy1-yi, c)
			e.px(cx1-yi, cy1-xi, c)
			e.px(cx2+xi, cy2+yi, c)
			e.px(cx2+yi, cy2+xi, c)
			e.px(cx1-xi, cy2+yi, c)
			e.px(cx1-yi, cy2+xi, c)
		}
		if d < 0 {
			d += 4*xi + 6
		} else {
			d += 4*(xi-yi) + 10
			yi--
		}
		xi++
	}
}

// ─────────────────────────────────────────────
// TEXTO CENTRADO
// ─────────────────────────────────────────────

// drawTextCentered centra el texto horizontal y verticalmente dentro del área (x,y,w,h).
func (e *EPD) drawTextCentered(x, y, w, h int, text string, f Font, c uint8) {
	tw := len(text)*(f.width+1) - 1
	tx := x + (w-tw)/2
	ty := y + (h-f.height)/2
	if tx < x {
		tx = x
	}
	if ty < y {
		ty = y
	}
	e.drawText(tx, ty, text, f, c)
}

// drawTextCenteredH centra el texto solo horizontalmente dentro del ancho w.
func (e *EPD) drawTextCenteredH(x, y, w int, text string, f Font, c uint8) {
	tw := len(text)*(f.width+1) - 1
	tx := x + (w-tw)/2
	if tx < x {
		tx = x
	}
	e.drawText(tx, y, text, f, c)
}
