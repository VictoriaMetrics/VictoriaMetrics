import { Theme } from "../types";

export const getCssVariable = (variable: string) => {
  return getComputedStyle(document.documentElement).getPropertyValue(`--${variable}`);
};

export const setCssVariable = (variable: string, value: string) => {
  document.documentElement.style.setProperty(`--${variable}`, value);
};

export const isSystemDark = () => window.matchMedia("(prefers-color-scheme: dark)").matches;

export const isDarkTheme = (theme: Theme) => (theme === Theme.system && isSystemDark()) || theme === Theme.dark;

