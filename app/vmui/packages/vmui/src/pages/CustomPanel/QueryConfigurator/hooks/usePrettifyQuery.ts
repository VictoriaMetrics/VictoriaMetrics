import { useAppState } from "../../../../state/common/StateContext";
import { StateUpdater } from "preact/compat";

export const usePrettifyQuery = (
  stateQuery: string[],
  setStateQuery: StateUpdater<string[]>,
  queryErrors: string[],
  setQueryErrors: StateUpdater<string[]>,
) => {

  const { serverUrl } = useAppState();

  const handlePrettifyQuery = async (i: number) => {
    const oldQuery = encodeURIComponent(stateQuery[i]);
    let response: Response;
    try {
      response = await fetch(`${serverUrl}/prettify-query?query=${oldQuery}`);
    } catch (e) {
      const newQueryErrors = [...queryErrors];
      newQueryErrors[i] = `${e}`;
      setQueryErrors(newQueryErrors);
      return;
    }

    if (response.status != 200) {
      const newQueryErrors = [...queryErrors];
      newQueryErrors[i] = "Error requesting /prettify-query, status: " + response.status;
      setQueryErrors(newQueryErrors);
    }

    const data = await response.json();

    if (data["status"] == "success") {
      const newQueryErrors = [...queryErrors];
      newQueryErrors[i] = "";
      setQueryErrors(newQueryErrors);

      const newStateQuery = [...stateQuery];
      newStateQuery[i] = data["query"];
      setStateQuery(newStateQuery);
    } else {
      const newQueryErrors = [...queryErrors];
      newQueryErrors[i] = data["msg"];
      setQueryErrors(newQueryErrors);
    }
  };

  return { handlePrettifyQuery: handlePrettifyQuery };
};
