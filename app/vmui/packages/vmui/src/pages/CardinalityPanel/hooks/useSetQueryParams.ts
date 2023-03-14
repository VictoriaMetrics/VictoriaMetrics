import { useEffect } from "react";
import { useCardinalityState } from "../../../state/cardinality/CardinalityStateContext";
import { compactObject } from "../../../utils/object";
import { useSearchParams } from "react-router-dom";

export const useSetQueryParams = () => {
  const { topN, match, date, focusLabel, extraLabel } = useCardinalityState();
  const [, setSearchParams] = useSearchParams();

  const setSearchParamsFromState = () => {
    const params = compactObject({
      topN,
      date,
      match,
      extraLabel,
      focusLabel,
    });

    setSearchParams(params as Record<string, string>);
  };

  useEffect(setSearchParamsFromState, [topN, match, date, focusLabel, extraLabel]);
  useEffect(setSearchParamsFromState, []);
};
