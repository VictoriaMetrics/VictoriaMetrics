import { useEffect, useState } from "preact/compat";
import { isSystemDark } from "../utils/theme";

const useThemeDetector = () => {
  const [isDarkTheme, setIsDarkTheme] = useState(isSystemDark());

  const mqListener = ((e: MediaQueryListEvent) => {
    setIsDarkTheme(e.matches);
  });

  useEffect(() => {
    const darkThemeMq = window.matchMedia("(prefers-color-scheme: dark)");
    darkThemeMq.addEventListener("change", mqListener);
    return () => darkThemeMq.removeEventListener("change", mqListener);
  }, []);

  return isDarkTheme;
};

export default useThemeDetector;
