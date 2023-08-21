import { useEffect } from "react";
import { useTimeState } from "../../../state/time/TimeStateContext";
import { useCustomPanelState } from "../../../state/customPanel/CustomPanelStateContext";
import { useAppState } from "../../../state/common/StateContext";
import { useQueryState } from "../../../state/query/QueryStateContext";
import { displayTypeTabs } from "../DisplayTypeSwitch";
import { compactObject } from "../../../utils/object";
import { useGraphState } from "../../../state/graph/GraphStateContext";
import { useSearchParams } from "react-router-dom";

export const useSetQueryParams = () => {
  const { tenantId } = useAppState();
  const { displayType } = useCustomPanelState();
  const { query } = useQueryState();
  const { duration, relativeTime, period: { date, step } } = useTimeState();
  const { customStep } = useGraphState();
  const [, setSearchParams] = useSearchParams();

  const setSearchParamsFromState = () => {
    const params: Record<string, unknown> = {};
    query.forEach((q, i) => {
      const group = `g${i}`;
      params[`${group}.expr`] = q;
      params[`${group}.range_input`] = duration;
      params[`${group}.end_input`] = date;
      params[`${group}.tab`] = displayTypeTabs.find(t => t.value === displayType)?.prometheusCode || 0;
      params[`${group}.relative_time`] = relativeTime;
      params[`${group}.tenantID`] = tenantId;

      if ((step !== customStep) && customStep) params[`${group}.step_input`] = customStep;
    });

    setSearchParams(compactObject(params) as Record<string, string>);
  };

  useEffect(setSearchParamsFromState, [tenantId, displayType, query, duration, relativeTime, date, step, customStep]);
  useEffect(setSearchParamsFromState, []);
};
