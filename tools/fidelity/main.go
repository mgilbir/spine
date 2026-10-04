// Command fidelity compares spine's slide renders with reference renders of
// the same deck, such as PowerPoint's or LibreOffice's PNG export, and
// reports how closely each slide matches.
//
//	spine-render -input deck.pptx -out ours -fallback-noto
//	fidelity -ours ours -ref reference -out report
//
// Files pair by the last number in their names, so spine's slide-0001.png
// pairs with PowerPoint's Slide1.png. Ours are resampled to each reference's
// size. Per slide it reports the structural similarity of the two images'
// luma (SSIM over 8 by 8 windows, 1 for identical), their mean absolute
// channel difference (0 to 255) and the share of pixels whose channels
// differ by more than -threshold, and writes a diff image: the reference,
// faded, with those pixels red. report.md and report.json summarize them.
//
// stdlib only; not imported by the library.
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// maxPixels bounds each decoded image.
const maxPixels = 64 << 20

type config struct {
	ours, ref, out string
	threshold      int
}

// Slide is one compared pair.
type Slide struct {
	Number    int     `json:"number"`
	Ours      string  `json:"ours"`
	Reference string  `json:"reference"`
	SSIM      float64 `json:"ssim"`
	MeanError float64 `json:"meanError"`
	Changed   float64 `json:"changed"`
	Diff      string  `json:"diff"`
}

// Report is the comparison of a deck.
type Report struct {
	Threshold int     `json:"threshold"`
	Slides    []Slide `json:"slides"`
	// Unpaired names files without a counterpart.
	Unpaired []string `json:"unpaired,omitempty"`
	MeanSSIM float64  `json:"meanSSIM"`
}

func main() {
	var c config
	flag.StringVar(&c.ours, "ours", "", "directory of spine's PNG renders (required)")
	flag.StringVar(&c.ref, "ref", "", "directory of reference PNG renders (required)")
	flag.StringVar(&c.out, "out", "", "report directory (required; existing files are not overwritten)")
	flag.IntVar(&c.threshold, "threshold", 32, "channel difference, 0 to 255, beyond which a pixel counts as changed")
	flag.Parse()
	if err := run(c); err != nil {
		fmt.Fprintln(os.Stderr, "fidelity:", err)
		os.Exit(1)
	}
}

var lastNumber = regexp.MustCompile(`(\d+)\D*$`)

// pngs maps the PNG files of a directory by the last number in their names.
func pngs(dir string) (map[int]string, []string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, nil, err
	}
	out := map[int]string{}
	var unnumbered []string
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".png") {
			continue
		}
		m := lastNumber.FindStringSubmatch(strings.TrimSuffix(e.Name(), filepath.Ext(e.Name())))
		n, err := strconv.Atoi(firstOr(m, 1))
		if m == nil || err != nil || out[n] != "" {
			unnumbered = append(unnumbered, filepath.Join(dir, e.Name()))
			continue
		}
		out[n] = filepath.Join(dir, e.Name())
	}
	return out, unnumbered, nil
}

func firstOr(m []string, i int) string {
	if len(m) > i {
		return m[i]
	}
	return ""
}

func run(c config) error {
	if c.ours == "" || c.ref == "" || c.out == "" || c.threshold < 0 || c.threshold > 255 {
		return errors.New("require -ours, -ref and -out, and a -threshold from 0 to 255")
	}
	ours, unOurs, err := pngs(c.ours)
	if err != nil {
		return err
	}
	refs, unRefs, err := pngs(c.ref)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(c.out, 0o755); err != nil {
		return err
	}
	report := Report{Threshold: c.threshold, Unpaired: append(unOurs, unRefs...)}
	var numbers []int
	for n := range refs {
		if ours[n] == "" {
			report.Unpaired = append(report.Unpaired, refs[n])
			continue
		}
		numbers = append(numbers, n)
	}
	for n := range ours {
		if refs[n] == "" {
			report.Unpaired = append(report.Unpaired, ours[n])
		}
	}
	sort.Ints(numbers)
	sort.Strings(report.Unpaired)
	if len(numbers) == 0 {
		return errors.New("no slides pair by number")
	}
	total := 0.0
	for _, n := range numbers {
		s, err := compare(n, ours[n], refs[n], c)
		if err != nil {
			return fmt.Errorf("slide %d: %w", n, err)
		}
		report.Slides = append(report.Slides, s)
		total += s.SSIM
	}
	report.MeanSSIM = total / float64(len(report.Slides))
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := create(filepath.Join(c.out, "report.json"), func(w io.Writer) error { _, err := w.Write(append(data, '\n')); return err }); err != nil {
		return err
	}
	if err := create(filepath.Join(c.out, "report.md"), func(w io.Writer) error { return markdown(w, report) }); err != nil {
		return err
	}
	return markdown(os.Stdout, report)
}

// create writes a new file, refusing to overwrite one.
func create(path string, write func(io.Writer) error) (err error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	defer func() {
		if e := f.Close(); err == nil {
			err = e
		}
	}()
	return write(f)
}

func markdown(w io.Writer, r Report) error {
	var b strings.Builder
	fmt.Fprintf(&b, "Mean SSIM %.4f over %d slides; changed means a channel differs by more than %d.\n\n", r.MeanSSIM, len(r.Slides), r.Threshold)
	b.WriteString("| Slide | SSIM | Mean error | Changed | Diff |\n|---:|---:|---:|---:|---|\n")
	for _, s := range r.Slides {
		fmt.Fprintf(&b, "| %d | %.4f | %.2f | %.2f%% | %s |\n", s.Number, s.SSIM, s.MeanError, 100*s.Changed, s.Diff)
	}
	if len(r.Unpaired) > 0 {
		b.WriteString("\nUnpaired: " + strings.Join(r.Unpaired, ", ") + "\n")
	}
	_, err := io.WriteString(w, b.String())
	return err
}

