package main

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
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

func TestBuildMySQLDriverConfigOptimizesTCP(t *testing.T) {
	cfg, err := buildMySQLDriverConfig("mock.user:mock_password@tcp(127.0.0.1:3306)/test")
	if err != nil {
		t.Fatalf("buildMySQLDriverConfig returned error: %v", err)
	}
	if cfg.Net != optimizedTCPNetworkName {
		t.Fatalf("net=%q want=%q", cfg.Net, optimizedTCPNetworkName)
	}
}

func TestBuildMySQLDriverConfigLeavesUnixUntouched(t *testing.T) {
	cfg, err := buildMySQLDriverConfig("mock.user:mock_password@unix(/tmp/mysql.sock)/test")
	if err != nil {
		t.Fatalf("buildMySQLDriverConfig returned error: %v", err)
	}
	if cfg.Net != "unix" {
		t.Fatalf("net=%q", cfg.Net)
	}
}

func TestWritePrometheusMetrics(t *testing.T) {
	m := newMetrics(time.Now().Add(-2*time.Second), 2, 2*time.Millisecond)
	m.record(0, 1200*time.Microsecond, nil)
	m.record(1, 3*time.Millisecond, nil)
	m.record(0, 5*time.Millisecond, assertErr{})

	rr := httptest.NewRecorder()
	writePrometheusMetrics(rr, m)
	out := rr.Body.String()

	for _, want := range []string{
		"mysqlbench_success_total 2",
		"mysqlbench_failure_total 1",
		"mysqlbench_latency_ms_count 2",
		"mysqlbench_latency_spike_threshold_ms 2.000",
		"mysqlbench_latency_spikes_total 2",
		"mysqlbench_latency_spike_ratio 0.666667",
		"mysqlbench_connection_latency_spikes_total{connection=\"0\"} 1",
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
	m.record(1, 500*time.Millisecond, assertErr{})

	windowErrs, windowHist, windowSlow := m.snapshotWindow()
	if windowErrs != 1 {
		t.Fatalf("windowErrs=%d", windowErrs)
	}
	if windowHist.count != 3 {
		t.Fatalf("windowCount=%d", windowHist.count)
	}
	if windowSlow.total != 3 {
		t.Fatalf("windowSlowTotal=%d", windowSlow.total)
	}
	if got := windowSlow.formatByConnection(); got != "conn0:1,conn1:1,conn2:1" {
		t.Fatalf("windowSlowByConnection=%q", got)
	}
	if got := ratio(windowSlow.total, windowHist.count+windowErrs); got != 0.75 {
		t.Fatalf("windowSlowRatio=%v", got)
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
	if totalSlow.total != 3 {
		t.Fatalf("totalSlow=%d", totalSlow.total)
	}
	if got := totalSlow.formatByConnection(); got != "conn0:1,conn1:1,conn2:1" {
		t.Fatalf("totalSlowByConnection=%q", got)
	}
	if got := ratio(totalSlow.total, totalOK+totalErrs); got != 0.75 {
		t.Fatalf("totalSlowRatio=%v", got)
	}
}

func TestWorkerIgnoresShutdownContextError(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	m := newMetrics(time.Now(), 1, 200*time.Millisecond)

	run := func(ctx context.Context) error {
		cancel()
		return ctx.Err()
	}

	worker(ctx, 0, run, m)

	totalOK, totalErrs, totalHist, totalSlow := m.snapshotTotal()
	if totalOK != 0 || totalErrs != 0 || totalHist.count != 0 || totalSlow.total != 0 {
		t.Fatalf("unexpected totals: ok=%d errs=%d count=%d slow=%d", totalOK, totalErrs, totalHist.count, totalSlow.total)
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

func TestIsValidConnectionMode(t *testing.T) {
	tests := []struct {
		name string
		mode string
		want bool
	}{
		{"long-running", "long-running", true},
		{"per-transaction", "per-transaction", true},
		{"empty", "", false},
		{"invalid", "invalid", false},
		{"case sensitive", "Long-Running", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidConnectionMode(tt.mode); got != tt.want {
				t.Fatalf("isValidConnectionMode(%q) = %v, want %v", tt.mode, got, tt.want)
			}
		})
	}
}

func TestTrimMatchingQuotes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"single quotes", "'hello'", "hello"},
		{"double quotes", `"hello"`, "hello"},
		{"mismatched quotes", "'hello\"", "'hello\""},
		{"too short", "x", "x"},
		{"empty", "", ""},
		{"no quotes", "no quotes", "no quotes"},
		{"empty between quotes", "''", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := trimMatchingQuotes(tt.input); got != tt.want {
				t.Fatalf("trimMatchingQuotes(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeFlagArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "combined dsn split and unquoted",
			args: []string{"-dsn 'user:pass@tcp(host)/db'", "-duration", "1s"},
			want: []string{"-dsn", "user:pass@tcp(host)/db", "-duration", "1s"},
		},
		{
			name: "non-dsn args pass through",
			args: []string{"-duration", "1s"},
			want: []string{"-duration", "1s"},
		},
		{
			name: "combined dsn no quotes",
			args: []string{"-dsn value"},
			want: []string{"-dsn", "value"},
		},
		{
			name: "non-dsn combined args untouched",
			args: []string{"-other flag"},
			want: []string{"-other flag"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := normalizeFlagArgs(tt.args)
			if len(got) != len(tt.want) {
				t.Fatalf("normalizeFlagArgs(%v) = %v, want %v", tt.args, got, tt.want)
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Fatalf("normalizeFlagArgs(%v)[%d] = %q, want %q", tt.args, i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestParseFlagsValidation(t *testing.T) {
	originalArgs := os.Args
	originalFlagSet := flag.CommandLine
	defer func() {
		os.Args = originalArgs
		flag.CommandLine = originalFlagSet
	}()

	tests := []struct {
		name string
		args []string
	}{
		{
			name: "concurrency zero",
			args: []string{"mysqlbench", "-concurrency", "0", "-duration", "1s", "-report-interval", "1s"},
		},
		{
			name: "duration zero",
			args: []string{"mysqlbench", "-duration", "0s", "-report-interval", "1s"},
		},
		{
			name: "empty dsn",
			args: []string{"mysqlbench", "-dsn", "", "-duration", "1s", "-report-interval", "1s"},
		},
		{
			name: "empty query",
			args: []string{"mysqlbench", "-query", "", "-duration", "1s", "-report-interval", "1s"},
		},
		{
			name: "prometheus-path without leading slash",
			args: []string{"mysqlbench", "-prometheus-listen", ":9090", "-prometheus-path", "metrics", "-duration", "1s", "-report-interval", "1s"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			flag.CommandLine = flag.NewFlagSet(tt.args[0], flag.ContinueOnError)
			os.Args = tt.args
			if _, err := parseFlags(); err == nil {
				t.Fatal("expected error but got nil")
			}
		})
	}
}

func TestHistogramSnapshotAndReset(t *testing.T) {
	h := newHistogram([]float64{1, 5, 10})
	h.observe(0.5)
	h.observe(3.0)
	h.observe(7.0)

	t.Run("snapshot has correct values", func(t *testing.T) {
		s := h.snapshotAndReset()
		if s.count != 3 {
			t.Fatalf("count=%d, want 3", s.count)
		}
		if s.sumMS == 0 {
			t.Fatal("sumMS should be non-zero")
		}
		if s.maxMS == 0 {
			t.Fatal("maxMS should be non-zero")
		}
	})

	t.Run("histogram zeroed after reset", func(t *testing.T) {
		s := h.snapshotAndReset()
		if s.count != 0 {
			t.Fatalf("count=%d after reset, want 0", s.count)
		}
		if s.sumMS != 0 {
			t.Fatalf("sumMS=%v after reset, want 0", s.sumMS)
		}
		if s.maxMS != 0 {
			t.Fatalf("maxMS=%v after reset, want 0", s.maxMS)
		}
	})
}

func TestHistogramObserveMax(t *testing.T) {
	t.Run("ascending order", func(t *testing.T) {
		h := newHistogram([]float64{10, 100})
		h.observe(1)
		h.observe(5)
		h.observe(50)
		s := h.snapshot()
		if s.maxMS != 50 {
			t.Fatalf("maxMS=%v, want 50", s.maxMS)
		}
	})

	t.Run("descending order", func(t *testing.T) {
		h := newHistogram([]float64{10, 100})
		h.observe(50)
		h.observe(5)
		h.observe(1)
		s := h.snapshot()
		if s.maxMS != 50 {
			t.Fatalf("maxMS=%v, want 50", s.maxMS)
		}
	})
}

func TestHistogramQuantileEdgeCases(t *testing.T) {
	t.Run("empty histogram", func(t *testing.T) {
		h := newHistogram([]float64{1, 5, 10})
		s := h.snapshot()
		if got := s.quantile(0.5); got != 0 {
			t.Fatalf("quantile(0.5) on empty = %v, want 0", got)
		}
	})

	t.Run("single observation", func(t *testing.T) {
		h := newHistogram([]float64{1, 5, 10})
		h.observe(3.0)
		s := h.snapshot()
		got := s.quantile(0.5)
		if got == 0 {
			t.Fatal("quantile(0.5) on single observation should be non-zero")
		}
	})
}

func TestQuantileEdgeCases(t *testing.T) {
	tests := []struct {
		name   string
		sorted []float64
		q      float64
		want   float64
	}{
		{"empty slice", nil, 0.5, 0},
		{"q<=0 returns first", []float64{10, 20, 30}, 0, 10},
		{"q>=1 returns last", []float64{10, 20, 30}, 1, 30},
		{"single element q=0", []float64{42}, 0, 42},
		{"single element q=0.5", []float64{42}, 0.5, 42},
		{"single element q=1", []float64{42}, 1, 42},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := quantile(tt.sorted, tt.q); got != tt.want {
				t.Fatalf("quantile(%v, %v) = %v, want %v", tt.sorted, tt.q, got, tt.want)
			}
		})
	}
}

func TestRatioEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		numerator   uint64
		denominator uint64
		want        float64
	}{
		{"denominator zero", 5, 0, 0},
		{"half", 1, 2, 0.5},
		{"three quarters", 3, 4, 0.75},
		{"numerator zero", 0, 10, 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ratio(tt.numerator, tt.denominator); got != tt.want {
				t.Fatalf("ratio(%d, %d) = %v, want %v", tt.numerator, tt.denominator, got, tt.want)
			}
		})
	}
}

func TestLoadAtomicSlice(t *testing.T) {
	slice := make([]atomic.Uint64, 3)
	slice[0].Store(10)
	slice[1].Store(20)
	slice[2].Store(30)

	got := loadAtomicSlice(slice)
	want := []uint64{10, 20, 30}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("loadAtomicSlice[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func TestSwapAtomicSlice(t *testing.T) {
	slice := make([]atomic.Uint64, 3)
	slice[0].Store(10)
	slice[1].Store(20)
	slice[2].Store(30)

	t.Run("returns original values", func(t *testing.T) {
		got := swapAtomicSlice(slice)
		want := []uint64{10, 20, 30}
		for i := range got {
			if got[i] != want[i] {
				t.Fatalf("swapAtomicSlice[%d] = %d, want %d", i, got[i], want[i])
			}
		}
	})

	t.Run("zeroed after swap", func(t *testing.T) {
		for i := range slice {
			if v := slice[i].Load(); v != 0 {
				t.Fatalf("slice[%d] = %d after swap, want 0", i, v)
			}
		}
	})
}

func TestSlowSnapshotFormatByConnection(t *testing.T) {
	tests := []struct {
		name   string
		byConn []uint64
		want   string
	}{
		{"all zeros", []uint64{0, 0, 0}, "-"},
		{"mixed", []uint64{1, 0, 3}, "conn0:1,conn2:3"},
		{"single connection", []uint64{5}, "conn0:5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := slowSnapshot{byConn: tt.byConn}
			if got := s.formatByConnection(); got != tt.want {
				t.Fatalf("formatByConnection() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestMetricsRecordEdgeCases(t *testing.T) {
	t.Run("out of bounds connectionID no panic", func(t *testing.T) {
		m := newMetrics(time.Now(), 2, 1*time.Millisecond)
		m.record(-1, 5*time.Millisecond, nil)
		m.record(99, 5*time.Millisecond, nil)

		totalOK, _, _, totalSlow := m.snapshotTotal()
		if totalOK != 2 {
			t.Fatalf("totalOK=%d, want 2", totalOK)
		}
		if totalSlow.total != 2 {
			t.Fatalf("totalSlow=%d, want 2", totalSlow.total)
		}
		if got := totalSlow.formatByConnection(); got != "-" {
			t.Fatalf("formatByConnection()=%q, want %q (out-of-bounds should not record per-conn)", got, "-")
		}
	})

	t.Run("failure records error not histogram", func(t *testing.T) {
		m := newMetrics(time.Now(), 1, 200*time.Millisecond)
		m.record(0, 1*time.Millisecond, assertErr{})

		totalOK, totalErrs, totalHist, _ := m.snapshotTotal()
		if totalOK != 0 {
			t.Fatalf("totalOK=%d, want 0", totalOK)
		}
		if totalErrs != 1 {
			t.Fatalf("totalErrs=%d, want 1", totalErrs)
		}
		if totalHist.count != 0 {
			t.Fatalf("histogram count=%d, want 0 (errors should not be in histogram)", totalHist.count)
		}
	})

	t.Run("duration equal to threshold not slow", func(t *testing.T) {
		m := newMetrics(time.Now(), 1, 200*time.Millisecond)
		m.record(0, 200*time.Millisecond, nil)

		_, _, _, totalSlow := m.snapshotTotal()
		if totalSlow.total != 0 {
			t.Fatalf("totalSlow=%d, want 0 (duration == threshold should not count as slow)", totalSlow.total)
		}
	})
}

// --- Fake driver infrastructure for runQuery tests ---

type fakeConnector struct {
	conn *fakeConn
}

func (fc *fakeConnector) Connect(_ context.Context) (driver.Conn, error) {
	return fc.conn, nil
}

func (fc *fakeConnector) Driver() driver.Driver {
	return fakeDriverImpl{}
}

type fakeDriverImpl struct{}

func (fakeDriverImpl) Open(_ string) (driver.Conn, error) {
	return nil, errors.New("use Connector")
}

type fakeConn struct {
	beginErr  error
	queryFunc func(query string) (driver.Rows, error)
	execFunc  func(query string) (driver.Result, error)
	commitErr error
}

func (c *fakeConn) Prepare(_ string) (driver.Stmt, error) {
	return nil, errors.New("not implemented")
}

func (c *fakeConn) Close() error { return nil }

func (c *fakeConn) Begin() (driver.Tx, error) {
	if c.beginErr != nil {
		return nil, c.beginErr
	}
	return &fakeTxDriver{conn: c}, nil
}

func (c *fakeConn) QueryContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Rows, error) {
	if c.queryFunc != nil {
		return c.queryFunc(query)
	}
	return &fakeRows{columns: []string{"result"}, closed: false}, nil
}

func (c *fakeConn) ExecContext(_ context.Context, query string, _ []driver.NamedValue) (driver.Result, error) {
	if c.execFunc != nil {
		return c.execFunc(query)
	}
	return fakeResult{}, nil
}

type fakeTxDriver struct {
	conn *fakeConn
}

func (t *fakeTxDriver) Commit() error {
	if t.conn.commitErr != nil {
		return t.conn.commitErr
	}
	return nil
}

func (t *fakeTxDriver) Rollback() error { return nil }

type fakeRows struct {
	columns []string
	data    [][]driver.Value
	pos     int
	closed  bool
}

func (r *fakeRows) Columns() []string {
	return r.columns
}

func (r *fakeRows) Close() error {
	r.closed = true
	return nil
}

func (r *fakeRows) Next(dest []driver.Value) error {
	if r.pos >= len(r.data) {
		return io.EOF
	}
	copy(dest, r.data[r.pos])
	r.pos++
	return nil
}

type fakeResult struct{}

func (fakeResult) LastInsertId() (int64, error) { return 0, nil }
func (fakeResult) RowsAffected() (int64, error) { return 1, nil }

func openFakeDB(conn *fakeConn) *sql.DB {
	return sql.OpenDB(&fakeConnector{conn: conn})
}

// --- runQuery tests ---

func TestRunQuerySelectSuccess(t *testing.T) {
	conn := &fakeConn{
		queryFunc: func(_ string) (driver.Rows, error) {
			return &fakeRows{
				columns: []string{"id", "name"},
				data:    [][]driver.Value{{int64(1), "test"}},
			}, nil
		},
	}
	db := openFakeDB(conn)
	defer db.Close()

	if err := runQuery(context.Background(), db, "SELECT id, name FROM t"); err != nil {
		t.Fatalf("runQuery returned error: %v", err)
	}
}

func TestRunQueryBeginTxError(t *testing.T) {
	conn := &fakeConn{
		beginErr: errors.New("begin failed"),
	}
	db := openFakeDB(conn)
	defer db.Close()

	err := runQuery(context.Background(), db, "SELECT 1")
	if err == nil {
		t.Fatal("expected error from BeginTx")
	}
	if !strings.Contains(err.Error(), "begin failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunQueryExecFallback(t *testing.T) {
	execCalled := false
	conn := &fakeConn{
		queryFunc: func(_ string) (driver.Rows, error) {
			return nil, errors.New("query does not return rows")
		},
		execFunc: func(_ string) (driver.Result, error) {
			execCalled = true
			return fakeResult{}, nil
		},
	}
	db := openFakeDB(conn)
	defer db.Close()

	if err := runQuery(context.Background(), db, "INSERT INTO t VALUES(1)"); err != nil {
		t.Fatalf("runQuery returned error: %v", err)
	}
	if !execCalled {
		t.Fatal("expected Exec fallback to be called")
	}
}

func TestRunQueryExecFallbackError(t *testing.T) {
	conn := &fakeConn{
		queryFunc: func(_ string) (driver.Rows, error) {
			return nil, errors.New("query does not return rows")
		},
		execFunc: func(_ string) (driver.Result, error) {
			return nil, errors.New("exec failed")
		},
	}
	db := openFakeDB(conn)
	defer db.Close()

	err := runQuery(context.Background(), db, "INSERT INTO t VALUES(1)")
	if err == nil {
		t.Fatal("expected error from Exec fallback")
	}
	if !strings.Contains(err.Error(), "exec failed") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunQueryQueryError(t *testing.T) {
	conn := &fakeConn{
		queryFunc: func(_ string) (driver.Rows, error) {
			return nil, errors.New("syntax error")
		},
	}
	db := openFakeDB(conn)
	defer db.Close()

	err := runQuery(context.Background(), db, "BAD SQL")
	if err == nil {
		t.Fatal("expected error from Query")
	}
	if !strings.Contains(err.Error(), "syntax error") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestShouldIgnoreQueryResultEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		makeCtx func() (context.Context, context.CancelFunc)
		err     error
		want    bool
	}{
		{
			name:    "nil error active ctx",
			makeCtx: func() (context.Context, context.CancelFunc) { return context.WithCancel(context.Background()) },
			err:     nil,
			want:    false,
		},
		{
			name:    "non-context error active ctx",
			makeCtx: func() (context.Context, context.CancelFunc) { return context.WithCancel(context.Background()) },
			err:     errors.New("some error"),
			want:    false,
		},
		{
			name: "context.Canceled with canceled ctx",
			makeCtx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
			err:  context.Canceled,
			want: true,
		},
		{
			name: "context.DeadlineExceeded with expired ctx",
			makeCtx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
				return ctx, cancel
			},
			err:  context.DeadlineExceeded,
			want: true,
		},
		{
			name: "wrapped context.Canceled with canceled ctx",
			makeCtx: func() (context.Context, context.CancelFunc) {
				ctx, cancel := context.WithCancel(context.Background())
				cancel()
				return ctx, cancel
			},
			err:  fmt.Errorf("query failed: %w", context.Canceled),
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := tt.makeCtx()
			defer cancel()
			if got := shouldIgnoreQueryResult(ctx, tt.err); got != tt.want {
				t.Fatalf("shouldIgnoreQueryResult() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWorkerRecordsMetrics(t *testing.T) {
	t.Run("records successes then stops", func(t *testing.T) {
		m := newMetrics(time.Now(), 1, 200*time.Millisecond)
		callCount := 0
		ctx, cancel := context.WithCancel(context.Background())
		run := func(_ context.Context) error {
			callCount++
			if callCount >= 3 {
				cancel()
				return context.Canceled
			}
			return nil
		}

		worker(ctx, 0, run, m)

		totalOK, totalErrs, _, _ := m.snapshotTotal()
		if totalOK != 2 {
			t.Fatalf("totalOK=%d, want 2", totalOK)
		}
		if totalErrs != 0 {
			t.Fatalf("totalErrs=%d, want 0", totalErrs)
		}
	})

	t.Run("records failures", func(t *testing.T) {
		m := newMetrics(time.Now(), 1, 200*time.Millisecond)
		callCount := 0
		ctx, cancel := context.WithCancel(context.Background())
		run := func(_ context.Context) error {
			callCount++
			if callCount >= 3 {
				cancel()
				return context.Canceled
			}
			return errors.New("query failed")
		}

		worker(ctx, 0, run, m)

		totalOK, totalErrs, _, _ := m.snapshotTotal()
		if totalOK != 0 {
			t.Fatalf("totalOK=%d, want 0", totalOK)
		}
		if totalErrs != 2 {
			t.Fatalf("totalErrs=%d, want 2", totalErrs)
		}
	})
}

// --- Dialer helper tests ---

func TestIsTCPNetwork(t *testing.T) {
	tests := []struct {
		name    string
		network string
		want    bool
	}{
		{"tcp", "tcp", true},
		{"tcp4", "tcp4", true},
		{"tcp6", "tcp6", true},
		{"unix", "unix", false},
		{"empty", "", false},
		{"udp", "udp", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isTCPNetwork(tt.network); got != tt.want {
				t.Fatalf("isTCPNetwork(%q) = %v, want %v", tt.network, got, tt.want)
			}
		})
	}
}

func TestOptimizedTCPNetworkNameFor(t *testing.T) {
	tests := []struct {
		name     string
		network  string
		wantName string
		wantOK   bool
	}{
		{"tcp", "tcp", "mysqlbench-tcp", true},
		{"tcp4", "tcp4", "mysqlbench-tcp4", true},
		{"tcp6", "tcp6", "mysqlbench-tcp6", true},
		{"unix", "unix", "", false},
		{"empty", "", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotName, gotOK := optimizedTCPNetworkNameFor(tt.network)
			if gotName != tt.wantName || gotOK != tt.wantOK {
				t.Fatalf("optimizedTCPNetworkNameFor(%q) = (%q, %v), want (%q, %v)", tt.network, gotName, gotOK, tt.wantName, tt.wantOK)
			}
		})
	}
}

func TestBuildMySQLDriverConfigEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		dsn     string
		wantNet string
		wantErr bool
	}{
		{"invalid DSN", "://bad", "", true},
		{"tcp gets optimized", "user:pass@tcp(127.0.0.1:3306)/db", optimizedTCPNetworkName, false},
		{"tcp4 gets optimized", "user:pass@tcp4(127.0.0.1:3306)/db", optimizedTCP4NetworkName, false},
		{"tcp6 gets optimized", "user:pass@tcp6([::1]:3306)/db", optimizedTCP6NetworkName, false},
		{"unix unchanged", "user:pass@unix(/tmp/mysql.sock)/db", "unix", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg, err := buildMySQLDriverConfig(tt.dsn)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error but got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.Net != tt.wantNet {
				t.Fatalf("net=%q, want %q", cfg.Net, tt.wantNet)
			}
		})
	}
}

// --- Reporting helper tests ---

func TestTrimFloat(t *testing.T) {
	tests := []struct {
		name  string
		input float64
		want  string
	}{
		{"integer 1", 1.0, "1"},
		{"fractional 0.25", 0.25, "0.25"},
		{"integer 100", 100.0, "100"},
		{"small fraction", 0.001, "0.001"},
		{"large integer", 10000.0, "10000"},
		{"one and a half", 1.5, "1.5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := trimFloat(tt.input); got != tt.want {
				t.Fatalf("trimFloat(%v) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestWritePrometheusMetricsZeroState(t *testing.T) {
	m := newMetrics(time.Now().Add(-1*time.Second), 1, 200*time.Millisecond)

	rr := httptest.NewRecorder()
	writePrometheusMetrics(rr, m)
	out := rr.Body.String()

	for _, want := range []string{
		"mysqlbench_success_total 0",
		"mysqlbench_failure_total 0",
		"mysqlbench_latency_ms_count 0",
		"mysqlbench_latency_spikes_total 0",
		"# HELP mysqlbench_success_total",
		"# TYPE mysqlbench_success_total",
		"# HELP mysqlbench_failure_total",
		"# TYPE mysqlbench_failure_total",
		"# HELP mysqlbench_tps",
		"# TYPE mysqlbench_tps",
		"# HELP mysqlbench_latency_ms",
		"# TYPE mysqlbench_latency_ms",
	} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in output:\n%s", want, out)
		}
	}
}

func TestWritePrometheusMetricsAllMetricNames(t *testing.T) {
	m := newMetrics(time.Now().Add(-2*time.Second), 2, 2*time.Millisecond)
	m.record(0, 1*time.Millisecond, nil)
	m.record(1, 5*time.Millisecond, nil)
	m.record(0, 3*time.Millisecond, assertErr{})

	rr := httptest.NewRecorder()
	writePrometheusMetrics(rr, m)
	out := rr.Body.String()

	for _, prefix := range []string{
		"mysqlbench_success_total",
		"mysqlbench_failure_total",
		"mysqlbench_tps",
		"mysqlbench_latency_ms_bucket",
		"mysqlbench_latency_ms_sum",
		"mysqlbench_latency_ms_count",
		"mysqlbench_latency_spike_threshold_ms",
		"mysqlbench_latency_spikes_total",
		"mysqlbench_latency_spike_ratio",
		"mysqlbench_connection_latency_spikes_total",
		"mysqlbench_memory_alloc_bytes",
		"mysqlbench_memory_sys_bytes",
	} {
		if !strings.Contains(out, prefix) {
			t.Fatalf("missing metric %q in output:\n%s", prefix, out)
		}
	}
}

func TestStartPrometheusServerNoOp(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg := config{prometheusListen: ""}
	m := newMetrics(time.Now(), 1, 200*time.Millisecond)

	// Should return immediately without panic or error.
	startPrometheusServer(ctx, cfg, m)
}

