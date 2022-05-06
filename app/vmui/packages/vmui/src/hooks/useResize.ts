import { useState, useEffect } from "preact/compat";

const useResize = (node: HTMLElement | null): {width: number, height: number} => {
  const [windowSize, setWindowSize] = useState({
    width: 0,
    height: 0,
  });
  useEffect(() => {
    const observer = new ResizeObserver((entries) => {
      const {width, height} = entries[0].contentRect;
      setWindowSize({width, height});
    });
    if (node) observer.observe(node);
    return () => {
      if (node) observer.unobserve(node);
    };
  }, []);
  return windowSize;
};

export default useResize;
