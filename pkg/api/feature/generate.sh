#!/bin/sh -ex

# go-to-protobuf seems unable to run in the package directory -> move to parent dir
cd ..
go-to-protobuf \
   --output-base=. \
   --go-header-file ../../hack/boilerplate.go.txt \
   --proto-import ../../vendor \
   --packages ./feature=feature \
   --keep-gogoproto=false \
   --apimachinery-packages "-k8s.io/apimachinery/pkg/util/intstr"
cd -

# Mangle the go_package option to comply with newer versions protoc-gen-go
sed s',go_package =.*,go_package = "sigs.k8s.io/node-feature-discovery/pkg/api/feature";,' \
    -i generated.proto
