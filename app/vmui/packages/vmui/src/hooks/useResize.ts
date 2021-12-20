import { useState, useEffect } from "react";

const useResize = (node: HTMLElement | null): {width: number, height: number} => {
  const [windowSize, setWindowSize] = useState({
    width: 0,
    height: 0,
  });
  useEffect(() => {
    if (!node) return;
    const handleResize = () => {
      setWindowSize({
        width: node.offsetWidth,
        height: node.offsetHeight,
      });
    };
    window.addEventListener("resize", handleResize);
    handleResize();
    return () => window.removeEventListener("resize", handleResize);
  }, []);
  return windowSize;
};

export default useResize;