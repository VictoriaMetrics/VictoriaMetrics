import { FC, useEffect } from "preact/compat";
import { getContrastColor } from "../../../utils/color";
import { getCssVariable, setCssVariable } from "../../../utils/theme";
import { AppParams, getAppModeParams } from "../../../utils/app-mode";

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

  const setAppModePalette = () => {
    colorVariables.forEach(variable => {
      const colorFromAppMode = palette[variable as keyof AppParams["palette"]];
      if (colorFromAppMode) setCssVariable(`color-${variable}`, colorFromAppMode);
    });
  };

  const setContrastText = () => {
    colorVariables.forEach(variable => {
      const color = getCssVariable(`color-${variable}`);
      const text = getContrastColor(color);
      setCssVariable(`${variable}-text`, text);
    });
  };

  useEffect(() => {
    setAppModePalette();
    setScrollbarSize();
    setContrastText();
    setLoadingTheme(false);
  }, []);

  return null;
};

export default ThemeProvider;
