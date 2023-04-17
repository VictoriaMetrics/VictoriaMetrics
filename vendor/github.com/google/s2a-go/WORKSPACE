workspace(name = "s2a_go")

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive")

# Bazel's proto rules.
#
# Last updated: April 8, 2022.
http_archive(
    name = "rules_proto",
    sha256 = "e017528fd1c91c5a33f15493e3a398181a9e821a804eb7ff5acdd1d2d6c2b18d",
    strip_prefix = "rules_proto-4.0.0-3.20.0",
    urls = [
        "https://github.com/bazelbuild/rules_proto/archive/refs/tags/4.0.0-3.20.0.tar.gz",
    ],
)
load("@rules_proto//proto:repositories.bzl", "rules_proto_dependencies", "rules_proto_toolchains")
rules_proto_dependencies()
rules_proto_toolchains()

# Bazel's Go rules.
#
# Last updated: April 8, 2022.
http_archive(
    name = "io_bazel_rules_go",
    sha256 = "f2dcd210c7095febe54b804bb1cd3a58fe8435a909db2ec04e31542631cf715c",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/rules_go/releases/download/v0.31.0/rules_go-v0.31.0.zip",
        "https://github.com/bazelbuild/rules_go/releases/download/v0.31.0/rules_go-v0.31.0.zip",
    ],
)

load("@io_bazel_rules_go//go:deps.bzl", "go_register_toolchains", "go_rules_dependencies")
go_rules_dependencies()
go_register_toolchains(version = "1.16")

# Bazel Gazelle.
#
# Last updated: April 8, 2022.
http_archive(
    name = "bazel_gazelle",
    sha256 = "5982e5463f171da99e3bdaeff8c0f48283a7a5f396ec5282910b9e8a49c0dd7e",
    urls = [
        "https://mirror.bazel.build/github.com/bazelbuild/bazel-gazelle/releases/download/v0.25.0/bazel-gazelle-v0.25.0.tar.gz",
        "https://github.com/bazelbuild/bazel-gazelle/releases/download/v0.25.0/bazel-gazelle-v0.25.0.tar.gz",
    ],
)
load("@bazel_gazelle//:deps.bzl", "gazelle_dependencies", "go_repository")
gazelle_dependencies()

# gRPC-Go.
go_repository(
  name = "org_golang_google_grpc",
  importpath = "google.golang.org/grpc",
  sum = "h1:NEpgUqV3Z+ZjkqMsxMg11IaDrXY4RY6CQukSGK0uI1M=",
  version = "v1.45.0",
)

# Google AppEngine.
#
# Last Updated: January 17, 2023.
go_repository(
  name = "org_golang_google_appengine",
  importpath = "google.golang.org/appengine",
  sum = "h1:/wp5JvzpHIxhs/dumFmF7BXTf3Z+dd4uXta4kVyO508=",
  version = "v1.4.0",
)

# Go Protobuf.
#
#
go_repository(
  name = "org_golang_protobuf",
  importpath = "github.com/golang/protobuf",
  version = "v1.5.2.",
)

# Google Protobuf.
#
# Last updated: April 8, 2022.
go_repository(
  name = "org_golang_google_protobuf",
  importpath = "google.golang.org/protobuf",
  version = "v1.28.0",
)

# Go Cryptography.
#
# Last updated: April 8, 2022.
go_repository(
  name = "org_golang_x_crypto",
  importpath = "golang.org/x/crypto",
  sum = "h1:iU7T1X1J6yxDr0rda54sWGkHgOp5XJrqm79gcNlC2VM=",
  version = "v0.0.0-20220408190544-5352b0902921",
)

# Go Sync.
#
# Last updated: April 8, 2022.
go_repository(
  name = "org_golang_x_sync",
  importpath = "golang.org/x/sync",
  sum = "h1:5KslGYwFpkhGh+Q16bwMP3cOontH8FOep7tGV86Y7SQ=",
  version = "v0.0.0-20210220032951-036812b2e83c",
)

# Go Cmp. No stable versions available.
#
# Last updated: June 4,2021.
go_repository(
  name = "com_github_google_go_cmp",
  importpath = "github.com/google/go-cmp",
  commit = "290a6a23966f9edffe2a0a4a1d8dd065cc0753fd"
)

# Go Sys.
#
# Last updated: April 8, 2022.
go_repository(
  name = "org_golang_x_sys",
  importpath = "golang.org/x/sys",
  sum = "h1:QyVthZKMsyaQwBTJE04jdNN0Pp5Fn9Qga0mrgxyERQM=",
  version = "v0.0.0-20220406163625-3f8b81556e12",
)

# Go Net.
#
# Last updated: April 8, 2022.
go_repository(
  name = "org_golang_x_net",
  importpath = "golang.org/x/net",
  sum = "h1:EN5+DfgmRMvRUrMGERW2gQl3Vc+Z7ZMnI/xdEpPSf0c=",
  version = "v0.0.0-20220407224826-aac1ed45d8e3",
)

# Go Text.
#
# Last updated: April 8, 2022.
go_repository(
  name = "org_golang_x_text",
  importpath = "golang.org/x/text",
  sum = "h1:olpwvP2KacW1ZWvsR7uQhoyTYvKAupfQrRGBFM352Gk=",
  version = "v0.3.7",
)

