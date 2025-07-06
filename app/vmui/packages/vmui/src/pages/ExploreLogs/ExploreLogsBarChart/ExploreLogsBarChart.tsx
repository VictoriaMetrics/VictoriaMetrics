import { FC, useCallback, useMemo } from "preact/compat";
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
import LineLoader from "../../../components/Main/LineLoader/LineLoader";
import { useSearchParams } from "react-router-dom";
import { getHitsTimeParams } from "../../../utils/logs";

interface Props {
  query: string;
  logHits: LogHits[];
  period: TimeParams;
  error?: string;
  isLoading: boolean;
  onApplyFilter: (value: string) => void;
}

const ExploreLogsBarChart: FC<Props> = ({ logHits, period, error, isLoading, onApplyFilter }) => {
  const { isMobile } = useDeviceDetect();
  const timeDispatch = useTimeDispatch();
  const [searchParams] = useSearchParams();
  const hideChart = useMemo(() => searchParams.get("hide_chart"), [searchParams]);

  const getYAxes = (logHits: LogHits[], timestamps: number[]) => {
    return logHits.map(hits => {
      const timestampValueMap = new Map();
      hits.timestamps.forEach((ts, idx) => {
        const unixTime = dayjs(ts).unix();
        timestampValueMap.set(unixTime, hits.values[idx] || null);
      });

      return timestamps.map(t => timestampValueMap.get(t) || null);
    });
  };

  const generateTimestamps = useCallback((date: dayjs.Dayjs) => {
    const result: number[] = [];
    const { start, end, step } = getHitsTimeParams(period);
    const stepsToFirstTimestamp = Math.ceil(start.diff(date, "milliseconds") / step);
    let firstTimestamp = date.add(stepsToFirstTimestamp * step, "milliseconds");

    // If the first timestamp is before 'start', set it to 'start'
    if (firstTimestamp.isBefore(start)) {
      firstTimestamp = start.clone();
    }

    // Calculate the total number of steps from 'firstTimestamp' to 'end'
    const totalSteps = Math.floor(end.diff(firstTimestamp, "milliseconds") / step);

    for (let i = 0; i <= totalSteps; i++) {
      result.push(firstTimestamp.add(i * step, "milliseconds").unix());
    }

    return result;
  }, [period]);

  const data = useMemo(() => {
    if (!logHits.length) return [[], []] as AlignedData;
    const xAxis = generateTimestamps(dayjs(logHits[0].timestamps[0]));
    const yAxes = getYAxes(logHits, xAxis);
    return [xAxis, ...yAxes] as AlignedData;
  }, [logHits]);

  const noDataMessage: string = useMemo(() => {
    if (isLoading) return "";

    const noData = data.every(d => d.length === 0);
    const noTimestamps = data[0].length === 0;
    const noValues = data[1].length === 0;
    if (hideChart) {
      return "Chart hidden. Hits updates paused.";
    } else if (noData) {
      return "No logs volume available\nNo volume information available for the current queries and time range.";
    } else if (noTimestamps) {
      return "No timestamp information available for the current queries and time range.";
    } else if (noValues) {
      return "No value information available for the current queries and time range.";
    } return "";
  }, [data, hideChart, isLoading]);

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
      {isLoading && <LineLoader/>}
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
          logHits={logHits}
          data={data}
          period={period}
          setPeriod={setPeriod}
          onApplyFilter={onApplyFilter}
        />
      )}
    </section>
  );
};

export default ExploreLogsBarChart;
