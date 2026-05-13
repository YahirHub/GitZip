package progress

import (
	"fmt"
	"io"
	"math"
	"strings"
	"sync"
	"time"
)

const defaultWidth = 24

// Bar is a lightweight terminal progress bar rendered in a single compact line.
type Bar struct {
	mu            sync.Mutex
	out           io.Writer
	total         int64
	current       int64
	width         int
	startedAt     time.Time
	lastDraw      time.Time
	lastLineWidth int
}

// New creates a compact progress bar that tracks bytes processed.
func New(out io.Writer, total int64) *Bar {
	bar := &Bar{
		out:       out,
		total:     max(total, 0),
		width:     defaultWidth,
		startedAt: time.Now(),
	}
	bar.render(true)
	return bar
}

// Add increments the processed byte count.
func (b *Bar) Add(n int64) {
	if n <= 0 {
		return
	}

	b.mu.Lock()
	b.current += n
	if b.current > b.total && b.total > 0 {
		b.current = b.total
	}
	b.mu.Unlock()
	b.render(false)
}

// Finish forces a final 100% render and prints a status marker.
func (b *Bar) Finish(success bool) {
	b.mu.Lock()
	if success && b.total > 0 {
		b.current = b.total
	}
	b.mu.Unlock()
	b.render(true)

	status := "✓"
	if !success {
		status = "✗"
	}
	fmt.Fprintf(b.out, " %s\n", status)
}

func (b *Bar) render(force bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	now := time.Now()
	if !force && !b.lastDraw.IsZero() && now.Sub(b.lastDraw) < 60*time.Millisecond {
		return
	}
	b.lastDraw = now

	percent := 1.0
	if b.total > 0 {
		percent = float64(b.current) / float64(b.total)
		if percent < 0 {
			percent = 0
		}
		if percent > 1 {
			percent = 1
		}
	}

	filled := int(math.Round(percent * float64(b.width)))
	if filled > b.width {
		filled = b.width
	}
	if filled < 0 {
		filled = 0
	}

	bar := strings.Repeat("█", filled) + strings.Repeat("░", b.width-filled)

	current := b.current
	if b.total == 0 {
		current = 0
	}

	line := fmt.Sprintf("[%s] %6.2f%% | %s / %s",
		bar,
		percent*100,
		FormatBytes(current),
		FormatBytes(b.total),
	)

	lineWidth := runeCount(line)
	padding := ""
	if b.lastLineWidth > lineWidth {
		padding = strings.Repeat(" ", b.lastLineWidth-lineWidth)
	}
	if lineWidth > b.lastLineWidth {
		b.lastLineWidth = lineWidth
	}

	fmt.Fprintf(b.out, "\r%s%s", line, padding)
}

// CountingReader reports bytes consumed from an underlying reader to a progress bar.
type CountingReader struct {
	reader io.Reader
	bar    *Bar
}

// NewCountingReader wraps reader and updates bar after each successful read.
func NewCountingReader(reader io.Reader, bar *Bar) *CountingReader {
	return &CountingReader{reader: reader, bar: bar}
}

// Read proxies the underlying reader and records consumed bytes.
func (r *CountingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 && r.bar != nil {
		r.bar.Add(int64(n))
	}
	return n, err
}

// FormatBytes formats byte counts in a compact human-readable representation.
func FormatBytes(value int64) string {
	if value < 1024 {
		return fmt.Sprintf("%d B", value)
	}

	units := []string{"KB", "MB", "GB", "TB", "PB"}
	n := float64(value)
	unit := "B"
	for _, candidate := range units {
		n /= 1024
		unit = candidate
		if n < 1024 {
			break
		}
	}

	return fmt.Sprintf("%.2f %s", n, unit)
}

func max(value, minimum int64) int64 {
	if value < minimum {
		return minimum
	}
	return value
}

func runeCount(value string) int {
	return len([]rune(value))
}
