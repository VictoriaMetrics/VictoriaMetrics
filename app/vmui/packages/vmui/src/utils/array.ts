export const arrayEquals = (a: (string | number)[], b: (string | number)[]) => {
  return a.length === b.length && a.every((val, index) => val === b[index]);
};

export const isDecreasing = (arr: number[]): boolean => {
  if (arr.length < 2) return false;

  return arr.every((v, i) => i === 0 || v < arr[i - 1]);
};
