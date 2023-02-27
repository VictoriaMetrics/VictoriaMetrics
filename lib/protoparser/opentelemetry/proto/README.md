# Opentelemetry proto files

Content copied from https://github.com/open-telemetry/opentelemetry-proto/tree/main/opentelemetry/proto

## Requirements
- protoc binary [link](http://google.github.io/proto-lens/installing-protoc.html)
- golang-proto-gen[link](https://developers.google.com/protocol-buffers/docs/reference/go-generated)

## Modifications

 Original proto files were modified:
1) changed package name for `package opentelemetry`.
2) changed import paths - changed directory names.
3) changed go_package for  `opentelemetry/pb`.


## How to generate pbs

 run command:
 ```bash
protoc -I=. --go_out=lib/protoparser/ lib/protoparser/opentelemetry/proto/*.proto
 ```