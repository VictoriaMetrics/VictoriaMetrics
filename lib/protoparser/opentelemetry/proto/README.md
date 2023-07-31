# Opentelemetry proto files

Content copied from https://github.com/open-telemetry/opentelemetry-proto/tree/main/opentelemetry/proto

## Requirements
- protoc binary [link](http://google.github.io/proto-lens/installing-protoc.html)
- golang-proto-gen[link](https://developers.google.com/protocol-buffers/docs/reference/go-generated)
- custom marshaller [link](https://github.com/planetscale/vtprotobuf)

## Modifications

 Original proto files were modified:
1) changed package name for `package opentelemetry`.
2) changed import paths - changed directory names.
3) changed go_package for  `opentelemetry/pb`.


## How to generate pbs

 run command:
 ```bash
export GOBIN=~/go/bin protoc
protoc -I=. --go_out=./lib/protoparser/opentelemetry --go-vtproto_out=./lib/protoparser/opentelemetry --plugin protoc-gen-go-vtproto="$GOBIN/protoc-gen-go-vtproto" --go-vtproto_opt=features=marshal+unmarshal+size  lib/protoparser/opentelemetry/proto/*.proto
 ```

Generated code will be at `lib/protoparser/opentelemetry/opentelemetry/`

 manually edit it:
 
1) remove all external imports
2) remove all unneeded methods
3) replace `unknownFields` with `unknownFields []byte`