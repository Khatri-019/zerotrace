package ingest

import (
	"context"
	"io"
	"net"

	"go.uber.org/zap"
	"google.golang.org/grpc"
	proto "github.com/zerotrace/zerotrace/proto"
)

type GRPCServer struct {
	proto.UnimplementedTraceIngestServer
	log *zap.Logger
}

func NewGRPCServer(log *zap.Logger) *GRPCServer {
	return &GRPCServer{log: log}
}

func (s *GRPCServer) SendSpans(ctx context.Context, req *proto.SendSpansRequest) (*proto.SendSpansResponse, error) {
	s.log.Debug("received SendSpans request")
	return &proto.SendSpansResponse{Accepted: true}, nil
}

func (s *GRPCServer) StreamSpans(stream proto.TraceIngest_StreamSpansServer) error {
	for {
		batch, err := stream.Recv()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		s.log.Debug("received span batch", zap.Int("spans", len(batch.Spans)))
		// pass to span assembler
		stream.Send(&proto.Ack{SpansAccepted: uint64(len(batch.Spans))})
	}
}

func Start(address string, srv *GRPCServer, log *zap.Logger) (*grpc.Server, error) {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	grpcServer := grpc.NewServer()
	proto.RegisterTraceIngestServer(grpcServer, srv)
	
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("gRPC server failed", zap.Error(err))
		}
	}()
	return grpcServer, nil
}
