import { useMemo } from "preact/compat";
import { LegendItemType } from "../../../../../types";
import { QueryGroup } from "../Legend";

type Props = {
  labels: LegendItemType[];
  query: string[];
  groupByLabel?: string;
}

export const useGroupSeries = ({ labels, query, groupByLabel }: Props) => {
  return useMemo(() => {
    return getGroupSeries(labels, query, groupByLabel);
  }, [labels, query, groupByLabel]);
};

const getGroupSeries = (
  labels: LegendItemType[],
  query: string[],
  groupByLabel?: string
): QueryGroup[] => {
  const groupMap = new Map<string | number, QueryGroup>();

  for (const label of labels) {
    const groupKey = groupByLabel
      ? `${groupByLabel}="${label.freeFormFields[groupByLabel] ?? ""}"`
      : `${query[label.group - 1]}`;

    if (!groupMap.has(groupKey)) {
      groupMap.set(groupKey, { group: String(groupKey), items: [] });
    }

    const groupEntry = groupMap.get(groupKey) as QueryGroup;
    groupEntry.items.push(label);
  }

  return Array.from(groupMap.values());
};
