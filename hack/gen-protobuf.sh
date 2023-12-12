#!/bin/bash -e
echo "Generating protobuf plugin"

protoc --go_out=pkg/plugin/generated/pb --go-grpc_out=pkg/plugin/generated/pb pkg/plugin/protobuf/*.proto

echo "Generated protobuf plugin"