type Entry<T> = {
  [K in keyof T]: [K, T[K]]
}[keyof T]

export function filterObject<T extends object>(
  obj: T,
  fn: (entry: Entry<T>, i: number, arr: Entry<T>[]) => boolean
) {
  return Object.fromEntries(
    (Object.entries(obj) as Entry<T>[]).filter(fn)
  ) as Partial<T>;
}

export function compactObject<T extends object>(obj: T) {
  return filterObject(obj, (entry) => !!entry[1] || typeof entry[1] === "number");
}

export function isEmptyObject(obj: object) {
  return Object.keys(obj).length === 0;
}


export const isObject = (value: unknown): value is Record<string, unknown> => {
  return value !== null && typeof value === "object" && !Array.isArray(value);
};

export const getValueByPath = <T extends object, P extends string>(
  obj: T,
  path: P
) => {
  return path.split(".").reduce((o: T | unknown, key) => {
    if(isObject(o)) return o[key];
    if(Array.isArray(o)) {
      const index = parseInt(key, 10);
      return o[index] !== undefined ? o[index] : undefined;
    }
    return undefined;
  }, obj);
};
