import { RefObject } from "react";
import useEventListener from "./useEventListener";
import { useCallback } from "preact/compat";

type Event = MouseEvent | TouchEvent;

const useClickOutside = <T extends HTMLElement = HTMLElement>(
  ref: RefObject<T>,
  handler: (event: Event) => void,
  preventRef?: RefObject<T> | null
) => {
  const listener = useCallback((event: Event) => {
    const el = ref?.current;
    const target = event.target as HTMLElement;
    const isPreventRef = preventRef?.current && preventRef.current.contains(target);
    if (!el || el.contains((event?.target as Node) || null) || isPreventRef) {
      return;
    }

    handler(event); // Call the handler only if the click is outside of the element passed.
  }, [ref, handler]);

  useEventListener("mousedown", listener);
  useEventListener("touchstart", listener);
};

export default useClickOutside;
