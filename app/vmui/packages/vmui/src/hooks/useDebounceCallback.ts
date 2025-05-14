import { useMemo, useRef } from "react";
import debounce from "lodash.debounce";
import { useUnmount } from "./useUnmount";

type DebounceOptions = {
  leading?: boolean;
  trailing?: boolean;
  maxWait?: number;
};

type ControlFunctions = {
  cancel: () => void;
  flush: () => void;
};

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export type DebouncedState<T extends (...args: any) => any> = ((
  ...args: Parameters<T>
) => void) &
  ControlFunctions;

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export function useDebounceCallback<T extends (...args: any) => any>(
  func: T,
  delay = 500,
  options?: DebounceOptions
): DebouncedState<T> {
  const funcRef = useRef(func);
  funcRef.current = func;

  const debounced = useMemo(() => {
    const debouncedFunc = debounce(
      (...args: Parameters<T>) => funcRef.current(...args),
      delay,
      options
    );

    const wrapped: DebouncedState<T> = (...args: Parameters<T>) => {
      debouncedFunc(...args);
    };

    wrapped.cancel = debouncedFunc.cancel;
    wrapped.flush = debouncedFunc.flush;

    return wrapped;
  }, [delay, options]);

  useUnmount(() => {
    debounced.cancel();
  });

  return debounced;
}
