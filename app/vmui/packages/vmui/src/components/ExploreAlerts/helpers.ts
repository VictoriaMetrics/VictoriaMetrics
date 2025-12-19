import dayjs from "dayjs";
import { Rule } from "../../types";

export const formatDuration = (raw: number) => {
  const duration = dayjs.duration(Math.round(raw * 1000));
  const fmt = [];
  if (duration.get("day")) fmt.push("D[d]");
  if (duration.get("hour")) fmt.push("H[h]");
  if (duration.get("minute")) fmt.push("m[m]");
  if (duration.get("millisecond")) {
    fmt.push("s.SSS[s]");
  } else if (!fmt.length || duration.get("second")) {
    fmt.push("s[s]");
  }
  return duration.format(fmt.join(" "));
};

export const formatEventTime = (raw: string) => {
  const t = dayjs(raw);
  return t.year() <= 1 ? "Never" : t.format("DD MMM YYYY HH:mm:ss");
}

export const getStates = (rule: Rule) => {
  if (!rule.alerts?.length) {
    return { [rule.state]: 1 };
  }
  return rule.alerts.reduce((acc, alert) => {
    acc[alert.state] = (acc[alert.state] ?? 0) + 1;
    return acc;
  }, {} as Record<string, number>);
};
