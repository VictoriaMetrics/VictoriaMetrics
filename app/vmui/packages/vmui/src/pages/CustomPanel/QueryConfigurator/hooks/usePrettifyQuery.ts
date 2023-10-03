import { useAppState } from "../../../../state/common/StateContext";

export interface PrettyQuery {
  query: string;
  error: string;
}

export const usePrettifyQuery = () => {
  const { serverUrl } = useAppState();

  const getPrettifiedQuery = async (query: string): Promise<PrettyQuery> => {
    try {
      const oldQuery = encodeURIComponent(query);
      const fetchUrl = `${serverUrl}/prettify-query?query=${oldQuery}`;

      // {"status": "success", "query": "metrics"}
      // {"status": "error", "msg": "labelFilterExpr: unexpected token ..."}
      const response = await fetch(fetchUrl);

      if (response.status != 200) {
        return {
          query: query,
          error: "Error requesting /prettify-query, status: " + response.status,
        };
      }

      const data = await response.json();

      if (data["status"] != "success") {
        return {
          query: query,
          error: String(data.msg) };
      }

      return {
        query: String(data.query),
        error: "" };

    } catch (e) {
      console.error(e);
      if (e instanceof Error && e.name !== "AbortError") {
        return { query: query, error: `${e.name}: ${e.message}` };
      }
      return { query: query, error: String(e) };
    }
  };

  return getPrettifiedQuery;
};
