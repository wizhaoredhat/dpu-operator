# dpu-operator


# Testing the gRPC using grpCurl

podman run -p 151:50151 quay.io/wizhao/grpc_server:0.0.1

go install github.com/fullstorydev/grpcurl/cmd/grpcurl@latest

```bash
[]# grpcurl --plaintext localhost:151 list
grpc.reflection.v1alpha.ServerReflection
pb.DpuConfigSrv
```

```bash
[]# grpcurl --plaintext localhost:151 list pb.DpuConfigSrv
pb.DpuConfigSrv.GetVersion
```

```bash
[]# grpcurl --plaintext localhost:151 describe pb.DpuConfigSrv
pb.DpuConfigSrv is a service:
service DpuConfigSrv {
  rpc GetVersion ( .pb.VersionRequest ) returns ( .pb.VersionReply );
}
```

```bash
[]# grpcurl --plaintext localhost:151 describe pb.DpuConfigSrv.GetVersion
pb.DpuConfigSrv.GetVersion is a method:
rpc GetVersion ( .pb.VersionRequest ) returns ( .pb.VersionReply );
```

```bash
[]# grpcurl --plaintext localhost:151 describe pb.VersionRequest
pb.VersionRequest is a message:
message VersionRequest {
  string component_name = 1;
}
```

```bash
[]# grpcurl --plaintext -format text -d 'component_name: "DPU"' localhost:151 pb.DpuConfigSrv.GetVersion
message: "Version 0.1 for DPU"
```

# Testing the gRPC with client

```bash
podman run -p 151:50151 quay.io/wizhao/grpc_server:0.0.1
podman run --network host quay.io/wizhao/grpc_client:0.0.1 -grpc_port 151
I1218 21:53:20.138453       1 grpc_client.go:22] Starting main loop gRPC client with port151
2023/12/18 21:53:20 Got version: Version 0.1 for DPU
```
