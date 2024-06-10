import { useAppState } from "../../../state/common/StateContext";
import { useState } from "react";
import { ErrorTypes } from "../../../types";
import { getExpandWithExprUrl } from "../../../api/expand-with-exprs";
import { useSearchParams } from "react-router-dom";

export const useExpandWithExprs = () => {
  const { serverUrl } = useAppState();
  const [searchParams, setSearchParams] = useSearchParams();

  const [data, setData] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();

  const fetchData = async (query: string) => {
    searchParams.set("expr", query);
    setSearchParams(searchParams);
    const fetchUrl = getExpandWithExprUrl(serverUrl, query);
    setLoading(true);
    try {
      const response = await fetch(fetchUrl);

      const resp = await response.json();

      setData(resp?.expr || "");
      setError(String(resp.error || ""));
    } catch (e) {
      if (e instanceof Error && e.name !== "AbortError") {
        setError(`${e.name}: ${e.message}`);
      }
    }
    setLoading(false);
  };

  return {
    data,
    error,
    loading,
    expand: fetchData
  };
};
