# Copyright 2021 Google LLC
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#    https://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# Description:
#  gRPC tranport credentials that use the S2A.

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

package(
    default_visibility = ["//:__subpackages__"],
)

licenses(["notice"])

exports_files(["LICENSE"])

go_library(
    name = "s2a",
    srcs = [
        "s2a.go",
        "s2a_options.go",
        "s2a_utils.go",
    ],
    importpath = "github.com/google/s2a-go",
    deps = [
        "//fallback:s2a_fallback",
        "//internal/handshaker",
        "//internal/handshaker/service",
        "//internal/proto/common_go_proto",
        "//internal/proto/v2/s2a_go_proto",
        "//internal/tokenmanager",
        "//internal/v2",
        "@com_github_golang_protobuf//proto",
        "@org_golang_google_grpc//credentials:go_default_library",
        "@org_golang_google_grpc//grpclog:go_default_library",
        "@org_golang_google_grpc//peer:go_default_library",
    ],
)

go_test(
    name = "s2a_test",
    size = "small",
    srcs = [
        "s2a_e2e_test.go",
        "s2a_options_test.go",
        "s2a_test.go",
        "s2a_utils_test.go",
    ],
    embed = [":s2a"],
    embedsrcs = [
        "//internal/v2/tlsconfigstore/example_cert_key:export_testdata",  # keep
    ],
    deps = [
        "//internal/fakehandshaker/service",
        "//internal/proto/common_go_proto",
        "//internal/proto/examples/helloworld_go_proto",
        "//internal/proto/s2a_go_proto",
        "//internal/proto/v2/s2a_go_proto",
        "//internal/v2",
        "//internal/v2/fakes2av2",
        "@com_github_google_go_cmp//cmp:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//credentials:go_default_library",
        "@org_golang_google_grpc//grpclog:go_default_library",
        "@org_golang_google_grpc//peer:go_default_library",
        "@org_golang_google_protobuf//testing/protocmp:go_default_library",
    ],
)
