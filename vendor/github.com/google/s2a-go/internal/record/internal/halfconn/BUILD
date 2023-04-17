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
#  The half connection manages the TLS connection in the inbound or outbound direction.

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

licenses(["notice"])

package(
    default_visibility = ["//internal:__subpackages__"],
)

go_library(
    name = "halfconn",
    srcs = [
        "ciphersuite.go",
        "counter.go",
        "expander.go",
        "halfconn.go",
    ],
    importpath = "github.com/google/s2a-go/internal/record/internal/halfconn",
    deps = [
        "//internal/proto/common_go_proto:common_go_proto",
        "//internal/record/internal/aeadcrypter",
        "@org_golang_x_crypto//cryptobyte",
        "@org_golang_x_crypto//hkdf",
    ],
)

go_test(
    name = "halfconn_test",
    size = "small",
    srcs = [
        "ciphersuite_test.go",
        "counter_test.go",
        "expander_test.go",
        "halfconn_test.go",
    ],
    embed = [":halfconn"],
    deps = [
        "//internal/proto/common_go_proto:common_go_proto",
        "//internal/record/internal/aeadcrypter",
        "//internal/record/internal/aeadcrypter/testutil",
    ],
)
