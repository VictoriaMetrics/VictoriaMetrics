import { useTopQueriesState } from "../../../state/topQueries/TopQueriesStateContext";
import { useEffect } from "react";
import { compactObject } from "../../../utils/object";
import { useSearchParams } from "react-router-dom";

export const useSetQueryParams = () => {
  const { topN, maxLifetime } = useTopQueriesState();
  const [, setSearchParams] = useSearchParams();

  const setSearchParamsFromState = () => {
    const params = compactObject({
      topN: String(topN),
      maxLifetime: maxLifetime,
    });

    setSearchParams(params);
  };

  useEffect(setSearchParamsFromState, [topN, maxLifetime]);
  useEffect(setSearchParamsFromState, []);
};
