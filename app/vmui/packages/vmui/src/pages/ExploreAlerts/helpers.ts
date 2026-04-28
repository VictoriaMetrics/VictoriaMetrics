export const getChanges = (title: string, prevValues: string[]): string[] => {
  if (title === "All") return [];

  const newValues = new Set<string>(prevValues);
  if (newValues.has(title)) {
    newValues.delete(title);
  } else {
    newValues.add(title);
  }

  return Array.from(newValues);
};
