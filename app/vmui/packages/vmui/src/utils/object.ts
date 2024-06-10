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
