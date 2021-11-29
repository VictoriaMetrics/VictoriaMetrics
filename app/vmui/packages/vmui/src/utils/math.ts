export const getMaxFromArray = (arr: number[]): number => {
  let len = arr.length;
  let max = -Infinity;
  while (len--) {
    if (arr[len] > max) max = arr[len];
  }
  return max;
};

export const getMinFromArray = (arr: number[]): number => {
  let len = arr.length;
  let min = Infinity;
  while (len--) {
    if (arr[len] < min) min = arr[len];
  }
  return min;
};