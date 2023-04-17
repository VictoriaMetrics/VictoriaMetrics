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
#  Provides tokens for authenticating to S2A.

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

licenses(["notice"])

package(
    default_visibility = ["//:__subpackages__"],
)

go_library(
    name = "tokenmanager",
    srcs = ["tokenmanager.go"],
    importpath = "github.com/google/s2a-go/internal/tokenmanager",
    deps = ["//internal/proto/common_go_proto:common_go_proto"],
)

go_test(
    name = "tokenmanager_test",
    size = "small",
    srcs = ["tokenmanager_test.go"],
    embed = [":tokenmanager"],
    deps = ["//internal/proto/common_go_proto:common_go_proto"],
)
