import { useEffect, useRef } from "react";

function usePrevious<T>(value: T): T | null {
  const ref = useRef<T>(null);
  useEffect(() => {
    ref.current = value;
  }, [value]);

  return ref.current;
}

export default usePrevious;
