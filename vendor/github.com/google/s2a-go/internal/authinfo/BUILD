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
#  Exposes post-handshake authentication and authorization info to the gRPC stack.

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

licenses(["notice"])

package(
    default_visibility = ["//internal:__subpackages__"],
)

go_library(
    name = "authinfo",
    srcs = ["authinfo.go"],
    importpath = "github.com/google/s2a-go/internal/authinfo",
    deps = [
        "//internal/proto/common_go_proto:common_go_proto",
        "//internal/proto/s2a_context_go_proto:s2a_context_go_proto",
        "//internal/proto/s2a_go_proto:s2a_go_proto",
        "@org_golang_google_grpc//credentials:go_default_library",
    ],
)

go_test(
    name = "authinfo_test",
    size = "small",
    srcs = ["authinfo_test.go"],
    embed = [":authinfo"],
    deps = [
        "//internal/proto/common_go_proto:common_go_proto",
        "//internal/proto/s2a_go_proto:s2a_go_proto",
        "@org_golang_google_grpc//credentials:go_default_library",
    ],
)

