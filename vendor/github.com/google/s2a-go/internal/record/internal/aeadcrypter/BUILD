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
#  The AEAD crypter interface and its AES-GCM and CHACHA-POLY implementations.

load("@io_bazel_rules_go//go:def.bzl", "go_library", "go_test")

licenses(["notice"])

package(
    default_visibility = ["//internal:__subpackages__"],
)

go_library(
    name = "aeadcrypter",
    srcs = [
        "aeadcrypter.go",
        "aesgcm.go",
        "chachapoly.go",
        "common.go",
    ],
    importpath = "github.com/google/s2a-go/internal/record/internal/aeadcrypter",
    deps = ["@org_golang_x_crypto//chacha20poly1305"],
)

go_test(
    name = "aeadcrypter_test",
    size = "small",
    srcs = [
        "aesgcm_test.go",
        "chachapoly_test.go",
        "common_test.go",
    ],
    data = ["//internal/record/internal/aeadcrypter/testdata:wycheproof"],
    embed = [":aeadcrypter"],
    deps = ["//internal/record/internal/aeadcrypter/testutil"],
)
