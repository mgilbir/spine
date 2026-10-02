package render

import (
	"context"
	"fmt"
	"math"
	"strings"
)

type curve struct {
	op  byte
	pts [3]point
}

func flattenCurves(ctx context.Context, curves []curve, scale float64, remaining *int) ([]edge, error) {
	var edges []edge
	var start, current point
	open := false
	line := func(next point) error {
		if current != next {
			if *remaining <= 0 {
				return fmt.Errorf("%w: flattened glyph edges", ErrLimit)
			}
			*remaining--
			edges = append(edges, edge{current, next})
		}
		current = next
		return nil
	}
	closePath := func() error {
		if open {
			if err := line(start); err != nil {
				return err
			}
		}
		open = false
		return nil
	}
	var cubic func(point, point, point, point, int) error
	cubic = func(a, b, c, d point, depth int) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		if distance(b, a, d) <= 1.0/16 && distance(c, a, d) <= 1.0/16 {
			return line(d)
		}
		if depth >= 24 {
			return fmt.Errorf("%w: glyph curve subdivision", ErrLimit)
		}
		ab, bc, cd := midpoint(a, b), midpoint(b, c), midpoint(c, d)
		abc, bcd := midpoint(ab, bc), midpoint(bc, cd)
		center := midpoint(abc, bcd)
		if err := cubic(a, ab, abc, center, depth+1); err != nil {
			return err
		}
		return cubic(center, bcd, cd, d, depth+1)
	}
	for _, s := range curves {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		pts := s.pts
		for i := range pts {
			pts[i].x *= scale
			pts[i].y *= scale
		}
		switch s.op {
		case 'M':
			if err := closePath(); err != nil {
				return nil, err
			}
			current, start, open = pts[0], pts[0], true
		case 'L':
			if err := line(pts[0]); err != nil {
				return nil, err
			}
		case 'Q':
			b := point{current.x + (pts[0].x-current.x)*2/3, current.y + (pts[0].y-current.y)*2/3}
			c := point{pts[1].x + (pts[0].x-pts[1].x)*2/3, pts[1].y + (pts[0].y-pts[1].y)*2/3}
			if err := cubic(current, b, c, pts[1], 0); err != nil {
				return nil, err
			}
		case 'C':
			if err := cubic(current, pts[0], pts[1], pts[2], 0); err != nil {
				return nil, err
			}
		}
	}
	if err := closePath(); err != nil {
		return nil, err
	}
	return edges, nil
}

func midpoint(a, b point) point { return point{(a.x + b.x) / 2, (a.y + b.y) / 2} }
func distance(p, a, b point) float64 {
	dx, dy := b.x-a.x, b.y-a.y
	if dx == 0 && dy == 0 {
		return math.Hypot(p.x-a.x, p.y-a.y)
	}
	t := math.Max(0, math.Min(1, ((p.x-a.x)*dx+(p.y-a.y)*dy)/(dx*dx+dy*dy)))
	return math.Hypot(p.x-(a.x+t*dx), p.y-(a.y+t*dy))
}

func curveString(ctx context.Context, curves []curve, scale float64, limit int64) (string, error) {
	var b strings.Builder
	open := false
	for _, s := range curves {
		if err := ctx.Err(); err != nil {
			return "", err
		}
		data := string(s.op)
		if s.op == 'M' {
			if open {
				data = "Z " + data
			}
			open = true
		}
		n := 1
		if s.op == 'Q' {
			n = 2
		}
		if s.op == 'C' {
			n = 3
		}
		for i := 0; i < n; i++ {
			data += number(s.pts[i].x*scale) + " " + number(s.pts[i].y*scale) + " "
		}
		if int64(len(data)) > limit-int64(b.Len()) {
			return "", fmt.Errorf("%w: glyph SVG bytes", ErrLimit)
		}
		_, _ = b.WriteString(data)
	}
	if open {
		if int64(b.Len()) >= limit {
			return "", fmt.Errorf("%w: glyph SVG bytes", ErrLimit)
		}
		_ = b.WriteByte('Z')
	}
	return b.String(), nil
}
