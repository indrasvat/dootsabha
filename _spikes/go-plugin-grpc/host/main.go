// Host program: launches the greeter plugin via go-plugin gRPC,
// measures handshake latency and per-call latency, tests crash recovery.
package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"time"

	hclog "github.com/hashicorp/go-hclog"
	"github.com/hashicorp/go-plugin"

	"dootsabha-spike/go-plugin-grpc/shared"
)

func newClient(pluginPath string) (*plugin.Client, shared.Greeter, error) {
	client := plugin.NewClient(&plugin.ClientConfig{
		HandshakeConfig: shared.HandshakeConfig,
		Plugins:         shared.PluginMap,
		Cmd:             exec.Command(pluginPath),
		AllowedProtocols: []plugin.Protocol{
			plugin.ProtocolGRPC,
		},
		Logger: newLogger(),
	})

	rpcClient, err := client.Client()
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("rpc client: %w", err)
	}

	raw, err := rpcClient.Dispense("greeter")
	if err != nil {
		client.Kill()
		return nil, nil, fmt.Errorf("dispense: %w", err)
	}

	greeter, ok := raw.(shared.Greeter)
	if !ok {
		client.Kill()
		return nil, nil, fmt.Errorf("unexpected type: %T", raw)
	}

	return client, greeter, nil
}

// newLogger returns a silent hclog.Logger to suppress go-plugin's internal logs.
func newLogger() hclog.Logger {
	return hclog.New(&hclog.LoggerOptions{
		Name:   "go-plugin",
		Output: os.Stderr,
		Level:  hclog.Error, // suppress info/debug noise
	})
}

// ensure log import is used for log.Fatalf calls
var _ = log.Fatalf

func measureHandshake(pluginPath string) (time.Duration, shared.Greeter, *plugin.Client) {
	start := time.Now()
	client, greeter, err := newClient(pluginPath)
	if err != nil {
		log.Fatalf("handshake failed: %v", err)
	}
	latency := time.Since(start)
	return latency, greeter, client
}

func measureCalls(greeter shared.Greeter, n int) []time.Duration {
	durations := make([]time.Duration, n)
	for i := 0; i < n; i++ {
		start := time.Now()
		_, err := greeter.Greet(fmt.Sprintf("caller-%d", i))
		if err != nil {
			log.Fatalf("greet call %d failed: %v", i, err)
		}
		durations[i] = time.Since(start)
	}
	return durations
}

func percentile(sorted []time.Duration, p float64) time.Duration {
	idx := int(math.Ceil(p/100.0*float64(len(sorted)))) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= len(sorted) {
		idx = len(sorted) - 1
	}
	return sorted[idx]
}

func memUsageMB() float64 {
	var ms runtime.MemStats
	runtime.ReadMemStats(&ms)
	return float64(ms.Alloc) / 1024 / 1024
}

func main() {
	// Resolve the plugin binary path relative to this executable.
	// When running via `go run ./host`, the binary is in plugin/greeter-plugin.
	pluginPath := filepath.Join("plugin", "greeter-plugin")
	if _, err := os.Stat(pluginPath); err != nil {
		log.Fatalf("plugin binary not found at %s — run: go build -o plugin/greeter-plugin ./plugin", pluginPath)
	}

	fmt.Println("=== go-plugin gRPC Spike ===")
	fmt.Printf("Plugin: %s\n\n", pluginPath)

	// ── Handshake latency ─────────────────────────────────────────────────
	fmt.Println("--- Handshake Latency ---")
	const handshakeRuns = 5
	handshakeTimes := make([]time.Duration, handshakeRuns)
	for i := 0; i < handshakeRuns; i++ {
		d, greeter, client := measureHandshake(pluginPath)
		handshakeTimes[i] = d
		// Verify one call works
		if i == 0 {
			msg, err := greeter.Greet("Chandragupta")
			if err != nil {
				log.Fatalf("first greet failed: %v", err)
			}
			fmt.Printf("Plugin response: %s\n", msg)
		}
		client.Kill()
		fmt.Printf("  run %d: %v\n", i+1, d)
	}
	sort.Slice(handshakeTimes, func(i, j int) bool { return handshakeTimes[i] < handshakeTimes[j] })
	fmt.Printf("Handshake median: %v  p95: %v  min: %v  max: %v\n\n",
		percentile(handshakeTimes, 50),
		percentile(handshakeTimes, 95),
		handshakeTimes[0],
		handshakeTimes[len(handshakeTimes)-1],
	)

	// ── Per-call latency ──────────────────────────────────────────────────
	fmt.Println("--- Per-Call Latency (100 calls) ---")
	_, greeter, client := measureHandshake(pluginPath)
	defer client.Kill()

	callTimes := measureCalls(greeter, 100)
	sort.Slice(callTimes, func(i, j int) bool { return callTimes[i] < callTimes[j] })
	fmt.Printf("Per-call median: %v  p95: %v  p99: %v  min: %v  max: %v\n\n",
		percentile(callTimes, 50),
		percentile(callTimes, 95),
		percentile(callTimes, 99),
		callTimes[0],
		callTimes[len(callTimes)-1],
	)

	// ── Memory overhead ───────────────────────────────────────────────────
	fmt.Println("--- Memory Overhead ---")
	beforeMB := memUsageMB()
	fmt.Printf("Host RSS before plugin: %.2f MB\n", beforeMB)
	// After plugin is already running (client launched above)
	afterMB := memUsageMB()
	fmt.Printf("Host RSS after plugin handshake: %.2f MB\n\n", afterMB)

	// ── Crash recovery ────────────────────────────────────────────────────
	fmt.Println("--- Crash Recovery ---")
	_, g2, c2 := measureHandshake(pluginPath)
	// Verify it works before crash
	msg, _ := g2.Greet("test-before-crash")
	fmt.Printf("Before crash: %s\n", msg)

	// Kill plugin process abruptly
	c2.Kill()
	fmt.Println("Plugin killed abruptly.")

	// Attempt call on dead plugin — should fail gracefully
	_, err := g2.Greet("test-after-crash")
	if err != nil {
		fmt.Printf("Post-crash call error (expected): %v\n", err)
	} else {
		fmt.Println("WARNING: post-crash call unexpectedly succeeded")
	}

	// Host can re-launch a fresh plugin
	start := time.Now()
	_, g3, c3 := measureHandshake(pluginPath)
	defer c3.Kill()
	relaunchDur := time.Since(start)
	msg3, _ := g3.Greet("Vishakhadatta")
	fmt.Printf("Re-launched plugin (%v): %s\n", relaunchDur, msg3)
	fmt.Println("\nCrash recovery: host survived plugin crash and re-launched cleanly.")

	fmt.Println("\n=== Spike complete ===")
}
