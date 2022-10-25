export const getMaxFromArray = (a: number[]) => {
  let len = a.length;
  let max = -Infinity;
  while (len--) {
    const v = a[len];
    if (Number.isFinite(v) && v > max) {
      max = v;
    }
  }
  return Number.isFinite(max) ? max : null;
};

export const getMinFromArray = (a: number[]) => {
  let len = a.length;
  let min = Infinity;
  while (len--) {
    const v = a[len];
    if (Number.isFinite(v) && v < min) {
      min = v;
    }
  }
  return Number.isFinite(min) ? min : null;
};

export const getAvgFromArray = (a: number[]) => a.reduce((a,b) => a+b) / a.length;
