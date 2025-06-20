import { useState, useEffect } from "react";
import useEventListener from "./useEventListener";

interface WindowSize {
    width: number
    height: number
}

const useWindowSize = (): WindowSize => {
  const [windowSize, setWindowSize] = useState<WindowSize>({
    width: 0,
    height: 0,
  });

  const handleSize = () => {
    setWindowSize({
      width: window.innerWidth,
      height: window.innerHeight,
    });
  };

  useEventListener("resize", handleSize);

  // Set size at the first client-side load
  useEffect(handleSize, []);

  return windowSize;
};

export default useWindowSize;
