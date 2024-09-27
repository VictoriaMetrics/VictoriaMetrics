export const getStreamPairs = (value: string): string[] => {
  const pairs = /^{.+}$/.test(value) ? value.slice(1, -1).split(",") : [value];
  return pairs.filter(Boolean);
};
