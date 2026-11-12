// Package botanbench makes Botan's `botan speed Whirlpool` readable as Go
// benchmarks, so abbench can interleave it with this module's own
// BenchmarkSum1K / BenchmarkSum64K and benchstat can compare them.
//
// Each benchmark runs `botan speed` once per buffer size (a Botan figure is a
// rate over its own --msec window, not a per-iteration time) and reports ns/op
// for one buffer, the same unit as the benchmarks of the module. Run it with
//
//	go test -c ./internal/botanbench && ./botanbench.test -test.bench . -test.run '^$'
package botanbench

import (
	"flag"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"sync"
	"testing"
)

var msec = flag.Int("botan.msec", 500, "botan speed --msec")

var rate = regexp.MustCompile(`Whirlpool hash buffer size ([0-9]+) bytes: ([0-9.]+) MiB/sec`)

var (
	mu   sync.Mutex
	done = map[int]float64{}
)

func measure(size int) (float64, error) {
	mu.Lock()
	defer mu.Unlock()
	if v, ok := done[size]; ok {
		return v, nil
	}
	out, err := exec.Command("botan", "speed", "--buf-size="+strconv.Itoa(size),
		"--msec="+strconv.Itoa(*msec), "Whirlpool").Output()
	if err != nil {
		return 0, err
	}
	m := rate.FindStringSubmatch(string(out))
	if m == nil {
		return 0, fmt.Errorf("unexpected botan output: %q", out)
	}
	mib, _ := strconv.ParseFloat(m[2], 64)
	v := float64(size) / (mib * 1048576) * 1e9
	done[size] = v
	return v, nil
}

func bench(b *testing.B, size int) {
	ns, err := measure(size)
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < b.N; i++ {
	}
	b.ReportMetric(ns, "ns/op")
}

func BenchmarkSum1K(b *testing.B)  { bench(b, 1024) }
func BenchmarkSum64K(b *testing.B) { bench(b, 65536) }
