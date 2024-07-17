import React, { FC, useMemo } from "preact/compat";
import "./style.scss";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import classNames from "classnames";
import { LogHits } from "../../../api/types";
import dayjs from "dayjs";
import { useTimeDispatch } from "../../../state/time/TimeStateContext";
import { AlignedData } from "uplot";
import BarHitsChart from "../../../components/Chart/BarHitsChart/BarHitsChart";
import Alert from "../../../components/Main/Alert/Alert";
import { TimeParams } from "../../../types";
import Spinner from "../../../components/Main/Spinner/Spinner";

interface Props {
  query: string;
  logHits: LogHits[];
  period: TimeParams;
  error?: string;
  isLoading: boolean;
}

const ExploreLogsBarChart: FC<Props> = ({ logHits, period, error, isLoading }) => {
  const { isMobile } = useDeviceDetect();
  const timeDispatch = useTimeDispatch();

  const data = useMemo(() => {
    const hits = logHits[0];
    if (!hits) return [[], []] as AlignedData;
    const { values, timestamps } = hits;
    const xAxis = timestamps.map(t => t ? dayjs(t).unix() : null).filter(Boolean);
    const yAxis = values.map(v => v || null);
    return [xAxis, yAxis] as AlignedData;
  }, [logHits]);

  const noDataMessage: string = useMemo(() => {
    const noData = data.every(d => d.length === 0);
    const noTimestamps = data[0].length === 0;
    const noValues = data[1].length === 0;
    if (noData) {
      return "No logs volume available\nNo volume information available for the current queries and time range.";
    } else if (noTimestamps) {
      return "No timestamp information available for the current queries and time range.";
    } else if (noValues) {
      return "No value information available for the current queries and time range.";
    } return "";
  }, [data]);

  const setPeriod = ({ from, to }: {from: Date, to: Date}) => {
    timeDispatch({ type: "SET_PERIOD", payload: { from, to } });
  };

  return (
    <section
      className={classNames({
        "vm-explore-logs-chart": true,
        "vm-block": true,
        "vm-block_mobile": isMobile,
      })}
    >
      {isLoading && <Spinner containerStyles={{ position: "absolute" }}/>}
      {!error && noDataMessage && (
        <div className="vm-explore-logs-chart__empty">
          <Alert variant="info">{noDataMessage}</Alert>
        </div>
      )}

      {error && noDataMessage && (
        <div className="vm-explore-logs-chart__empty">
          <Alert variant="error">{error}</Alert>
        </div>
      )}

      {data && (
        <BarHitsChart
          data={data}
          period={period}
          setPeriod={setPeriod}
        />
      )}
    </section>
  );
};

export default ExploreLogsBarChart;
