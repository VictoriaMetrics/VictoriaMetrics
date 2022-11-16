import { useEffect } from "react";
import { useCardinalityState } from "../../../state/cardinality/CardinalityStateContext";
import { compactObject } from "../../../utils/object";
import { setQueryStringWithoutPageReload } from "../../../utils/query-string";

export const useSetQueryParams = () => {
  const { topN, match, date, focusLabel, extraLabel } = useCardinalityState();

  const setSearchParamsFromState = () => {
    const params = compactObject({
      topN,
      date,
      match,
      extraLabel,
      focusLabel,
    });

    setQueryStringWithoutPageReload(params);
  };

  useEffect(setSearchParamsFromState, [topN, match, date, focusLabel, extraLabel]);
  useEffect(setSearchParamsFromState, []);
};
