#!/bin/bash
export PATH=$PATH:/usr/local/go/bin:~/go/bin
go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
mkdir -p agent/proto collector/proto
protoc --go_out=agent/proto --go-grpc_out=agent/proto --go_out=collector/proto --go-grpc_out=collector/proto proto/zerotrace.proto
