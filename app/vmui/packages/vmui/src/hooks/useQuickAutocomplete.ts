import useEventListener from "./useEventListener";
import { useQueryDispatch } from "../state/query/QueryStateContext";
import { useCallback } from "react";

export const useQuickAutocomplete = () => {
  const queryDispatch = useQueryDispatch();

  const setQuickAutocomplete = useCallback((value: boolean) => {
    queryDispatch({ type: "SET_AUTOCOMPLETE_QUICK", payload: value });
  }, [queryDispatch]);

  const handleKeyDown = (e: KeyboardEvent) => {
    /** @see AUTOCOMPLETE_QUICK_KEY */
    const { code, ctrlKey, altKey } = e;
    if (code === "Space" && (ctrlKey || altKey)) {
      e.preventDefault();
      setQuickAutocomplete(true);
    }
  };

  useEventListener("keydown", handleKeyDown);

  return setQuickAutocomplete;
};
