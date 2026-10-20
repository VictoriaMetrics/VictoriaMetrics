import { QueryUpdater } from "./types";
import { escapeLabelName } from "../../utils/metric";
import dayjs from "dayjs";
import { TenantPeriodContext } from "../../router";
import { TimeParams } from "../../types";
import { DATE_FORMAT } from "../../constants/date";

export const queryUpdater: QueryUpdater = {
  seriesCountByMetricName: ({ query }): string => {
    return getSeriesSelector("__name__", query);
  },
  seriesCountByLabelName: ({ query }): string => {
    return `{${query}!=""}`;
  },
  seriesCountByFocusLabelValue: ({ query, focusLabel }): string => {
    return getSeriesSelector(focusLabel, query);
  },
  seriesCountByLabelValuePair: ({ query }): string => {
    const a = query.split("=");
    const label = a[0];
    const value = a.slice(1).join("=");
    return getSeriesSelector(label, value);
  },
  labelValueCountByLabelName: ({ query, match }): string => {
    if (match === "") {
      return `{${query}!=""}`;
    }
    return `${match.replace("}", "")}, ${query}!=""}`;
  },
};

const getSeriesSelector = (label: string | null, value: string): string => {
  if (!label) {
    return "";
  }
  return "{" + escapeLabelName(label) + "=" + JSON.stringify(value) + "}";
};

export const getCardinalityTenantPeriod = ({ searchParams }: TenantPeriodContext): TimeParams => {
  const raw = searchParams.get("date") || dayjs().tz().format(DATE_FORMAT);
  const date = dayjs.utc(raw).startOf("day");

  return {
    date: date.format(DATE_FORMAT),
    start: date.unix(),
    end: Math.min(date.endOf("day").valueOf(), Date.now()) / 1000,
  };
};