func decode(path string) (_ *image.NRGBA, err error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() {
		if e := f.Close(); err == nil {
			err = e
		}
	}()
	cfg, err := png.DecodeConfig(f)
	if err != nil {
		return nil, err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 || int64(cfg.Width)*int64(cfg.Height) > maxPixels {
		return nil, fmt.Errorf("%s: %d by %d pixels is out of range", path, cfg.Width, cfg.Height)
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	img, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	// Transparency shows white, as a slide's page does.
	out := image.NewNRGBA(img.Bounds().Sub(img.Bounds().Min))
	draw.Draw(out, out.Rect, image.White, image.Point{}, draw.Src)
	draw.Draw(out, out.Rect, img, img.Bounds().Min, draw.Over)
	return out, nil
}

func compare(n int, oursPath, refPath string, c config) (Slide, error) {
	s := Slide{Number: n, Ours: oursPath, Reference: refPath}
	ref, err := decode(refPath)
	if err != nil {
		return s, err
	}
	ours, err := decode(oursPath)
	if err != nil {
		return s, err
	}
	ours = resample(ours, ref.Rect.Dx(), ref.Rect.Dy())
	s.SSIM = ssim(luma(ours), luma(ref), ref.Rect.Dx(), ref.Rect.Dy())
	diff := image.NewNRGBA(ref.Rect)
	sum, changed := 0.0, 0
	for i := 0; i < len(ref.Pix); i += 4 {
		worst := 0
		for k := 0; k < 3; k++ {
			d := int(ours.Pix[i+k]) - int(ref.Pix[i+k])
			if d < 0 {
				d = -d
			}
			sum += float64(d)
			worst = max(worst, d)
		}
		if worst > c.threshold {
			changed++
			copy(diff.Pix[i:i+4], []byte{255, 0, 0, 255})
			continue
		}
		// The reference, faded towards white.
		for k := 0; k < 3; k++ {
			diff.Pix[i+k] = uint8(191 + int(ref.Pix[i+k])/4)
		}
		diff.Pix[i+3] = 255
	}
	pixels := float64(len(ref.Pix) / 4)
	s.MeanError, s.Changed = sum/(3*pixels), float64(changed)/pixels
	s.Diff = filepath.Join(c.out, fmt.Sprintf("diff-%04d.png", n))
	return s, create(s.Diff, func(w io.Writer) error { return png.Encode(w, diff) })
}

// resample scales an image to w by h pixels, each target pixel the average
// of the source area it covers.
func resample(src *image.NRGBA, w, h int) *image.NRGBA {
	sw, sh := src.Rect.Dx(), src.Rect.Dy()
	if sw == w && sh == h {
		return src
	}
	out := image.NewNRGBA(image.Rect(0, 0, w, h))
	fx, fy := float64(sw)/float64(w), float64(sh)/float64(h)
	for y := 0; y < h; y++ {
		y0, y1 := float64(y)*fy, float64(y+1)*fy
		for x := 0; x < w; x++ {
			x0, x1 := float64(x)*fx, float64(x+1)*fx
			var acc [4]float64
			area := 0.0
			for sy := int(y0); sy < int(math.Ceil(y1)) && sy < sh; sy++ {
				cy := math.Min(y1, float64(sy+1)) - math.Max(y0, float64(sy))
				for sx := int(x0); sx < int(math.Ceil(x1)) && sx < sw; sx++ {
					cx := math.Min(x1, float64(sx+1)) - math.Max(x0, float64(sx))
					a := cx * cy
					p := src.Pix[sy*src.Stride+4*sx:]
					for k := 0; k < 4; k++ {
						acc[k] += a * float64(p[k])
					}
					area += a
				}
			}
			q := out.Pix[y*out.Stride+4*x:]
			for k := 0; k < 4; k++ {
				q[k] = uint8(math.Round(acc[k] / area))
			}
		}
	}
	return out
}

func luma(img *image.NRGBA) []float64 {
	out := make([]float64, 0, len(img.Pix)/4)
	for i := 0; i < len(img.Pix); i += 4 {
		out = append(out, 0.299*float64(img.Pix[i])+0.587*float64(img.Pix[i+1])+0.114*float64(img.Pix[i+2]))
	}
	return out
}

// ssim is the mean structural similarity of two luma planes over
// non-overlapping 8 by 8 windows, with the usual constants for 8-bit data.
func ssim(a, b []float64, w, h int) float64 {
	const c1, c2 = (0.01 * 255) * (0.01 * 255), (0.03 * 255) * (0.03 * 255)
	total, windows := 0.0, 0
	for y0 := 0; y0 < h; y0 += 8 {
		for x0 := 0; x0 < w; x0 += 8 {
			var sa, sb, saa, sbb, sab, n float64
			for y := y0; y < min(y0+8, h); y++ {
				for x := x0; x < min(x0+8, w); x++ {
					p, q := a[y*w+x], b[y*w+x]
					sa, sb, saa, sbb, sab, n = sa+p, sb+q, saa+p*p, sbb+q*q, sab+p*q, n+1
				}
			}
			ma, mb := sa/n, sb/n
			va, vb, cov := saa/n-ma*ma, sbb/n-mb*mb, sab/n-ma*mb
			total += ((2*ma*mb + c1) * (2*cov + c2)) / ((ma*ma + mb*mb + c1) * (va + vb + c2))
			windows++
		}
	}
	return total / float64(windows)
}
