import { AppStateProvider } from "../state/common/StateContext";
import { TimeStateProvider } from "../state/time/TimeStateContext";
import { QueryStateProvider } from "../state/query/QueryStateContext";
import { CustomPanelStateProvider } from "../state/customPanel/CustomPanelStateContext";
import { GraphStateProvider } from "../state/graph/GraphStateContext";
import { CardinalityStateProvider } from "../state/cardinality/CardinalityStateContext";
import { TopQueriesStateProvider } from "../state/topQueries/TopQueriesStateContext";
import { SnackbarProvider } from "./Snackbar";

import { combineComponents } from "../utils/combine-components";
import { DashboardsStateProvider } from "../state/dashboards/DashboardsStateContext";

const providers = [
  AppStateProvider,
  TimeStateProvider,
  QueryStateProvider,
  CustomPanelStateProvider,
  GraphStateProvider,
  CardinalityStateProvider,
  TopQueriesStateProvider,
  SnackbarProvider,
  DashboardsStateProvider
];

export default combineComponents(...providers);
