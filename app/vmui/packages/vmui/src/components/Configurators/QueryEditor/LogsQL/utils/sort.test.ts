import { hasSortPipe } from "./sort";

describe("hasSortPipe()", () => {
  /** Queries that MUST be recognised as containing a sort/order pipe. */
  const positive: string[] = [
    // ───── basic usage ─────
    "sort by (_time)",
    "| sort by (_time)",
    "|sort(_time) desc",
    "| order by (foo desc)",
    "_time:5m | sort by (_stream, _time)",

    // ───── documented options ─────
    "_time:1h | sort by (request_duration desc) limit 10",
    "_time:1h | sort by (request_duration desc) partition by (host) limit 3",
    "_time:5m | sort by (_time) rank as position",

    // ───── whitespace / tabs ─────
    "|\t  sort\tby (host)",

    // ───── no space after the pipe ─────
    "foo|sort by (_time)",
  ];

  /** Queries that MUST **not** be recognised (false positives). */
  const negative: string[] = [
    "",                                   // empty
    "error | sample 100",                 // no sort
    "|sorted(field)",                     // 'sorted' ≠ 'sort'
    "|sorter(field)",                     // 'sorter' ≠ 'sort'
    "my_sort(field)",                     // function name
    "| sorta by (field)",                 // 'sorta'
    "foo | orderliness by (bar)",         // 'orderliness' ≠ 'order'
  ];

  it.each(positive)("detects pipe in ➜  %s", query => {
    expect(hasSortPipe(query)).toBe(true);
  });

  it.each(negative)("does NOT detect pipe in ➜  %s", query => {
    expect(hasSortPipe(query)).toBe(false);
  });
});
