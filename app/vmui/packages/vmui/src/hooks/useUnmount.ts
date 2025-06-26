import { useEffect, useRef } from "react";

export function useUnmount(fn: () => void) {
  const fnRef = useRef(fn);

  useEffect(() => {
    fnRef.current = fn;
  }, [fn]);

  useEffect(() => {
    return () => {
      fnRef.current();
    };
  }, []);
}
