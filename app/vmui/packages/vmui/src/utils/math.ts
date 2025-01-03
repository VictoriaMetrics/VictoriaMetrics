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

export const getAvgFromArray = (a: number[]) => {
  let mean = a[0];
  let n = 1;
  for (let i = 1; i < a.length; i++) {
    const v = a[i];
    if (Number.isFinite(v)) {
      mean = mean * (n-1)/n + v / n;
      n++;
    }
  }
  return mean;
};

export const getMedianFromArray = (a: number[]) => {
  let len = a.length;
  const aCopy = [];
  while (len--) {
    const v = a[len];
    if (Number.isFinite(v)) {
      aCopy.push(v);
    }
  }
  aCopy.sort();
  return aCopy[aCopy.length>>1];
};

export const getLastFromArray = (a: number[]) => {
  let len = a.length;
  while (len--) {
    const v = a[len];
    if (Number.isFinite(v)) {
      return v;
    }
  }
};

export const formatNumberShort = (value: number) => {
  if (value >= 1_000_000_000) {
    return (value / 1_000_000_000).toFixed(1).replace(/\.0$/, "") + "B"; // Миллиарды
  } else if (value >= 1_000_000) {
    return (value / 1_000_000).toFixed(1).replace(/\.0$/, "") + "M"; // Миллионы
  } else if (value >= 1_000) {
    return (value / 1_000).toFixed(1).replace(/\.0$/, "") + "K"; // Тысячи
  } else {
    return value.toString(); // Для чисел меньше 1000
  }
};
