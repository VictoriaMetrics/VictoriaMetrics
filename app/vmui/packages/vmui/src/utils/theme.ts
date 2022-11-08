export const getVariableColor = (colorName: string) => {
  return getComputedStyle(document.documentElement).getPropertyValue(`--color-${colorName}`);
};
