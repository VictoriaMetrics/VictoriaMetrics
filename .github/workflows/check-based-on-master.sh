#!/usr/bin/env bash

set -x -e
MASTER_ID=$(git rev-parse "origin/master")
CURRENT_ID=$(git rev-parse "HEAD")
BASED_ON_ID=$(git merge-base "$CURRENT_ID" "$MASTER_ID")

if [[ "$MASTER_ID" != "$BASED_ON_ID" ]]
then
  echo "Current branch is based on $BASED_ON_ID, which is not the latest \`master\`: $MASTER_ID. Consider \`git rebase origin/master\`."
  exit 1
fi
