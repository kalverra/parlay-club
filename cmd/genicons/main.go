// Command genicons draws the app icons served under /static.
//
// It writes the home-screen, manifest, and maskable icons from the default
// theme's colors, so the art can be regenerated from source or swapped without
// touching the server. Run it with the rest of the generators:
//
//	go generate ./...
package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
)

// supersample is the scale icons are drawn at before averaging down, which
// keeps the ellipse and lace edges smooth.
const supersample = 4

var (
	// The paper-turf theme's page, accent, and page-ink colors.
	backgroundColor = color.NRGBA{R: 0x14, G: 0x26, B: 0x1c, A: 0xff}
	accentColor     = color.NRGBA{R: 0xd9, G: 0x77, B: 0x06, A: 0xff}
	inkColor        = color.NRGBA{R: 0xfa, G: 0xf6, B: 0xee, A: 0xff}
)

type iconSpec struct {
	name      string
	size      int
	maskable  bool
	fullBleed bool
}

func main() {
	out := flag.String("out", "static", "directory to write the PNGs into")
	flag.Parse()

	if err := run(*out); err != nil {
		fmt.Fprintf(os.Stderr, "genicons: %v\n", err)
		os.Exit(1)
	}
}

func run(out string) error {
	icons := []iconSpec{
		// iOS applies its own mask, so its touch icon stays a full square.
		{name: "icon-180.png", size: 180, fullBleed: true},
		{name: "icon-192.png", size: 192},
		{name: "icon-512.png", size: 512},
		{name: "icon-maskable-512.png", size: 512, maskable: true, fullBleed: true},
	}

	if err := os.MkdirAll(out, 0o750); err != nil {
		return fmt.Errorf("create %s: %w", out, err)
	}

	for _, spec := range icons {
		path := filepath.Join(out, spec.name)
		if err := writeIcon(path, spec); err != nil {
			return err
		}
	}

	return nil
}

func writeIcon(path string, spec iconSpec) error {
	file, err := os.Create(path) //nolint:gosec // the output path is a committed static asset.
	if err != nil {
		return fmt.Errorf("create %s: %w", path, err)
	}

	if err := png.Encode(file, draw(spec.size, spec.maskable, spec.fullBleed)); err != nil {
		_ = file.Close()
		return fmt.Errorf("encode %s: %w", path, err)
	}

	if err := file.Close(); err != nil {
		return fmt.Errorf("close %s: %w", path, err)
	}

	return nil
}

// draw renders one icon at supersample then averages it down to size.
func draw(size int, maskable, fullBleed bool) *image.NRGBA {
	big := size * supersample
	canvas := image.NewNRGBA(image.Rect(0, 0, big, big))

	// A maskable icon is full bleed because the platform crops it to its own
	// shape. iOS crops its touch icon too. The rest get the rounded square
	// Android and desktop browsers show as-is.
	for y := range big {
		for x := range big {
			if fullBleed || insideRoundedSquare(x, y, big, big, big/5) {
				canvas.SetNRGBA(x, y, backgroundColor)
			}
		}
	}

	cx, cy := float64(big)/2, float64(big)/2
	rx, ry := float64(big)*0.36, float64(big)*0.24
	if maskable {
		// Keep the mark inside the maskable safe zone so no crop clips it.
		rx, ry = float64(big)*0.28, float64(big)*0.19
	}
	fillEllipse(canvas, cx, cy, rx, ry, accentColor)

	lace := max(1, big/55)
	half := int(rx * 0.46)
	fillRect(canvas, int(cx)-half, int(cy)-lace/2, int(cx)+half, int(cy)+lace/2, inkColor)
	for _, offset := range []float64{-0.26, 0, 0.26} {
		x := int(cx + rx*offset)
		fillRect(canvas, x-lace/2, int(cy)-lace*2, x+lace/2, int(cy)+lace*2, inkColor)
	}

	return downsample(canvas, size)
}

func insideRoundedSquare(x, y, width, height, radius int) bool {
	switch {
	case x >= radius && x < width-radius:
		return true
	case y >= radius && y < height-radius:
		return true
	}

	cornerX := radius
	if x >= width-radius {
		cornerX = width - radius - 1
	}
	cornerY := radius
	if y >= height-radius {
		cornerY = height - radius - 1
	}

	dx, dy := x-cornerX, y-cornerY
	return dx*dx+dy*dy <= radius*radius
}

func fillEllipse(canvas *image.NRGBA, cx, cy, rx, ry float64, fill color.NRGBA) {
	for y := int(cy - ry); y <= int(cy+ry); y++ {
		for x := int(cx - rx); x <= int(cx+rx); x++ {
			dx := (float64(x) - cx) / rx
			dy := (float64(y) - cy) / ry
			if dx*dx+dy*dy <= 1 {
				canvas.SetNRGBA(x, y, fill)
			}
		}
	}
}

func fillRect(canvas *image.NRGBA, x0, y0, x1, y1 int, fill color.NRGBA) {
	bounds := canvas.Bounds()
	for y := max(y0, bounds.Min.Y); y < min(y1, bounds.Max.Y); y++ {
		for x := max(x0, bounds.Min.X); x < min(x1, bounds.Max.X); x++ {
			canvas.SetNRGBA(x, y, fill)
		}
	}
}

func downsample(src *image.NRGBA, size int) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	total := uint32(supersample * supersample)

	for y := range size {
		for x := range size {
			var r, g, b, a uint32
			for dy := range supersample {
				for dx := range supersample {
					c := src.NRGBAAt(x*supersample+dx, y*supersample+dy)
					r += uint32(c.R)
					g += uint32(c.G)
					b += uint32(c.B)
					a += uint32(c.A)
				}
			}

			dst.SetNRGBA(x, y, color.NRGBA{
				R: uint8(r / total),
				G: uint8(g / total),
				B: uint8(b / total),
				A: uint8(a / total),
			})
		}
	}

	return dst
}
