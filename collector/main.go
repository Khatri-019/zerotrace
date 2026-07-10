package main

import (
	"context"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"

	"github.com/zerotrace/zerotrace/collector/api"
	"github.com/zerotrace/zerotrace/collector/config"
	"github.com/zerotrace/zerotrace/collector/graph"
	"github.com/zerotrace/zerotrace/collector/ingest"
	"github.com/zerotrace/zerotrace/collector/store"
	proto "github.com/zerotrace/zerotrace/proto"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	// ── Config ──────────────────────────────────────────────────────────────
	cfg, err := config.Load("deploy/configs/collector.yaml", log)
	if err != nil {
		log.Fatal("config load failed", zap.Error(err))
	}
	log.Info("config loaded",
		zap.String("grpc", cfg.GRPC.Address),
		zap.String("http", cfg.HTTP.Address),
		zap.String("data", cfg.Storage.Path),
	)

	// ── Storage ──────────────────────────────────────────────────────────────
	ttl := time.Duration(cfg.Retention.Hours) * time.Hour
	badgerStore, err := store.NewBadgerStore(cfg.Storage.Path, ttl)
	if err != nil {
		log.Fatal("failed to init BadgerDB", zap.Error(err))
	}
	defer badgerStore.Close()

	traceIndex := store.NewIndex(1000)

	// ── Dependency graph ────────────────────────────────────────────────────
	depGraph := graph.NewDependencyGraph()

	// ── WebSocket hub ────────────────────────────────────────────────────────
	wsHub := api.NewHub(log)

	// ── gRPC ingest server ───────────────────────────────────────────────────
	grpcSrv := ingest.NewGRPCServer(
		log,
		badgerStore,
		traceIndex,
		depGraph,
		func(spans []*proto.Span) { wsHub.BroadcastSpans(spans) },
	)
	grpcServer, err := ingest.Start(cfg.GRPC.Address, grpcSrv, log)
	if err != nil {
		log.Fatal("failed to start gRPC server", zap.Error(err))
	}
	defer grpcServer.GracefulStop()

	// ── REST + WebSocket HTTP server ─────────────────────────────────────────
	restHandler := api.NewRESTHandler(badgerStore, traceIndex, depGraph, log)
	r := mux.NewRouter()
	api.SetupREST(r, restHandler)
	api.SetupWebSocket(r, wsHub, log)

	httpSrv := &http.Server{
		Addr:         cfg.HTTP.Address,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// ── Graceful shutdown ────────────────────────────────────────────────────
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		log.Info("HTTP server listening", zap.String("address", cfg.HTTP.Address))
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal("HTTP server failed", zap.Error(err))
		}
	}()

	// BadgerDB GC runs periodically in the background
	go func() {
		ticker := time.NewTicker(5 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				badgerStore.RunGC()
			}
		}
	}()

	log.Info("zerotrace-collector ready",
		zap.String("grpc", cfg.GRPC.Address),
		zap.String("http", cfg.HTTP.Address),
	)
	<-ctx.Done()

	log.Info("shutting down...")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	log.Info("shutdown complete")
}
