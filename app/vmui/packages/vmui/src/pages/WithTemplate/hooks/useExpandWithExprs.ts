import { useAppState } from "../../../state/common/StateContext";
import { useState } from "react";
import { ErrorTypes } from "../../../types";
import { getExpandWithExprUrl } from "../../../api/expand-with-exprs";

export const useExpandWithExprs = () => {
  const { serverUrl } = useAppState();

  const [data, setData] = useState("");
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<ErrorTypes | string>();


  const fetchData = async (query: string) => {
    const fetchUrl = getExpandWithExprUrl(serverUrl, query);
    setLoading(true);
    try {
      const response = await fetch(fetchUrl);
      const resp = await response.json();
      console.log(resp);

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
