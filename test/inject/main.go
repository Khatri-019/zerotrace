// inject — sends realistic multi-service traces to the ZeroTrace collector via gRPC.
// This populates the Service Map with real edge data even before the eBPF agent is running.
//
// Usage:
//
//	cd test/inject && go run . --target localhost:4317 --traces 80 --loops 3
package main

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	mathrand "math/rand"
	"time"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	proto "github.com/zerotrace/zerotrace/proto"
)

// ── Topology definition ───────────────────────────────────────────────────────

// Edge represents a call from one service to another.
type Edge struct {
	from    string
	to      string
	latency time.Duration
	errRate float64
}

// Realistic microservices call graph:
//
//	browser → frontend → api-gateway → auth-service
//	                   → product-service → postgres
//	                   → cart-service → redis
//	                   → order-service → postgres
//	                                   → payment-gw
var topology = []Edge{
	{"browser",         "frontend",        12 * time.Millisecond, 0.01},
	{"frontend",        "api-gateway",      3 * time.Millisecond, 0.005},
	{"api-gateway",     "auth-service",     8 * time.Millisecond, 0.02},
	{"api-gateway",     "product-service",  5 * time.Millisecond, 0.01},
	{"api-gateway",     "cart-service",     6 * time.Millisecond, 0.01},
	{"api-gateway",     "order-service",   10 * time.Millisecond, 0.03},
	{"product-service", "postgres",         4 * time.Millisecond, 0.005},
	{"cart-service",    "redis",            1 * time.Millisecond, 0.001},
	{"order-service",   "postgres",         5 * time.Millisecond, 0.005},
	{"order-service",   "payment-gw",      45 * time.Millisecond, 0.05},
}

// ── ID helpers ────────────────────────────────────────────────────────────────

func newID(n int) string {
	b := make([]byte, n)
	cryptorand.Read(b) //nolint:errcheck
	return hex.EncodeToString(b)
}

// ── Span building ─────────────────────────────────────────────────────────────

func jitter(base time.Duration) time.Duration {
	factor := 0.7 + 0.6*mathrand.Float64()
	return time.Duration(float64(base) * factor)
}

func buildSpanPair(traceID, parentSpanID string, edge Edge, wallStart time.Time) []*proto.Span {
	startNs := wallStart.UnixNano()
	dur := jitter(edge.latency)
	endNs := startNs + dur.Nanoseconds()
	isError := mathrand.Float64() < edge.errRate

	statusCode := "200"
	if isError {
		statusCode = "500"
	}

	clientSpanID := newID(8)
	serverSpanID := newID(8)

	clientSpan := &proto.Span{
		TraceId:       traceID,
		SpanId:        clientSpanID,
		ParentSpanId:  parentSpanID,
		ServiceName:   edge.from,
		OperationName: fmt.Sprintf("POST /api/%s", edge.to),
		StartTimeNs:   startNs,
		EndTimeNs:     endNs,
		Tags: map[string]string{
			"peer.address":     fmt.Sprintf("%s:80", edge.to),
			"local.address":    fmt.Sprintf("%s:0", edge.from),
			"process.comm":     edge.from,
			"http.method":      "POST",
			"http.path":        fmt.Sprintf("/api/%s", edge.to),
			"http.status_code": statusCode,
			"bytes.sent":       fmt.Sprintf("%d", 200+mathrand.Intn(1800)),
			"bytes.received":   fmt.Sprintf("%d", 100+mathrand.Intn(900)),
			"tls":              "true",
		},
		Kind: proto.SpanKind_SPAN_KIND_CLIENT,
	}

	serverSpan := &proto.Span{
		TraceId:       traceID,
		SpanId:        serverSpanID,
		ParentSpanId:  clientSpanID,
		ServiceName:   edge.to,
		OperationName: fmt.Sprintf("POST /api/%s", edge.to),
		StartTimeNs:   startNs + 500_000, // 0.5ms network RTT
		EndTimeNs:     endNs - 500_000,
		Tags: map[string]string{
			"peer.address":     fmt.Sprintf("%s:0", edge.from),
			"local.address":    fmt.Sprintf("%s:80", edge.to),
			"process.comm":     edge.to,
			"http.status_code": statusCode,
		},
		Kind: proto.SpanKind_SPAN_KIND_SERVER,
	}

	return []*proto.Span{clientSpan, serverSpan}
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	target := flag.String("target", "localhost:4317", "Collector gRPC address")
	tracesN := flag.Int("traces", 80, "Number of traces to inject per loop")
	loops := flag.Int("loops", 1, "Number of injection loops (0 = infinite)")
	flag.Parse()

	log, _ := zap.NewDevelopment()
	defer log.Sync() //nolint:errcheck

	conn, err := grpc.NewClient(*target,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatal("grpc dial failed", zap.String("target", *target), zap.Error(err))
	}
	defer conn.Close()

	client := proto.NewTraceIngestClient(conn)

	log.Info("injecting traces",
		zap.String("target", *target),
		zap.Int("traces_per_loop", *tracesN),
		zap.Int("loops", *loops),
	)

	inject := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var spans []*proto.Span
		now := time.Now()
		entryEdges := topology[:4] // browser→frontend→api-gw→*

		for i := 0; i < *tracesN; i++ {
			traceID := newID(16)
			t := now.Add(-time.Duration(mathrand.Intn(300)) * time.Second)

			// Always start with a full request path: entry → api-gateway hop
			entry := entryEdges[mathrand.Intn(len(entryEdges))]
			pair := buildSpanPair(traceID, "", entry, t)
			spans = append(spans, pair...)
			parentID := pair[1].SpanId // server span of entry becomes parent

			// Add 1-4 downstream hops
			for _, edge := range topology[4:] {
				if mathrand.Float64() < 0.65 {
					downPair := buildSpanPair(traceID, parentID, edge, t.Add(jitter(5*time.Millisecond)))
					spans = append(spans, downPair...)
				}
			}
		}

		resp, err := client.SendSpans(ctx, &proto.SendSpansRequest{
			Batch: &proto.SpanBatch{
				AgentId: "inject-tool",
				Host:    "localhost",
				Spans:   spans,
			},
		})
		if err != nil {
			log.Error("SendSpans failed", zap.Error(err))
			return
		}
		log.Info("injected",
			zap.Int("spans", len(spans)),
			zap.Bool("accepted", resp.GetAccepted()),
		)
	}

	if *loops == 0 {
		for {
			inject()
			time.Sleep(5 * time.Second)
		}
	}
	for i := 0; i < *loops; i++ {
		inject()
	}
	log.Info("done — open http://localhost:5173/graph to see the service map")
}
