import { StateProvider } from "../state/common/StateContext";
import { GraphStateProvider } from "../state/graph/GraphStateContext";
import { CardinalityStateProvider } from "../state/cardinality/CardinalityStateContext";
import { TopQueriesStateProvider } from "../state/topQueries/TopQueriesStateContext";
import { SnackbarProvider } from "./Snackbar";

import { combineComponents } from "../utils/combine-components";

const providers = [
  StateProvider, /* Serialized into query string, common app settings */
  GraphStateProvider, /* Chart settings */
  CardinalityStateProvider, /* Cardinality settings */
  TopQueriesStateProvider, /* Top Queries settings */
  SnackbarProvider /* Display various snackbars */
];

export default combineComponents(...providers);
