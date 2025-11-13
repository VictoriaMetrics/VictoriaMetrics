// In-place quickselect: reorders the array so arr[k] is the k-th smallest (avg O(n));
// uses a Floyd–Rivest speedup.

export default function quickselect<T>(
  arr: T[],
  k: number,
  left: number = 0,
  right: number = arr.length - 1,
  compare: (a: T, b: T) => number = defaultCompare as (a: T, b: T) => number
): void {
  while (right > left) {
    if (right - left > 600) {
      const n = right - left + 1;
      const m = k - left + 1;
      const z = Math.log(n);
      const s = 0.5 * Math.exp((2 * z) / 3);
      const sd = 0.5 * Math.sqrt((z * s * (n - s)) / n) * (m - n / 2 < 0 ? -1 : 1);
      const newLeft = Math.max(left, Math.floor(k - (m * s) / n + sd));
      const newRight = Math.min(right, Math.floor(k + ((n - m) * s) / n + sd));
      quickselect(arr, k, newLeft, newRight, compare);
    }

    const t = arr[k];
    let i = left;
    let j: number = right;

    swap(arr, left, k);
    if (compare(arr[right], t) > 0) swap(arr, left, right);

    while (i < j) {
      swap(arr, i, j);
      i++;
      j--;
      while (compare(arr[i], t) < 0) i++;
      while (compare(arr[j], t) > 0) j--;
    }

    if (compare(arr[left], t) === 0) {
      swap(arr, left, j);
    } else {
      j++;
      swap(arr, j, right);
    }

    if (j <= k) left = j + 1;
    if (k <= j) right = j - 1;
  }
}

function swap<T>(arr: T[], i: number, j: number): void {
  const tmp = arr[i];
  arr[i] = arr[j];
  arr[j] = tmp;
}

function defaultCompare(a: number, b: number): number {
  return a < b ? -1 : a > b ? 1 : 0;
}
