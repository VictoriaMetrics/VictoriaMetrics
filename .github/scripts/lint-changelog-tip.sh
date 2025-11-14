#!/usr/bin/env sh

set -e

CHANGELOG_FILE="docs/victoriametrics/changelog/CHANGELOG.md"

GITHUB_BASE_REF=${GITHUB_BASE_REF:-"master"}
GIT_REMOTE=${GIT_REMOTE:-"origin"}

git diff "${GIT_REMOTE}/${GITHUB_BASE_REF}"...HEAD -- $CHANGELOG_FILE > diff.txt
if ! grep -q "^+" diff.txt; then
  echo "No additions in CHANGELOG.md"
  exit 0
fi

ADDED_LINES=$(grep "^+\S" diff.txt | sed 's/^+//')

START_TIP=$(grep -n "^## tip" "$CHANGELOG_FILE" | head -1 | cut -d: -f1)
if [ -z "$START_TIP" ]; then
  echo "ERROR: ${CHANGELOG_FILE} does not contain a ## tip section"
  exit 1
fi

END_TIP=$(awk "NR>$START_TIP && /^## / {print NR; exit}" "${CHANGELOG_FILE}")
if [ -z "$END_TIP" ]; then
  END_TIP=$(wc -l < "$CHANGELOG_FILE")
fi

BAD=0
while IFS= read -r line; do
  # Grep exact line inside the file and get line numbers
  MATCHES=$(grep -n -F "$line" "$CHANGELOG_FILE" | cut -d: -f1)
  for m in $MATCHES; do
    if [ "$m" -lt "$START_TIP" ] || [ "$m" -gt "$END_TIP" ]; then
      echo "'$line' on line ${m} is outside ## tip section (lines ${START_TIP}-${END_TIP})"
      BAD=1
    fi
  done
done << EOF
$ADDED_LINES
EOF

if [ "$BAD" -ne 0 ]; then
  echo "CHANGELOG modifications must be placed inside the ## tip section."
  exit 1
fi

echo "CHANGELOG modifications are valid."