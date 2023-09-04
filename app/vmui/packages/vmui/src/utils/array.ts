export const arrayEquals = (a: (string|number)[], b: (string|number)[]) => {
  return a.length === b.length && a.every((val, index) => val === b[index]);
};

export function groupByMultipleKeys<T>(items: T[], keys: (keyof T)[]): { keys: string[], values: T[] }[] {
  const groups = items.reduce((result, item) => {
    const compositeKey = keys.map(key => `${key}: ${item[key] || "-"}`).join("|");

    (result[compositeKey] = result[compositeKey] || []).push(item);

    return result;
  }, {} as { [key: string]: T[] });

  return Object.entries(groups).map(([keyString, values]) => ({
    keys: keyString.split("|"),
    values
  }));
}

