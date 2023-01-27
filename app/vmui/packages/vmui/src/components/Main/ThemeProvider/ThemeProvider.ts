import { FC, useEffect } from "preact/compat";
import { getContrastColor } from "../../../utils/color";
import { getCssVariable, setCssVariable } from "../../../utils/theme";
import { AppParams, getAppModeParams } from "../../../utils/app-mode";
import { getFromStorage } from "../../../utils/storage";
import { darkPalette, lightPalette } from "../../../constants/palette";

interface StyleVariablesProps {
  setLoadingTheme: (val: boolean) => void
}

const colorVariables = [
  "primary",
  "secondary",
  "error",
  "warning",
  "info",
  "success",
];

export const ThemeProvider: FC<StyleVariablesProps> = ({ setLoadingTheme }) => {

  const { palette = {} } = getAppModeParams();

  const setScrollbarSize = () => {
    const { innerWidth, innerHeight } = window;
    const { clientWidth, clientHeight } = document.documentElement;
    setCssVariable("scrollbar-width", `${innerWidth - clientWidth}px`);
    setCssVariable("scrollbar-height", `${innerHeight - clientHeight}px`);
  };

  const setContrastText = () => {
    colorVariables.forEach(variable => {
      const color = getCssVariable(`color-${variable}`);
      const text = getContrastColor(color);
      setCssVariable(`${variable}-text`, text);
    });
  };

  const setAppModePalette = () => {
    colorVariables.forEach(variable => {
      const colorFromAppMode = palette[variable as keyof AppParams["palette"]];
      if (colorFromAppMode) setCssVariable(`color-${variable}`, colorFromAppMode);
    });

    setContrastText();
  };

  const setTheme = () => {
    const darkTheme = getFromStorage("DARK_THEME");
    const palette = darkTheme ? darkPalette : lightPalette;
    Object.entries(palette).forEach(([variable, value]) => {
      setCssVariable(variable, value);
    });
    setContrastText();
  };

  useEffect(() => {
    setAppModePalette();
    setScrollbarSize();
    setTheme();
    setLoadingTheme(false);

    window.addEventListener("storage", setTheme);
    return () => {
      window.removeEventListener("storage", setTheme);
    };
  }, []);

  return null;
};

export default ThemeProvider;
