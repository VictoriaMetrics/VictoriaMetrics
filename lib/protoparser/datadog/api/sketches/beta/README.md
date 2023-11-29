# Datadog proto files

Content copied from https://github.com/DataDog/agent-payload/blob/master/proto/metrics/agent_payload.proto

## Requirements
- protoc binary [link](http://google.github.io/proto-lens/installing-protoc.html)
- golang-proto-gen[link](https://developers.google.com/protocol-buffers/docs/reference/go-generated)
- custom marshaller [link](https://github.com/planetscale/vtprotobuf)

## Modifications

 Original proto files were modified:
1) changed package name for `package beta`.
2) changed import paths - changed directory names.
3) changed go_package for  `./pb`.


## How to generate pbs

 run command:
 ```bash
export GOBIN=~/go/bin protoc
protoc -I=. --go_out=./lib/protoparser/datadog/api/sketches/beta --go-vtproto_out=./lib/protoparser/datadog/api/sketches/beta --plugin protoc-gen-go-vtproto="$GOBIN/protoc-gen-go-vtproto" --go-vtproto_opt=features=unmarshal lib/protoparser/datadog/api/sketches/beta/proto/*.proto
 ```

 Generated code will be at `lib/protoparser/datadog/api/sketches/beta/pb`

 manually edit it:
 
1) remove all external imports
2) remove all unneeded methods
3) replace `unknownFields` with `unknownFields []byte`
