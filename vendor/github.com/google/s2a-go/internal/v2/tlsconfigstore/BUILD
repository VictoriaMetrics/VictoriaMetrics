# Copyright 2022 Google LLC
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

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

package(
  default_visibility = ["//internal/v2:__subpackages__"],
)

# TODO(rmehta19) : Investigate how to use embedsrcs with data in a top level directory. If possible, move example_cert_key under s2a-go/internal/v2

go_library(
  name = "tlsconfigstore",
  srcs = ["tlsconfigstore.go"],
  importpath = "github.com/google/s2a-go/internal/v2/tlsconfigstore",
  embedsrcs = [
    "//internal/v2/tlsconfigstore/example_cert_key:export_testdata", # keep
  ],
  deps = [
    "//internal/proto/common_go_proto:common_go_proto",
    "//internal/proto/v2/common_go_proto:common_go_proto",
    "//internal/proto/v2/s2a_go_proto:s2a_go_proto",
    "//internal/tokenmanager",
    "//internal/v2/certverifier:certverifier",
    "//internal/v2/remotesigner:remotesigner",
    "@org_golang_google_grpc//codes:go_default_library",
    "@org_golang_google_grpc//grpclog:go_default_library",
  ],
)

go_test(
  name = "tlsconfigstore_test",
  srcs = ["tlsconfigstore_test.go"],
  embed = [":tlsconfigstore"],
  embedsrcs = [
    "//internal/v2/tlsconfigstore/example_cert_key:export_testdata", # keep
  ],
  deps = [
    "//internal/proto/common_go_proto:common_go_proto",
    "//internal/proto/v2/s2a_go_proto:s2a_go_proto",
    "//internal/tokenmanager",
    "//internal/v2/fakes2av2",
    "@com_github_google_go_cmp//cmp:go_default_library",
    "@com_github_google_go_cmp//cmp/cmpopts:go_default_library",
    "@org_golang_google_grpc//:go_default_library",
    "@org_golang_google_grpc//credentials/insecure:go_default_library",
    "@org_golang_google_protobuf//testing/protocmp:go_default_library",
  ],
)
