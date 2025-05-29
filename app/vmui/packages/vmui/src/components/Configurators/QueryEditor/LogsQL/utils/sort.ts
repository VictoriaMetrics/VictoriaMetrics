const hasSortPipeRe = /(?:^|\|)\s*(?:sort|order)\b/i;

export function hasSortPipe(query: string): boolean {
  return hasSortPipeRe.test(query);
}
