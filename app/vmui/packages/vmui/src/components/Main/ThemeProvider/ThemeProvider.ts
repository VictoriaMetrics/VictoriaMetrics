import { FC, useEffect, useState } from "preact/compat";
import { getContrastColor } from "../../../utils/color";
import { getCssVariable, isSystemDark, setCssVariable } from "../../../utils/theme";
import { AppParams, getAppModeParams } from "../../../utils/app-mode";
import { getFromStorage } from "../../../utils/storage";
import { darkPalette, lightPalette } from "../../../constants/palette";
import { Theme } from "../../../types";
import { useAppDispatch, useAppState } from "../../../state/common/StateContext";
import useSystemTheme from "../../../hooks/useSystemTheme";

interface ThemeProviderProps {
  onLoaded: (val: boolean) => void
}

const colorVariables = [
  "primary",
  "secondary",
  "error",
  "warning",
  "info",
  "success",
];

export const ThemeProvider: FC<ThemeProviderProps> = ({ onLoaded }) => {

  const { palette: paletteAppMode = {} } = getAppModeParams();
  const { theme } = useAppState();
  const isDarkTheme = useSystemTheme();
  const dispatch = useAppDispatch();

  const [palette, setPalette] = useState({
    [Theme.dark]: darkPalette,
    [Theme.light]: lightPalette,
    [Theme.system]: isSystemDark() ? darkPalette : lightPalette
  });

  const setScrollbarSize = () => {
    const { innerWidth, innerHeight } = window;
    const { clientWidth, clientHeight } = document.documentElement;
    setCssVariable("scrollbar-width", `${innerWidth - clientWidth}px`);
    setCssVariable("scrollbar-height", `${innerHeight - clientHeight}px`);
  };

  const setContrastText = () => {
    colorVariables.forEach((variable, i) => {
      const color = getCssVariable(`color-${variable}`);
      const text = getContrastColor(color);
      setCssVariable(`${variable}-text`, text);

      if (i === colorVariables.length - 1) {
        dispatch({ type: "SET_DARK_THEME" });
        onLoaded(true);
      }
    });
  };

  const setAppModePalette = () => {
    colorVariables.forEach(variable => {
      const colorFromAppMode = paletteAppMode[variable as keyof AppParams["palette"]];
      if (colorFromAppMode) setCssVariable(`color-${variable}`, colorFromAppMode);
    });

    setContrastText();
  };

  const setTheme = () => {
    const theme = (getFromStorage("THEME") || Theme.system) as Theme;
    const result = palette[theme];
    Object.entries(result).forEach(([variable, value]) => {
      setCssVariable(variable, value);
    });
    setContrastText();
  };

  const updatePalette = () => {
    const newSystemPalette = isSystemDark() ? darkPalette : lightPalette;
    if (palette[Theme.system] === newSystemPalette) {
      setTheme();
      return;
    }
    setPalette(prev => ({
      ...prev,
      [Theme.system]: newSystemPalette
    }));
  };

  useEffect(() => {
    setAppModePalette();
    setScrollbarSize();
    setTheme();
  }, [palette]);

  useEffect(updatePalette, [theme, isDarkTheme]);

  return null;
};

export default ThemeProvider;
