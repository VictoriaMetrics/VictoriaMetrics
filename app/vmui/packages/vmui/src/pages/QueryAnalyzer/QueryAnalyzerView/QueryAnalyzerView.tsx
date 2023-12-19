import React, { FC, useMemo, useState, useEffect } from "preact/compat";
import Trace from "../../../components/TraceQuery/Trace";
import { DataAnalyzerType } from "../index";
import classNames from "classnames";
import { DisplayType, displayTypeTabs } from "../../CustomPanel/DisplayTypeSwitch";
import GraphTips from "../../../components/Chart/GraphTips/GraphTips";
import GraphSettings from "../../../components/Configurators/GraphSettings/GraphSettings";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import { AxisRange } from "../../../state/graph/reducer";
import { useGraphDispatch, useGraphState } from "../../../state/graph/GraphStateContext";
import Tabs from "../../../components/Main/Tabs/Tabs";
import TracingsView from "../../../components/TraceQuery/TracingsView";
import "./style.scss";
import GraphView from "../../../components/Views/GraphView/GraphView";
import JsonView from "../../../components/Views/JsonView/JsonView";
import { InstantMetricResult, MetricResult } from "../../../api/types";
import { isHistogramData } from "../../../utils/metric";
import { TimeParams } from "../../../types";
import { dateFromSeconds, formatDateToUTC, humanizeSeconds } from "../../../utils/time";
import { findMostCommonStep } from "./utils";
import TableSettings from "../../../components/Table/TableSettings/TableSettings";
import { getColumns } from "../../../hooks/useSortedCategories";
import { useCustomPanelDispatch, useCustomPanelState } from "../../../state/customPanel/CustomPanelStateContext";
import TableView from "../../../components/Views/TableView/TableView";

type Props = {
  data: DataAnalyzerType[]
}

const QueryAnalyzerView: FC<Props> = ({ data }) => {
  const { isMobile } = useDeviceDetect();
  const { tableCompact } = useCustomPanelState();
  const customPanelDispatch = useCustomPanelDispatch();

  const [traces, setTraces] = useState<Trace[]>([]);
  const [graphData, setGraphData] = useState<MetricResult[]>();
  const [liveData, setLiveData] = useState<InstantMetricResult[]>();
  const [isHistogram, setIsHistogram] = useState(false);
  const [queries, setQueries] = useState<string[]>([]);
  const [displayColumns, setDisplayColumns] = useState<string[]>();

  const columns = useMemo(() => getColumns(liveData || []).map(c => c.key), [liveData]);

  const tabs = useMemo(() => {
    const hasInstantQuery = data.some(d => d.data.resultType === "vector");
    if (hasInstantQuery) return displayTypeTabs;
    return displayTypeTabs.filter(t => t.value === "chart");
  }, [data]);
  const [displayType, setDisplayType] = useState(tabs[0].value);

  const period: TimeParams = useMemo(() => {
    const params = data[0]?.vmui?.params;

    const result = {
      start: +(params?.start || 0),
      end: +(params?.end || 0),
      step: params?.step,
      date: ""
    };

    if (!params) {
      const dataResult = data.filter(d => d.data.resultType === "matrix").map(d => d.data.result).flat();
      const times = dataResult.map(r => r.values ? r.values?.map(v => v[0]) : [0]).flat();
      const uniqTimes = Array.from(new Set(times.filter(Boolean))).sort((a, b) => a - b);
      const step = humanizeSeconds(findMostCommonStep(uniqTimes));
      result.start = uniqTimes[0];
      result.end = uniqTimes[uniqTimes.length - 1];
      result.step = step;
    }

    result.date = formatDateToUTC(dateFromSeconds(result.end));
    return result;
  }, [data]);

  const { yaxis } = useGraphState();
  const graphDispatch = useGraphDispatch();

  const setYaxisLimits = (limits: AxisRange) => {
    graphDispatch({ type: "SET_YAXIS_LIMITS", payload: limits });
  };

  const toggleEnableLimits = () => {
    graphDispatch({ type: "TOGGLE_ENABLE_YAXIS_LIMITS" });
  };

  const handleChangeDisplayType = (newValue: string) => {
    setDisplayType(newValue as DisplayType);
  };

  const handleTraceDelete = (trace: Trace) => {
    setTraces(prev => prev.filter((data) => data.idValue !== trace.idValue));
  };

  const toggleTableCompact = () => {
    customPanelDispatch({ type: "TOGGLE_TABLE_COMPACT" });
  };

  useEffect(() => {
    const resultType = displayType === "chart" ? "matrix" : "vector";
    const traces = data.filter(d => d.data.resultType === resultType && d.trace)
      .map(d => d.trace ? new Trace(d.trace, d?.vmui?.params?.query || "Query") : null);
    setTraces(traces.filter(Boolean) as Trace[]);
  }, [data, displayType]);

  useEffect(() => {
    const tempQueries: string[] = [];
    const tempGraphData: MetricResult[] = [];
    const tempLiveData: InstantMetricResult[] = [];

    data.forEach((d, i) => {
      const result = d.data.result.map((r) => ({ ...r, group: Number(d.vmui?.params?.id ?? i) + 1 }));
      if (d.data.resultType === "matrix") {
        tempGraphData.push(...result as MetricResult[]);
        tempQueries.push(d.vmui?.params?.query || "Query");
      } else {
        tempLiveData.push(...result as InstantMetricResult[]);
      }
    });

    setQueries(tempQueries);
    setGraphData(tempGraphData);
    setLiveData(tempLiveData);
  }, [data]);

  useEffect(() => {
    setIsHistogram(!!graphData && isHistogramData(graphData));
  }, [graphData]);

  return (
    <div
      className={classNames({
        "vm-query-analyzer-view": true,
        "vm-query-analyzer-view_mobile": isMobile,
      })}
    >
      {!!traces.length && (
        <TracingsView
          traces={traces}
          onDeleteClick={handleTraceDelete}
        />
      )}
      <div
        className={classNames({
          "vm-block": true,
          "vm-block_mobile": isMobile,
        })}
      >
        <div className="vm-custom-panel-body-header">
          <Tabs
            activeItem={displayType}
            items={tabs}
            onChange={handleChangeDisplayType}
          />
          <div className="vm-custom-panel-body-header__left">
            {displayType === "chart" && <GraphTips/>}
            {displayType === "chart" && (
              <GraphSettings
                yaxis={yaxis}
                setYaxisLimits={setYaxisLimits}
                toggleEnableLimits={toggleEnableLimits}
              />
            )}
            {displayType === "table" && (
              <TableSettings
                columns={columns}
                defaultColumns={displayColumns}
                onChangeColumns={setDisplayColumns}
                tableCompact={tableCompact}
                toggleTableCompact={toggleTableCompact}
              />
            )}
          </div>
        </div>
        {graphData && period && (displayType === "chart") && (
          <GraphView
            data={graphData}
            period={period}
            customStep={period.step || "1s"}
            query={queries}
            yaxis={yaxis}
            setYaxisLimits={setYaxisLimits}
            setPeriod={() => null}
            height={isMobile ? window.innerHeight * 0.5 : 500}
            isHistogram={isHistogram}
          />
        )}
        {liveData && (displayType === "code") && (
          <JsonView data={liveData}/>
        )}
        {liveData && (displayType === "table") && (
          <TableView
            data={liveData}
            displayColumns={displayColumns}
          />
        )}
      </div>
    </div>
  );
};

export default QueryAnalyzerView;
