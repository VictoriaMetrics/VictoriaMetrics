import { Logs } from "../../../../api/types";


export const parseStreamToObject = (stream: string) =>  {
  const trimmedStr = stream.slice(1, -1);
  const pairs = trimmedStr.split(",");
  const obj: Record<string, string> = {};
  pairs.forEach(pair => {
    const [key = "", value = ""] = pair.split("=");
    const trimmedKey = key.trim();
    obj[trimmedKey] = value.trim().replace(/"/g, "");
  });

  return obj;
};

export const convertSetToArray = (obj: Record<string, Set<string>>) => {
  return Object.keys(obj).reduce((acc, key) => {
    acc[key] = [...obj[key]];
    return acc;
  }, {} as Record<string, string[]>);
};

export const extractUniqueValues = (
  logs: Logs[],
  getKeyValues: (log: Logs) => Record<string, string>,
  excludeKeys: string[] = []
): Record<string, string[]> => {
  const result = logs.reduce((acc, log) => {
    const keyValues = getKeyValues(log);
    Object.entries(keyValues)
      .filter(([key]) => !excludeKeys.includes(key))
      .forEach(([key, value]) => {
        if (!acc[key] && key) {
          acc[key] = new Set();
        }
        if (value) {
          acc[key].add(value);
        }
      });
    return acc;
  }, {} as Record<string, Set<string>>);

  return convertSetToArray(result);
};
