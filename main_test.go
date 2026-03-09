package main

import (
	"flag"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

func TestQuantile(t *testing.T) {
	vals := []float64{1, 2, 3, 4, 5}
	if got := quantile(vals, 0.95); got != 5 {
		t.Fatalf("q95 got %v", got)
	}
	if got := quantile(vals, 0.99); got != 5 {
		t.Fatalf("q99 got %v", got)
	}
	if got := quantile(vals, 0.5); got != 3 {
		t.Fatalf("q50 got %v", got)
	}
}

func TestHistogramQuantile(t *testing.T) {
	h := newHistogram([]float64{1, 2, 5})
	h.observe(0.8)
	h.observe(1.2)
	h.observe(10)

	s := h.snapshot()
	if s.count != 3 {
		t.Fatalf("count=%d", s.count)
	}
	if got := s.quantile(0.95); got != 10 {
		t.Fatalf("q95=%v", got)
	}
}

func TestWritePrometheusMetrics(t *testing.T) {
	m := newMetrics(time.Now().Add(-2*time.Second), 2, 2*time.Millisecond)
	m.record(0, 1200*time.Microsecond, nil)
	m.record(1, 3*time.Millisecond, nil)
	m.record(0, 0, assertErr{})

	rr := httptest.NewRecorder()
	writePrometheusMetrics(rr, m)
	out := rr.Body.String()

	for _, want := range []string{
		"mysqlbench_success_total 2",
		"mysqlbench_failure_total 1",
		"mysqlbench_latency_ms_count 2",
		"mysqlbench_latency_spike_threshold_ms 2.000",
		"mysqlbench_latency_spikes_total 1",
		"mysqlbench_connection_latency_spikes_total{connection=\"0\"} 0",
		"mysqlbench_connection_latency_spikes_total{connection=\"1\"} 1",
		"mysqlbench_memory_alloc_bytes",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in output: %s", want, out)
		}
	}
}

func TestMetricsSlowCounts(t *testing.T) {
	m := newMetrics(time.Now(), 3, 200*time.Millisecond)
	m.record(0, 250*time.Millisecond, nil)
	m.record(0, 199*time.Millisecond, nil)
	m.record(2, 350*time.Millisecond, nil)
	m.record(1, 0, assertErr{})

	windowErrs, windowHist, windowSlow := m.snapshotWindow()
	if windowErrs != 1 {
		t.Fatalf("windowErrs=%d", windowErrs)
	}
	if windowHist.count != 3 {
		t.Fatalf("windowCount=%d", windowHist.count)
	}
	if windowSlow.total != 2 {
		t.Fatalf("windowSlowTotal=%d", windowSlow.total)
	}
	if got := windowSlow.formatByConnection(); got != "conn0:1,conn2:1" {
		t.Fatalf("windowSlowByConnection=%q", got)
	}

	windowErrs, windowHist, windowSlow = m.snapshotWindow()
	if windowErrs != 0 || windowHist.count != 0 || windowSlow.total != 0 {
		t.Fatalf("window reset failed: errs=%d count=%d slow=%d", windowErrs, windowHist.count, windowSlow.total)
	}

	totalOK, totalErrs, totalHist, totalSlow := m.snapshotTotal()
	if totalOK != 3 || totalErrs != 1 {
		t.Fatalf("totals ok=%d errs=%d", totalOK, totalErrs)
	}
	if totalHist.count != 3 {
		t.Fatalf("totalCount=%d", totalHist.count)
	}
	if totalSlow.total != 2 {
		t.Fatalf("totalSlow=%d", totalSlow.total)
	}
	if got := totalSlow.formatByConnection(); got != "conn0:1,conn2:1" {
		t.Fatalf("totalSlowByConnection=%q", got)
	}
}

type assertErr struct{}

func (assertErr) Error() string { return "boom" }

func TestParseFlagsConnectionMode(t *testing.T) {
	originalArgs := os.Args
	originalFlagSet := flag.CommandLine
	defer func() {
		os.Args = originalArgs
		flag.CommandLine = originalFlagSet
	}()

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	os.Args = []string{"mysqlbench", "-connection-mode", connectionModePerTxn, "-duration", "1s", "-report-interval", "1s", "-slow-threshold", "200ms"}
	cfg, err := parseFlags()
	if err != nil {
		t.Fatalf("parseFlags returned error: %v", err)
	}
	if cfg.connectionMode != connectionModePerTxn {
		t.Fatalf("connectionMode=%q", cfg.connectionMode)
	}
	if cfg.slowThreshold != 200*time.Millisecond {
		t.Fatalf("slowThreshold=%s", cfg.slowThreshold)
	}

	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	os.Args = []string{"mysqlbench", "-connection-mode", "bad-mode", "-duration", "1s", "-report-interval", "1s"}
	if _, err := parseFlags(); err == nil {
		t.Fatal("expected invalid connection-mode error")
	}
}

func TestParseFlagsDSNCombinedArgument(t *testing.T) {
	originalArgs := os.Args
	originalFlagSet := flag.CommandLine
	defer func() {
		os.Args = originalArgs
		flag.CommandLine = originalFlagSet
	}()

	wantDSN := "mock.user:mock_password@tcp(127.0.0.1:3306)/test"
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	os.Args = []string{
		"mysqlbench",
		"-dsn '" + wantDSN + "'",
		"-duration", "1s",
		"-report-interval", "1s",
	}

	cfg, err := parseFlags()
	if err != nil {
		t.Fatalf("parseFlags returned error: %v", err)
	}
	if cfg.dsn != wantDSN {
		t.Fatalf("dsn=%q want=%q", cfg.dsn, wantDSN)
	}
	if gotUser := strings.SplitN(cfg.dsn, ":", 2)[0]; gotUser != "mock.user" {
		t.Fatalf("username=%q", gotUser)
	}
}

func TestParseFlagsDSNQuotedValue(t *testing.T) {
	originalArgs := os.Args
	originalFlagSet := flag.CommandLine
	defer func() {
		os.Args = originalArgs
		flag.CommandLine = originalFlagSet
	}()

	wantDSN := "mock.user:mock_password@tcp(127.0.0.1:3306)/test"
	flag.CommandLine = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	os.Args = []string{
		"mysqlbench",
		"-dsn", "'" + wantDSN + "'",
		"-duration", "1s",
		"-report-interval", "1s",
	}

	cfg, err := parseFlags()
	if err != nil {
		t.Fatalf("parseFlags returned error: %v", err)
	}
	if cfg.dsn != wantDSN {
		t.Fatalf("dsn=%q want=%q", cfg.dsn, wantDSN)
	}
}

func TestFormatDSNForLog(t *testing.T) {
	got, err := formatDSNForLog("mock.user:mock_password@tcp(127.0.0.1:3306)/test?parseTime=true&tls=true")
	if err != nil {
		t.Fatalf("formatDSNForLog returned error: %v", err)
	}

	for _, want := range []string{
		`user="mock.user"`,
		`password_set=true`,
		`net="tcp"`,
		`addr="127.0.0.1:3306"`,
		`db="test"`,
		`parse_time=true`,
		`tls="true"`,
		`params=-`,
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in %q", want, got)
		}
	}
	if strings.Contains(got, "mock_password") {
		t.Fatalf("password should be redacted, got %q", got)
	}
}

func TestFormatDSNForLogInvalid(t *testing.T) {
	if _, err := formatDSNForLog("://bad"); err == nil {
		t.Fatal("expected parse error for invalid dsn")
	}
}
