package trayicon

import (
	"encoding/binary"
	"image"
	"image/color"
)

// ProxyPilotICO returns a small multi-size .ico payload suitable for Windows tray icons.
// It uses classic BMP/AND-mask frames for maximum compatibility with Windows loaders.
func ProxyPilotICO() []byte {
	sizes := []int{16, 32, 48}

	type iconImg struct {
		size int
		bmp  []byte
	}

	images := make([]iconImg, 0, len(sizes))
	for _, s := range sizes {
		img := renderProxyPilotIcon(s)
		bmp := encodeICOFrameBMP32(img)
		if len(bmp) == 0 {
			continue
		}
		images = append(images, iconImg{size: s, bmp: bmp})
	}
	if len(images) == 0 {
		return nil
	}

	// ICONDIR (6 bytes) + N*ICONDIRENTRY (16 bytes) + image blobs
	headerSize := 6 + 16*len(images)
	total := headerSize
	for _, im := range images {
		total += len(im.bmp)
	}
	out := make([]byte, 0, total)

	// ICONDIR
	var hdr [6]byte
	binary.LittleEndian.PutUint16(hdr[0:2], 0) // reserved
	binary.LittleEndian.PutUint16(hdr[2:4], 1) // type=icon
	binary.LittleEndian.PutUint16(hdr[4:6], uint16(len(images)))
	out = append(out, hdr[:]...)

	// Entries
	offset := uint32(headerSize)
	for _, im := range images {
		var e [16]byte
		w := byte(im.size)
		h := byte(im.size)
		if im.size >= 256 {
			w, h = 0, 0
		}
		e[0] = w
		e[1] = h
		e[2] = 0 // colors
		e[3] = 0 // reserved
		binary.LittleEndian.PutUint16(e[4:6], 1)  // planes
		binary.LittleEndian.PutUint16(e[6:8], 32) // bitcount
		binary.LittleEndian.PutUint32(e[8:12], uint32(len(im.bmp)))
		binary.LittleEndian.PutUint32(e[12:16], offset)
		offset += uint32(len(im.bmp))
		out = append(out, e[:]...)
	}

	// Image data
	for _, im := range images {
		out = append(out, im.bmp...)
	}
	return out
}

func renderProxyPilotIcon(size int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	bg := color.RGBA{R: 12, G: 18, B: 28, A: 255}
	accent := color.RGBA{R: 66, G: 165, B: 245, A: 255}
	fg := color.RGBA{R: 235, G: 245, B: 255, A: 255}

	// Background
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.SetRGBA(x, y, bg)
		}
	}

	// Accent border (1px)
	for x := 0; x < size; x++ {
		img.SetRGBA(x, 0, accent)
		img.SetRGBA(x, size-1, accent)
	}
	for y := 0; y < size; y++ {
		img.SetRGBA(0, y, accent)
		img.SetRGBA(size-1, y, accent)
	}

	// Draw a simple "PP" monogram using a tiny bitmap font scaled to size.
	scale := 1
	if size >= 32 {
		scale = 2
	}
	if size >= 64 {
		scale = 3
	}
	drawGlyph(img, size, fg, scale, 2*scale, 2*scale, glyphP)
	drawGlyph(img, size, fg, scale, (2*scale)+(6*scale), 2*scale, glyphP)

	// Small wing accent.
	drawRect(img, size, accent, 2*scale, (2*scale)+(11*scale), 12*scale, 2*scale)

	return img
}

var glyphP = []string{
	"11110",
	"10001",
	"11110",
	"10000",
	"10000",
}

func drawGlyph(img *image.RGBA, size int, c color.RGBA, scale int, ox int, oy int, rows []string) {
	for y, row := range rows {
		for x, ch := range row {
			if ch != '1' {
				continue
			}
			drawRect(img, size, c, ox+(x*scale), oy+(y*scale), scale, scale)
		}
	}
}

func drawRect(img *image.RGBA, size int, c color.RGBA, x int, y int, w int, h int) {
	for yy := y; yy < y+h; yy++ {
		if yy < 0 || yy >= size {
			continue
		}
		for xx := x; xx < x+w; xx++ {
			if xx < 0 || xx >= size {
				continue
			}
			img.SetRGBA(xx, yy, c)
		}
	}
}

func encodeICOFrameBMP32(img image.Image) []byte {
	b := img.Bounds()
	w := b.Dx()
	h := b.Dy()
	if w <= 0 || h <= 0 || w > 255 || h > 255 {
		return nil
	}

	// AND mask: 1bpp, rows padded to 32-bit boundaries.
	maskStride := ((w + 31) / 32) * 4
	maskSize := maskStride * h
	mask := make([]byte, maskSize) // all zeros (opaque)

	// XOR bitmap: 32bpp BGRA, bottom-up rows.
	pixels := make([]byte, w*h*4)
	i := 0
	for y := h - 1; y >= 0; y-- {
		for x := 0; x < w; x++ {
			r, g, bb, a := img.At(b.Min.X+x, b.Min.Y+y).RGBA()
			pixels[i+0] = byte(bb >> 8)
			pixels[i+1] = byte(g >> 8)
			pixels[i+2] = byte(r >> 8)
			pixels[i+3] = byte(a >> 8)
			i += 4
		}
	}

	// BITMAPINFOHEADER (40 bytes).
	var bih [40]byte
	binary.LittleEndian.PutUint32(bih[0:4], 40)           // biSize
	binary.LittleEndian.PutUint32(bih[4:8], uint32(w))    // biWidth
	binary.LittleEndian.PutUint32(bih[8:12], uint32(h*2)) // biHeight (XOR + AND)
	binary.LittleEndian.PutUint16(bih[12:14], 1)          // biPlanes
	binary.LittleEndian.PutUint16(bih[14:16], 32)         // biBitCount
	binary.LittleEndian.PutUint32(bih[16:20], 0)          // biCompression (BI_RGB)
	binary.LittleEndian.PutUint32(bih[20:24], uint32(len(pixels)+len(mask))) // biSizeImage

	out := make([]byte, 0, len(bih)+len(pixels)+len(mask))
	out = append(out, bih[:]...)
	out = append(out, pixels...)
	out = append(out, mask...)
	return out
}

