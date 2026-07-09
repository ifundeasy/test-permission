package seed

import (
	"fmt"
	"time"
)

// Progress prints seeding progress: phase, rows done/total, rate, ETA.
// Printed every `every` records (default ≈ 25 batches of 1000).
type Progress struct {
	label string
	every int

	phase string
	total int
	done  int
	start time.Time
}

func NewProgress(label string, every int) *Progress {
	if every <= 0 {
		every = 25_000
	}
	return &Progress{label: label, every: every}
}

func (p *Progress) Begin(phase string, total int) {
	p.phase, p.total, p.done, p.start = phase, total, 0, time.Now()
	fmt.Printf("[%s] phase=%s starting (~%s rows)\n", p.label, phase, human(total))
}

func (p *Progress) Add(n int) {
	p.done += n
	if p.done%p.every < n {
		elapsed := time.Since(p.start).Seconds()
		rate := float64(p.done) / max(elapsed, 0.001)
		eta := "-"
		if p.total > p.done && rate > 0 {
			eta = (time.Duration(float64(p.total-p.done)/rate) * time.Second).Truncate(time.Second).String()
		}
		fmt.Printf("[%s] phase=%s %s/%s rows  %s rows/s  ETA %s\n",
			p.label, p.phase, human(p.done), human(p.total), human(int(rate)), eta)
	}
}

func (p *Progress) End() {
	elapsed := time.Since(p.start).Truncate(time.Millisecond)
	fmt.Printf("[%s] phase=%s DONE: %s rows in %s\n", p.label, p.phase, human(p.done), elapsed)
}

func human(n int) string {
	switch {
	case n >= 1_000_000:
		return fmt.Sprintf("%.2fM", float64(n)/1_000_000)
	case n >= 1_000:
		return fmt.Sprintf("%.1fk", float64(n)/1_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}
