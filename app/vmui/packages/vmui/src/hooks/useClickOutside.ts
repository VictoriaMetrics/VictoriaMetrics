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
      const target = event.target as HTMLElement;

      let checkParents = target.parentNode;
      const nodes = [];
      while(checkParents?.parentNode) {
        nodes.unshift(checkParents.parentNode as HTMLElement);
        checkParents = checkParents.parentNode;
      }
      const hasPopper = nodes.map(node => node.classList ? node.classList.contains("vm-popper") : false).some(el => el);
      const isPreventRef = preventRef?.current && preventRef.current.contains(target);
      if (!el || el.contains((event?.target as Node) || null) || isPreventRef || hasPopper) {
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
