import { useEffect, RefObject } from "react";

type Event = MouseEvent | TouchEvent;

const useClickOutside = <T extends HTMLElement = HTMLElement>(
  ref: RefObject<T>,
  handler: (event: Event) => void,
  preventRef?: RefObject<T>
) => {
  useEffect(() => {
    const el = ref?.current;

    const listener = (event: Event) => {
      const target = event.target as HTMLElement;
      const isPreventRef = preventRef?.current && preventRef.current.contains(target);
      if (!el || el.contains((event?.target as Node) || null) || isPreventRef) {
        return;
      }

      handler(event); // Call the handler only if the click is outside of the element passed.
    };

    document.addEventListener("mousedown", listener);
    document.addEventListener("touchstart", listener);

    const removeListeners = () => {
      document.removeEventListener("mousedown", listener);
      document.removeEventListener("touchstart", listener);
    };

    if (!el) removeListeners();
    return removeListeners;
  }, [ref, handler]); // Reload only if ref or handler changes
};

export default useClickOutside;
