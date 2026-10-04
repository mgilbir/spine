package render

import (
	"context"
	"fmt"
	"image"
	"math"
	"math/bits"

	"github.com/mgilbir/forme/style"
)

const pathSamples = 8

type paintCommand struct {
	d     drawing
	edges []edge
	clips [][]edge
	// sampler filters an image drawing while it paints.
	sampler *imageSampler
}

// begin prepares a command to paint the pixels x0 to x1 of its rows.
func (c *paintCommand) begin(scale float64, x0, x1 int) {
	if c.d.image != nil {
		c.sampler = newImageSampler(c.d, scale, x0, x1)
	}
}

// beginRow prepares row y.
func (c *paintCommand) beginRow(y int) {
	if c.sampler != nil {
		c.sampler.beginRow(y)
	}
}

// color is the command's paint at a pixel of the current row.
func (c *paintCommand) color(x, y int, scale float64) style.RGBA {
	if c.sampler != nil {
		return c.sampler.row[x-c.sampler.x0]
	}
	return c.d.pixelColor(x, y, scale)
}

func (p *Page) paintCommands(ctx context.Context, scale float64, width, height int) ([]paintCommand, error) {
	remaining := p.limits.MaxPathSegments
	cache := map[*geometry][]edge{}
	get := func(g *geometry) ([]edge, error) {
		if edges, ok := cache[g]; ok {
			return edges, nil
		}
		edges, err := flatten(ctx, g, scale, &remaining)
		if err == nil {
			cache[g] = edges
		}
		return edges, err
	}
	var commands []paintCommand
	var visits, checks int64
	for _, d := range p.draws {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for _, g := range d.clips {
			d.rect = meet(d.rect, g.bounds)
		}
		cmd := paintCommand{d: d}
		var err error
		if d.path != nil {
			cmd.edges, err = get(d.path)
			if err != nil {
				return nil, err
			}
		}
		for _, g := range d.clips {
			edges, err := get(g)
			if err != nil {
				return nil, err
			}
			cmd.clips = append(cmd.clips, edges)
		}
		x0, y0, x1, y1 := pixelBounds(d.rect, scale, width, height)
		n := int64(x1-x0) * int64(y1-y0)
		if n > p.limits.MaxPixelVisits-visits {
			return nil, fmt.Errorf("%w: pixel visits", ErrLimit)
		}
		visits += n
		if d.path != nil || len(d.clips) > 0 {
			// Charge scan/sort work conservatively, plus per-pixel coverage.
			rows := int64(y1-y0) * pathSamples
			for _, edges := range append([][]edge{cmd.edges}, cmd.clips...) {
				perRow := int64(len(edges)) * int64(bits.Len(uint(len(edges)))+2)
				if perRow != 0 && rows > (p.limits.MaxEdgeChecks-checks)/perRow {
					return nil, fmt.Errorf("%w: edge checks", ErrLimit)
				}
				checks += rows * perRow
			}
			if n > (p.limits.MaxEdgeChecks-checks)/pathSamples {
				return nil, fmt.Errorf("%w: path coverage", ErrLimit)
			}
			checks += n * pathSamples
		}
		commands = append(commands, cmd)
	}
	return commands, nil
}

// Vertical supersampling with exact horizontal interval coverage. Clip
// intervals are intersected before coverage, never multiplied as alpha masks.
func paintPath(ctx context.Context, img *image.RGBA, cmd paintCommand, scale float64) error {
	r := cmd.d.rect
	x0, y0, x1, y1 := pixelBounds(r, scale, img.Bounds().Dx(), img.Bounds().Dy())
	coverage := make([]float64, x1-x0)
	var scratch []crossing
	cmd.begin(scale, x0, x1)
	for y := y0; y < y1; y++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		cmd.beginRow(y)
		clear(coverage)
		for sample := 0; sample < pathSamples; sample++ {
			sy := float64(y) + (float64(sample)+0.5)/pathSamples
			if sy < r.y0*scale || sy >= r.y1*scale {
				continue
			}
			spans := []interval{{r.x0 * scale, r.x1 * scale}}
			if cmd.d.path != nil {
				var fill []interval
				fill, scratch = scan(cmd.edges, sy, scratch, cmd.d.path.nonzero)
				spans = intersect(spans, fill)
			}
			for _, edges := range cmd.clips {
				var clip []interval
				clip, scratch = scan(edges, sy, scratch, false)
				spans = intersect(spans, clip)
			}
			for _, s := range spans {
				// Intersect before conversion, including paths far off-page.
				lo, hi := math.Max(float64(x0), s.lo), math.Min(float64(x1), s.hi)
				if hi <= lo {
					continue
				}
				for x := int(math.Floor(lo)); x < int(math.Ceil(hi)); x++ {
					if (x-x0)%1024 == 0 {
						if err := ctx.Err(); err != nil {
							return err
						}
					}
					coverage[x-x0] += (math.Min(float64(x+1), hi) - math.Max(float64(x), lo)) / pathSamples
				}
			}
		}
		for x := x0; x < x1; x++ {
			if (x-x0)%1024 == 0 {
				if err := ctx.Err(); err != nil {
					return err
				}
			}
			c := cmd.color(x, y, scale)
			a := math.Min(1, coverage[x-x0]) * c.A
			i := y*img.Stride + x*4
			img.Pix[i] = uint8(math.Round(c.R*a + float64(img.Pix[i])*(1-a)))
			img.Pix[i+1] = uint8(math.Round(c.G*a + float64(img.Pix[i+1])*(1-a)))
			img.Pix[i+2] = uint8(math.Round(c.B*a + float64(img.Pix[i+2])*(1-a)))
			img.Pix[i+3] = uint8(math.Round(255*a + float64(img.Pix[i+3])*(1-a)))
		}
	}
	return nil
}
