
export const splitByCursor = (
  value: string,
  caret: [number, number]
) => {
  if (caret[0] !== caret[1]) {
    return { beforeCursor: value, afterCursor: "" };
  }

  return {
    beforeCursor: value.substring(0, caret[0]),
    afterCursor: value.substring(caret[1]),
  };
};


export const extractMetric = (expr: string): string => {
  const fnRegex = /\w+\((?<metricName>[^)]+)\)\s+(by|without|on|ignoring)\s*\(\w*/gi;
  const fnMatch = [...expr.matchAll(fnRegex)];

  if (fnMatch[0]?.groups?.metricName) {
    return fnMatch[0].groups.metricName;
  }

  const plainRegex = /^\s*\b(?<metricName>[^{}(),\s]+)(?={|$)/g;
  const match = [...expr.matchAll(plainRegex)];
  return match[0]?.groups?.metricName || "";
};

export const extractCurrentLabel = (expr: string): string => {
  const regexp = /[a-z_:-][\w\-.:/]*\b(?=\s*(=|!=|=~|!~))/g;
  const match = expr.match(regexp);
  return match ? match[match.length - 1] : "";
};


export const extractLabelMatchers = (
  expr: string,
  currentLabel?: string
): string[] => {
  const regexp = /([a-z_:-][\w\-.:/]*)\s*(?:=|!=|=~|!~)\s*"[^"]*"/g;

  const matches = [...expr.matchAll(regexp)];
  // m[1] = label name
  // m[0] = full matcher string

  if (!currentLabel) return matches.map(m => m[0]);

  return matches
    .filter(m => m[1] !== currentLabel)
    .map(m => m[0]);
};
