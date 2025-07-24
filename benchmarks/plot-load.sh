#!/bin/bash

# Check the number of arguments
if [ "$#" -lt 1 ] || [ "$#" -gt 2 ]; then
  echo "Usage: $(basename $0) file1.csv [file2.csv]"
  exit 1
fi

file1="$1"
file2="$2"

# Check if the files exist
if [ ! -f "$file1" ]; then
  echo "File not found: $file1"
  exit 1
fi

if [ -n "$file2" ] && [ ! -f "$file2" ]; then
  echo "File not found: $file2"
  exit 1
fi

# Temporary file for plot data
plotdata="/tmp/tsbs_plot.dat"

if [ -z "$file2" ]; then
  # === One file ===
  awk -F, '/^Summary:/ {exit} NR > 1 {print $1, $2}' "$file1" > "$plotdata"
else
  # === Two files ===

  # Get time and per.metric/s from file1
  awk -F, '/^Summary:/ {exit} NR > 1 {print $1, $2}' "$file1" > /tmp/file1.dat
  # Get per.metric/s from file2
  awk -F, '/^Summary:/ {exit} NR > 1 {print $2}' "$file2" > /tmp/file2.dat
  # Merge by rows: time, val1, val2
  paste /tmp/file1.dat /tmp/file2.dat | awk '{print $1, $2, $3}' > "$plotdata"
fi

# === Build plot dynamically ===
gnuplot -persist <<-EOF
  set datafile separator " "
  set title "per.metric/s"
  set xlabel "Timestamp"
  set xdata time
  set timefmt "%s"
  set format x "%H:%M:%S"
  set ylabel "per. metric/s"
  set format y "%'.0f"
  set grid

  plot "$plotdata" using 1:2 with lines title "$(basename "$file1")" \
  $( [[ -n "$file2" ]] && echo ", \\
        \"$plotdata\" using 1:3 with lines title \"$(basename "$file2")\"" )
EOF
