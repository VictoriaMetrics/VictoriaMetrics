export const getCssVariable = (variable: string) => {
  return getComputedStyle(document.documentElement).getPropertyValue(`--${variable}`);
};

export const setCssVariable = (variable: string, value: string) => {
  document.documentElement.style.setProperty(`--${variable}`, value);
};
