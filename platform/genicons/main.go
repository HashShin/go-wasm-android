// genicons — generate all platform icons from app/Icon.png
// Uses only the standard library (no external deps).
// Outputs:
//   platform/android/app/src/main/res/mipmap-*/ic_launcher.png  (5 densities)
//   app/ui/favicon.png  (32x32)
//   app/ui/favicon.ico  (16, 32, 48px)
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
)

var androidDensities = []struct {
	name string
	size int
}{
	{"mdpi", 48},
	{"hdpi", 72},
	{"xhdpi", 96},
	{"xxhdpi", 144},
	{"xxxhdpi", 192},
}

func main() {
	log.SetFlags(0)

	root := "."
	if len(os.Args) > 1 {
		root = os.Args[1]
	}

	iconPath := filepath.Join(root, "app", "Icon.png")
	src, err := loadImage(iconPath)
	if err != nil {
		log.Printf("[icons] app/Icon.png not found — skipping icon generation")
		return
	}
	log.Printf("[icons] %s (%dx%d)", iconPath, src.Bounds().Dx(), src.Bounds().Dy())

	// ── Android mipmap icons ──────────────────────────────────────────────────
	for _, d := range androidDensities {
		dir := filepath.Join(root, "platform", "android", "app", "src", "main", "res",
			fmt.Sprintf("mipmap-%s", d.name))
		must(os.MkdirAll(dir, 0755))
		out := filepath.Join(dir, "ic_launcher.png")
		must(savePNG(out, resize(src, d.size, d.size)))
		log.Printf("[icons] android mipmap-%s → %dx%d", d.name, d.size, d.size)
	}

	// ── Web favicon.png (32x32) ───────────────────────────────────────────────
	faviconPNG := filepath.Join(root, "app", "ui", "favicon.png")
	must(savePNG(faviconPNG, resize(src, 32, 32)))
	log.Printf("[icons] favicon.png → 32x32")

	// ── Web favicon.ico (16, 32, 48) ──────────────────────────────────────────
	faviconICO := filepath.Join(root, "app", "ui", "favicon.ico")
	must(writeICO(faviconICO, []image.Image{
		resize(src, 16, 16),
		resize(src, 32, 32),
		resize(src, 48, 48),
	}))
	log.Printf("[icons] favicon.ico → 16, 32, 48px")

	log.Println("[icons] done")
}

// ── image helpers ─────────────────────────────────────────────────────────────

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	return img, err
}

// resize scales src to w×h using bilinear interpolation.
func resize(src image.Image, w, h int) image.Image {
	sb := src.Bounds()
	sw := float64(sb.Dx())
	sh := float64(sb.Dy())
	dst := image.NewRGBA(image.Rect(0, 0, w, h))

	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			sx := (float64(x)+0.5)*sw/float64(w) - 0.5
			sy := (float64(y)+0.5)*sh/float64(h) - 0.5

			x0 := int(math.Floor(sx))
			y0 := int(math.Floor(sy))
			x1 := x0 + 1
			y1 := y0 + 1
			fx := sx - float64(x0)
			fy := sy - float64(y0)

			clampX := func(v int) int { return clamp(v, 0, sb.Dx()-1) }
			clampY := func(v int) int { return clamp(v, 0, sb.Dy()-1) }

			c00 := toFloat(src.At(sb.Min.X+clampX(x0), sb.Min.Y+clampY(y0)))
			c10 := toFloat(src.At(sb.Min.X+clampX(x1), sb.Min.Y+clampY(y0)))
			c01 := toFloat(src.At(sb.Min.X+clampX(x0), sb.Min.Y+clampY(y1)))
			c11 := toFloat(src.At(sb.Min.X+clampX(x1), sb.Min.Y+clampY(y1)))

			dst.SetRGBA(x, y, color.RGBA{
				R: uint8(bilerp(c00[0], c10[0], c01[0], c11[0], fx, fy)),
				G: uint8(bilerp(c00[1], c10[1], c01[1], c11[1], fx, fy)),
				B: uint8(bilerp(c00[2], c10[2], c01[2], c11[2], fx, fy)),
				A: uint8(bilerp(c00[3], c10[3], c01[3], c11[3], fx, fy)),
			})
		}
	}
	return dst
}

func toFloat(c color.Color) [4]float64 {
	r, g, b, a := c.RGBA()
	return [4]float64{float64(r >> 8), float64(g >> 8), float64(b >> 8), float64(a >> 8)}
}

func bilerp(c00, c10, c01, c11, fx, fy float64) float64 {
	top := c00*(1-fx) + c10*fx
	bot := c01*(1-fx) + c11*fx
	return top*(1-fy) + bot*fy
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func savePNG(path string, img image.Image) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

// writeICO writes a .ico containing PNG-encoded images.
// PNG-in-ICO is supported by all modern browsers and Windows Vista+.
func writeICO(path string, imgs []image.Image) error {
	var pngData [][]byte
	for _, img := range imgs {
		var b bytes.Buffer
		if err := png.Encode(&b, img); err != nil {
			return err
		}
		pngData = append(pngData, b.Bytes())
	}

	var buf bytes.Buffer
	n := len(imgs)

	binary.Write(&buf, binary.LittleEndian, uint16(0))
	binary.Write(&buf, binary.LittleEndian, uint16(1))
	binary.Write(&buf, binary.LittleEndian, uint16(n))

	offset := uint32(6 + n*16)
	for i, img := range imgs {
		bnd := img.Bounds()
		w, h := uint8(bnd.Dx()), uint8(bnd.Dy())
		if bnd.Dx() >= 256 {
			w = 0
		}
		if bnd.Dy() >= 256 {
			h = 0
		}
		binary.Write(&buf, binary.LittleEndian, w)
		binary.Write(&buf, binary.LittleEndian, h)
		binary.Write(&buf, binary.LittleEndian, uint8(0))
		binary.Write(&buf, binary.LittleEndian, uint8(0))
		binary.Write(&buf, binary.LittleEndian, uint16(1))
		binary.Write(&buf, binary.LittleEndian, uint16(32))
		binary.Write(&buf, binary.LittleEndian, uint32(len(pngData[i])))
		binary.Write(&buf, binary.LittleEndian, offset)
		offset += uint32(len(pngData[i]))
	}

	for _, d := range pngData {
		buf.Write(d)
	}

	return os.WriteFile(path, buf.Bytes(), 0644)
}

func must(err error) {
	if err != nil {
		log.Fatal(err)
	}
}
