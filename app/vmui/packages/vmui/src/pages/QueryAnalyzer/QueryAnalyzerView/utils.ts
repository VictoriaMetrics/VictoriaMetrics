export const findMostCommonStep = (numbers: number[]) => {
  const differences: number[] = numbers.slice(1).map((num, i) => num - numbers[i]);

  const counts: { [key: string]: number } = {};
  differences.forEach(diff => {
    const key = diff.toString();
    counts[key] = (counts[key] || 0) + 1;
  });

  let mostCommonStep = 0;
  let maxCount = 0;
  for (const diff in counts) {
    if (counts[diff] > maxCount) {
      maxCount = counts[diff];
      mostCommonStep = Number(diff);
    }
  }

  return mostCommonStep;
};
