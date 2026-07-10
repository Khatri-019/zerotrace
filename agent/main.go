package main

import (
	"context"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"go.uber.org/zap"
	"github.com/zerotrace/zerotrace/agent/config"
	"github.com/zerotrace/zerotrace/agent/correlator"
	"github.com/zerotrace/zerotrace/agent/exporter"
	"github.com/zerotrace/zerotrace/agent/loader"
	"github.com/zerotrace/zerotrace/agent/reader"
	proto "github.com/zerotrace/zerotrace/proto"
)

func main() {
	log, _ := zap.NewProduction()
	defer log.Sync()

	cfg, err := config.Load("deploy/configs/agent.yaml", log)
	if err != nil {
		log.Fatal("config load failed", zap.Error(err))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	mgr, err := loader.New(cfg, log)
	if err != nil {
		log.Fatal("BPF loader failed", zap.Error(err))
	}
	defer mgr.Close()

	eventCh := make(chan reader.RawEvent, 100000)
	go reader.PollRingBuffer(ctx, mgr.RingBufReader(), eventCh, log)

	var wgCorrelator sync.WaitGroup
	wgCorrelator.Add(1)

	spanCh := make(chan *proto.Span, 10000)

	go func() {
		defer wgCorrelator.Done()
		correlator.Run(ctx, eventCh, spanCh, log)
	}()

	var wgExporter sync.WaitGroup
	wgExporter.Add(1)

	exp, err := exporter.New(cfg.Collector.Address, log)
	if err != nil {
		log.Fatal("exporter init failed", zap.Error(err))
	}
	defer exp.Close()

	go func() {
		defer wgExporter.Done()
		exp.Run(spanCh, cfg.Export.BatchSize, cfg.Export.FlushIntervalMS)
	}()

	go loader.ScanAndAttachUprobes(ctx, mgr, cfg, log)

	log.Info("zerotrace-agent running", zap.String("collector", cfg.Collector.Address))
	<-ctx.Done()
	log.Info("shutting down, waiting for components to flush...")
	
	// Wait for correlator to finish flushing
	wgCorrelator.Wait()
	close(spanCh)

	// Wait for exporter to finish draining
	done := make(chan struct{})
	go func() {
		wgExporter.Wait()
		close(done)
	}()
	select {
	case <-done:
		log.Info("shutdown complete")
	case <-time.After(5 * time.Second):
		log.Warn("shutdown timed out")
	}
}
