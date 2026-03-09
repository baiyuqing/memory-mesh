package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"
)

func reporter(ctx context.Context, m *metrics, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			we, wl, ws := m.snapshotWindow()
			ts, te, tl, slow := m.snapshotTotal()
			windowQueries := wl.count + we
			totalQueries := ts + te
			elapsed := time.Since(m.startedAt).Seconds()
			wm := readMemStats()
			fmt.Printf("[%s] interval_ok=%d interval_err=%d interval_p95=%.2fms interval_p99=%.2fms interval_max=%.2fms interval_slow_over_%s=%d interval_slow_ratio=%.6f interval_slow_by_conn=%s total_ok=%d total_err=%d total_tps=%.2f total_p95=%.2fms total_p99=%.2fms total_max=%.2fms total_slow_over_%s=%d total_slow_ratio=%.6f mem_alloc_mb=%.2f mem_sys_mb=%.2f\n",
				time.Now().Format(time.RFC3339), wl.count, we, wl.quantile(0.95), wl.quantile(0.99), wl.maxMS,
				ws.threshold, ws.total, ratio(ws.total, windowQueries), ws.formatByConnection(),
				ts, te, float64(ts)/elapsed, tl.quantile(0.95), tl.quantile(0.99), tl.maxMS, slow.threshold, slow.total, ratio(slow.total, totalQueries),
				float64(wm.Alloc)/(1024*1024), float64(wm.Sys)/(1024*1024))
		}
	}
}

func startPrometheusServer(ctx context.Context, cfg config, m *metrics) {
	if cfg.prometheusListen == "" {
		return
	}
	mux := http.NewServeMux()
	mux.HandleFunc(cfg.prometheusPath, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
		writePrometheusMetrics(w, m)
	})

	srv := &http.Server{Addr: cfg.prometheusListen, Handler: mux}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			fmt.Fprintf(os.Stderr, "prometheus server error: %v\n", err)
		}
	}()
	fmt.Printf("Prometheus metrics endpoint listening at http://%s%s\n", cfg.prometheusListen, cfg.prometheusPath)
}

func writePrometheusMetrics(w http.ResponseWriter, m *metrics) {
	ts, te, hs, slow := m.snapshotTotal()
	elapsed := time.Since(m.startedAt).Seconds()
	wm := readMemStats()

	fmt.Fprintln(w, "# HELP mysqlbench_success_total Total successful queries.")
	fmt.Fprintln(w, "# TYPE mysqlbench_success_total counter")
	fmt.Fprintf(w, "mysqlbench_success_total %d\n", ts)

	fmt.Fprintln(w, "# HELP mysqlbench_failure_total Total failed queries.")
	fmt.Fprintln(w, "# TYPE mysqlbench_failure_total counter")
	fmt.Fprintf(w, "mysqlbench_failure_total %d\n", te)

	fmt.Fprintln(w, "# HELP mysqlbench_tps Average TPS since benchmark start.")
	fmt.Fprintln(w, "# TYPE mysqlbench_tps gauge")
	fmt.Fprintf(w, "mysqlbench_tps %.6f\n", float64(ts)/elapsed)

	fmt.Fprintln(w, "# HELP mysqlbench_latency_ms Latency histogram in milliseconds.")
	fmt.Fprintln(w, "# TYPE mysqlbench_latency_ms histogram")
	var cumulative uint64
	for i, c := range hs.bucketCounts {
		cumulative += c
		if i < len(hs.bounds) {
			fmt.Fprintf(w, "mysqlbench_latency_ms_bucket{le=\"%s\"} %d\n", trimFloat(hs.bounds[i]), cumulative)
			continue
		}
		fmt.Fprintf(w, "mysqlbench_latency_ms_bucket{le=\"+Inf\"} %d\n", cumulative)
	}
	fmt.Fprintf(w, "mysqlbench_latency_ms_sum %.3f\n", hs.sumMS)
	fmt.Fprintf(w, "mysqlbench_latency_ms_count %d\n", hs.count)

	fmt.Fprintln(w, "# HELP mysqlbench_latency_spike_threshold_ms Configured latency spike threshold in milliseconds.")
	fmt.Fprintln(w, "# TYPE mysqlbench_latency_spike_threshold_ms gauge")
	fmt.Fprintf(w, "mysqlbench_latency_spike_threshold_ms %.3f\n", float64(slow.threshold.Microseconds())/1000.0)

	fmt.Fprintln(w, "# HELP mysqlbench_latency_spikes_total Total queries above the configured latency spike threshold.")
	fmt.Fprintln(w, "# TYPE mysqlbench_latency_spikes_total counter")
	fmt.Fprintf(w, "mysqlbench_latency_spikes_total %d\n", slow.total)

	fmt.Fprintln(w, "# HELP mysqlbench_latency_spike_ratio Ratio of queries above the configured latency spike threshold.")
	fmt.Fprintln(w, "# TYPE mysqlbench_latency_spike_ratio gauge")
	fmt.Fprintf(w, "mysqlbench_latency_spike_ratio %.6f\n", ratio(slow.total, ts+te))

	fmt.Fprintln(w, "# HELP mysqlbench_connection_latency_spikes_total Total queries above the configured latency spike threshold by connection.")
	fmt.Fprintln(w, "# TYPE mysqlbench_connection_latency_spikes_total counter")
	for i, count := range slow.byConn {
		fmt.Fprintf(w, "mysqlbench_connection_latency_spikes_total{connection=\"%d\"} %d\n", i, count)
	}

	fmt.Fprintln(w, "# HELP mysqlbench_memory_alloc_bytes Current heap allocation in bytes.")
	fmt.Fprintln(w, "# TYPE mysqlbench_memory_alloc_bytes gauge")
	fmt.Fprintf(w, "mysqlbench_memory_alloc_bytes %d\n", wm.Alloc)

	fmt.Fprintln(w, "# HELP mysqlbench_memory_sys_bytes Total bytes obtained from system.")
	fmt.Fprintln(w, "# TYPE mysqlbench_memory_sys_bytes gauge")
	fmt.Fprintf(w, "mysqlbench_memory_sys_bytes %d\n", wm.Sys)
}

func trimFloat(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func readMemStats() runtime.MemStats {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return ms
}
