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
#  Utility for calling the S2A.

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

licenses(["notice"])

package(
    default_visibility = ["//:__subpackages__"],
)

go_library(
    name = "service",
    srcs = ["service.go"],
    importpath = "github.com/google/s2a-go/internal/handshaker/service",
    deps = [
        "@org_golang_google_appengine//:go_default_library",
        "@org_golang_google_appengine//socket:go_default_library",
        "@org_golang_google_grpc//:go_default_library",
        "@org_golang_google_grpc//grpclog:go_default_library",
    ],
)

go_test(
    name = "service_test",
    size = "small",
    srcs = ["service_test.go"],
    embed = [":service"],
    deps = ["@org_golang_google_grpc//:go_default_library"],
)
