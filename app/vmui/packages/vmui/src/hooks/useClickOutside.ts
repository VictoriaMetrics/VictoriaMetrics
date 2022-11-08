import { useEffect, RefObject } from "react";

type Event = MouseEvent | TouchEvent;

const useClickOutside = <T extends HTMLElement = HTMLElement>(
  ref: RefObject<T>,
  handler: (event: Event) => void,
  preventRef?: RefObject<T>
) => {
  useEffect(() => {
    const listener = (event: Event) => {
      const el = ref?.current;
      const isPreventRef = preventRef?.current && preventRef.current.contains(event.target as Node);
      if (!el || el.contains((event?.target as Node) || null) || isPreventRef) {
        return;
      }

      handler(event); // Call the handler only if the click is outside of the element passed.
    };

    document.addEventListener("mousedown", listener);
    document.addEventListener("touchstart", listener);

    return () => {
      document.removeEventListener("mousedown", listener);
      document.removeEventListener("touchstart", listener);
    };
  }, [ref, handler]); // Reload only if ref or handler changes
};

export default useClickOutside;
