#!/usr/bin/env bash
set -x -e -u

git fetch origin "$GITHUB_BASE_REF"

BASE_ID=$(git rev-parse --verify "origin/$GITHUB_BASE_REF")
CURRENT_ID=$(git rev-parse --verify "HEAD")

# Oldest commit that's not in the base branch
FIRST_BRANCH_ID=$(git rev-list "origin/$GITHUB_BASE_REF..HEAD" | tail -n 1)

# Common ancestor (usually just parent of $FIRST_BRANCH_ID)
# Command will return error 1 if not found anything. So we || true to proceed.
FORK_POINT_ID=$(git merge-base "$FIRST_BRANCH_ID" "origin/$GITHUB_BASE_REF") || true

if [[ -z "$FORK_POINT_ID" ]]
then
  echo "Current branch is not forked from its base branch origin/$GITHUB_BASE_REF"
  exit 1
fi

if [[ "$BASE_ID" != "$FORK_POINT_ID" ]]
then
  echo "Current branch (at $CURRENT_ID) forked at $FORK_POINT_ID is not $BASE_ID (which is the latest \"$GITHUB_BASE_REF\")"
  exit 1
fi
