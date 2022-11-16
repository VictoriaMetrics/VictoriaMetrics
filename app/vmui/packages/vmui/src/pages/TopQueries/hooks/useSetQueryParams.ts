import { useTopQueriesState } from "../../../state/topQueries/TopQueriesStateContext";
import { useEffect } from "react";
import { compactObject } from "../../../utils/object";
import { setQueryStringWithoutPageReload } from "../../../utils/query-string";

export const useSetQueryParams = () => {
  const { topN, maxLifetime } = useTopQueriesState();

  const setSearchParamsFromState = () => {
    const params = compactObject({
      topN: String(topN),
      maxLifetime: maxLifetime,
    });

    setQueryStringWithoutPageReload(params);
  };

  useEffect(setSearchParamsFromState, [topN, maxLifetime]);
  useEffect(setSearchParamsFromState, []);
};
