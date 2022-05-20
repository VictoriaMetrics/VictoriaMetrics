import { useRef, useEffect } from "react";

export default (value: any) => {
  const ref = useRef();
  useEffect(() => {
    ref.current = value;
  }, [value]);

  return ref.current;
};
