import { useState, useEffect } from "preact/compat";
import { StateUpdater } from "preact/hooks";
import { useSearchParams } from "react-router-dom";

const useStateSearchParams = <T>(defaultState: T, key: string): [T, StateUpdater<T>] => {
  const [searchParams] = useSearchParams();
  const currentValue = searchParams.get(key) ? searchParams.get(key) as unknown as T : defaultState;
  const [state, setState] = useState<T>(currentValue);

  useEffect(() => {
    if ((currentValue as unknown as T) !== state) {
      setState(currentValue as unknown as T);
    }
  }, [currentValue]);

  return [state, setState];
};

export default useStateSearchParams;
