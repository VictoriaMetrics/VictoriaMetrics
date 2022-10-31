import { useEffect } from "react";
import { useTimeState } from "../../../state/time/TimeStateContext";
import { useCustomPanelState } from "../../../state/customPanel/CustomPanelStateContext";
import { useAppState } from "../../../state/common/StateContext";
import { useQueryState } from "../../../state/query/QueryStateContext";
import { displayTypeTabs } from "../DisplayTypeSwitch";
import { setQueryStringWithoutPageReload } from "../../../utils/query-string";
import { compactObject } from "../../../utils/object";

export const useSetQueryParams = () => {
  const { tenantId } = useAppState();
  const { displayType } = useCustomPanelState();
  const { query } = useQueryState();
  const { duration, relativeTime, period: { date, step } } = useTimeState();

  const setSearchParamsFromState = () => {
    const params: Record<string, unknown> = {};
    query.forEach((q, i) => {
      const group = `g${i}`;
      params[`${group}.expr`] = q;
      params[`${group}.range_input`] = duration;
      params[`${group}.end_input`] = date;
      params[`${group}.step_input`] = step;
      params[`${group}.tab`] = displayTypeTabs.find(t => t.value === displayType)?.prometheusCode || 0;
      params[`${group}.relative_time`] = relativeTime;
      params[`${group}.tenantID`] = tenantId;
    });

    setQueryStringWithoutPageReload(compactObject(params));
  };

  useEffect(setSearchParamsFromState, [tenantId, displayType, query, duration, relativeTime, date, step]);
  useEffect(setSearchParamsFromState, []);
};
