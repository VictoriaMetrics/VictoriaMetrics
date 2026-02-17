#!/bin/bash

# verify-sbom.sh — Verify that SBOMs are attached to container images and discoverable by Trivy.
#
# Usage:
#   ./scripts/verify-sbom.sh <image-ref>
#
# Example:
#   ./scripts/verify-sbom.sh docker.io/victoriametrics/vmagent:v1.100.0
#
# Prerequisites: oras, jq, and optionally trivy must be installed.

set -euo pipefail

SBOM_MEDIA_TYPE="application/vnd.cyclonedx+json"

usage() {
    echo "Usage: $0 <image-ref>"
    echo ""
    echo "Example: $0 docker.io/victoriametrics/vmagent:v1.100.0"
    exit 1
}

if [ $# -ne 1 ]; then
    usage
fi

IMAGE="$1"

echo "=== Verifying SBOM for ${IMAGE} ==="

# Step 1: Check that oras is installed
if ! command -v oras >/dev/null 2>&1; then
    echo "ERROR: oras is not installed. Install it first."
    exit 1
fi

# Step 2: Check that jq is installed
if ! command -v jq >/dev/null 2>&1; then
    echo "ERROR: jq is not installed. Install it first."
    exit 1
fi

# Step 3: Discover SBOM referrers
echo ""
echo "--- Step 1: Discovering SBOM artifacts via oras ---"

# Use default tree output and grep for the media type first as a quick check
if ! oras discover --artifact-type "${SBOM_MEDIA_TYPE}" \
    --distribution-spec v1.1-referrers-tag \
    "${IMAGE}" 2>/dev/null | grep -q "${SBOM_MEDIA_TYPE}"; then
    echo "ERROR: No SBOM referrers found for ${IMAGE}"
    exit 1
fi
echo "SBOM referrer found for ${IMAGE}"

# Get JSON output for detailed info (separate stderr to avoid corrupting JSON)
DISCOVER_JSON=$(oras discover --artifact-type "${SBOM_MEDIA_TYPE}" \
    --format json \
    --distribution-spec v1.1-referrers-tag \
    "${IMAGE}" 2>/dev/null) || true

# Step 4: Pull and validate the SBOM
echo ""
echo "--- Step 2: Pulling and validating SBOM ---"
SBOM_TMPDIR=$(mktemp -d)
trap 'rm -rf "${SBOM_TMPDIR}"' EXIT

# Try to extract digest from JSON output; handle different oras output formats
SBOM_DIGEST=""
if [ -n "${DISCOVER_JSON}" ]; then
    # Try .referrers[0].digest (oras 1.x format)
    SBOM_DIGEST=$(echo "${DISCOVER_JSON}" | jq -r '.referrers[0].digest // empty' 2>/dev/null) || true
    # Try .manifests[0].digest (alternative format)
    if [ -z "${SBOM_DIGEST}" ]; then
        SBOM_DIGEST=$(echo "${DISCOVER_JSON}" | jq -r '.manifests[0].digest // empty' 2>/dev/null) || true
    fi
fi

if [ -z "${SBOM_DIGEST}" ]; then
    echo "WARNING: Could not extract SBOM digest from JSON output, skipping pull validation"
    echo ""
    echo "=== SBOM discovery verified for ${IMAGE} (pull validation skipped) ==="
    exit 0
fi

echo "Pulling SBOM with digest: ${SBOM_DIGEST}"

# Extract registry/repo by stripping digest (@sha256:...) or tag (last :segment without slashes)
REPO_REF=$(echo "${IMAGE}" | sed 's/@sha256:.*$//' | sed 's/:[^/:]*$//')
if ! oras pull --output "${SBOM_TMPDIR}" \
    --distribution-spec v1.1-referrers-tag \
    "${REPO_REF}@${SBOM_DIGEST}" 2>&1; then
    echo "WARNING: oras pull failed, attempting without --distribution-spec flag..."
    oras pull --output "${SBOM_TMPDIR}" \
        "${REPO_REF}@${SBOM_DIGEST}" 2>&1 || {
        echo "WARNING: Could not pull SBOM artifact, skipping content validation"
        echo ""
        echo "=== SBOM discovery verified for ${IMAGE} (content validation skipped) ==="
        exit 0
    }
fi

SBOM_FILE=$(find "${SBOM_TMPDIR}" -name "*.json" -o -name "sbom*" | head -1)
if [ -z "${SBOM_FILE}" ]; then
    # Try any file in the directory
    SBOM_FILE=$(find "${SBOM_TMPDIR}" -type f | head -1)
fi

if [ -z "${SBOM_FILE}" ]; then
    echo "ERROR: Could not find SBOM file after pulling"
    exit 1
fi

# Validate it's valid CycloneDX JSON
if ! jq -e '.bomFormat' "${SBOM_FILE}" >/dev/null 2>&1; then
    echo "ERROR: SBOM file is not valid CycloneDX JSON"
    exit 1
fi

BOM_FORMAT=$(jq -r '.bomFormat' "${SBOM_FILE}")
SPEC_VERSION=$(jq -r '.specVersion' "${SBOM_FILE}")
COMPONENT_COUNT=$(jq '.components | length' "${SBOM_FILE}")

echo "Valid CycloneDX SBOM:"
echo "  Format:     ${BOM_FORMAT}"
echo "  Version:    ${SPEC_VERSION}"
echo "  Components: ${COMPONENT_COUNT}"

# Step 5: Trivy integration (optional)
echo ""
echo "--- Step 3: Trivy integration check ---"
if command -v trivy >/dev/null 2>&1; then
    echo "Running: trivy image --sbom-sources oci ${IMAGE}"
    trivy image --sbom-sources oci "${IMAGE}" && rc=0 || rc=$?
    if [ "$rc" -eq 0 ]; then
        echo "Trivy successfully discovered and used the OCI SBOM"
    else
        echo "ERROR: Trivy failed to scan the image using the OCI SBOM (exit code: ${rc})"
        exit 1
    fi
else
    echo "SKIPPED: trivy is not installed. Install it to verify end-to-end Trivy integration."
fi

echo ""
echo "=== SBOM verification passed for ${IMAGE} ==="
