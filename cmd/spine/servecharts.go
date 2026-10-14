package main

import (
	"bytes"
	"context"
	"fmt"
	"image"
	"image/png"
	"time"

	"github.com/mgilbir/aster"
	"github.com/mgilbir/spine/internal/renderfonts"
	"github.com/mgilbir/spine/render"
)

// chartMemory bounds what one chart's Vega specification may make the
// renderer hold. The same bound and renderer as spine-render's.
const chartMemory = 256 << 20

// chartRenderer draws charts with aster, which loads nothing from outside
// the specification, and uses the mapped fonts, falling back to its embedded
// Liberation Sans. Each chart is bounded by chartMemory and by timeout; the
// renderer cannot be interrupted sooner. close releases it.
func chartRenderer(fonts []renderfonts.File, timeout time.Duration) (render.ChartRenderer, func() error, error) {
	opts := []aster.Option{aster.WithLoader(aster.DenyLoader{}), aster.WithMemoryLimit(chartMemory), aster.WithTimeout(timeout)}
	var copied int64
	for _, f := range fonts {
		program := f.Program()
		if f.Face != nil {
			// Each face copies out the tables it shares with the others.
			if copied += int64(len(program)); copied > renderfonts.Budget {
				return nil, nil, fmt.Errorf("chart fonts copied out of font collections exceed 32 MiB budget; map fewer faces or use -charts=false")
			}
		}
		opts = append(opts, aster.WithFont(f.Family, program))
	}
	conv, err := aster.New(opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("chart renderer: %w", err)
	}
	draw := func(ctx context.Context, spec []byte, scale float64) (image.Image, error) {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		data, err := conv.VegaToPNG(spec, aster.WithScale(scale))
		if err != nil {
			return nil, err
		}
		return png.Decode(bytes.NewReader(data))
	}
	return draw, conv.Close, nil
}
