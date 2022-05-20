export const arrayEquals = (a: (string|number)[], b: (string|number)[]) => {
  return a.length === b.length && a.every((val, index) => val === b[index]);
};

