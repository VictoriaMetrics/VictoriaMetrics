import dayjs from "dayjs";
import { Group, Rule } from "../../types";

// VMALERT_SOURCE_LABEL is the meta-label added by vmselect to every group
// returned by a fan-out request to multiple -vmalert.proxyURL.
// See lib/vmalertproxy.SourceLabel.
export const VMALERT_SOURCE_LABEL = "__vmalert_source";

// getVMAlertSource returns the name of the vmalert, which owns the given group.
//
// It returns an empty string if a single vmalert is configured at -vmalert.proxyURL,
// since vmselect doesn't add VMALERT_SOURCE_LABEL in this case.
export const getVMAlertSource = (group?: Group): string =>
  group?.labels?.[VMALERT_SOURCE_LABEL] || "";

// GROUP_SOURCE_PARAM carries the vmalert owning the group, rule or alert opened by a vmui link.
//
// It is deliberately distinct from the `vmalert_source` filter arg. Sharing a single arg for
// both purposes makes them fight over the URL: the filter clears the arg as soon as no source
// is selected, which would drop the source from entity links, while a link setting the arg
// would silently apply a filter the user never chose.
export const GROUP_SOURCE_PARAM = "group_source";

// groupSourceParam returns the GROUP_SOURCE_PARAM query arg for the given source.
// An empty source yields an empty string, since a single-vmalert setup needs no routing hint.
export const groupSourceParam = (source: string): string =>
  source ? `&${GROUP_SOURCE_PARAM}=${encodeURIComponent(source)}` : "";

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
  return t.year() <= 1 ? "Never" : t.tz().format("DD MMM YYYY HH:mm:ss");
};

export const getStates = (rule: Rule) => {
  if (!rule.alerts?.length) {
    return { [rule.state]: 1 };
  }
  return rule.alerts.reduce((acc, alert) => {
    acc[alert.state] = (acc[alert.state] ?? 0) + 1;
    return acc;
  }, {} as Record<string, number>);
};
