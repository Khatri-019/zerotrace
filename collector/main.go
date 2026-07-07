package main

import (
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"go.uber.org/zap"
	"github.com/zerotrace/zerotrace/collector/api"
	"github.com/zerotrace/zerotrace/collector/config"
	"github.com/zerotrace/zerotrace/collector/ingest"
	"github.com/zerotrace/zerotrace/collector/store"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	cfg, err := config.Load("deploy/configs/collector.yaml", log)
	if err != nil {
		log.Fatal("config load failed", zap.Error(err))
	}

	// Setup Storage
	badgerStore, err := store.NewBadgerStore(cfg.Storage.DataPath, time.Duration(cfg.Storage.TTLHours)*time.Hour)
	if err != nil {
		log.Fatal("failed to init storage", zap.Error(err))
	}
	defer badgerStore.Close()

	// Setup gRPC Ingest
	grpcSrv := ingest.NewGRPCServer(log)
	srv, err := ingest.Start(cfg.Ingest.Address, grpcSrv, log)
	if err != nil {
		log.Fatal("failed to start gRPC server", zap.Error(err))
	}
	defer srv.Stop()

	// Setup HTTP API
	r := mux.NewRouter()
	api.SetupREST(r)
	api.SetupWebSocket(r, log)

	log.Info("collector starting", zap.String("grpc", cfg.Ingest.Address), zap.String("http", cfg.API.Address))
	if err := http.ListenAndServe(cfg.API.Address, r); err != nil {
		log.Fatal("http server failed", zap.Error(err))
	}
}
