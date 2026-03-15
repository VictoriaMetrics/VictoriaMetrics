import { FC } from "preact/compat";
import { LegendItemType } from "../../../../types";
import "./style.scss";
import LegendGroup from "./LegendGroup";
import { useLegendGroup } from "./hooks/useLegendGroup";
import { useGroupSeries } from "./hooks/useGroupSeries";
import LegendConfigs from "./LegendConfigs/LegendConfigs";

export type QueryGroup = {
  group: number | string;
  items: LegendItemType[]
}

interface LegendProps {
  labels: LegendItemType[];
  query: string[];
  isPredefinedPanel?: boolean;
  onChange: (item: LegendItemType, metaKey: boolean) => void;
}

const Legend: FC<LegendProps> = ({ labels, query, isPredefinedPanel, onChange }) => {
  const { groupByLabel } = useLegendGroup();
  const groupSeries = useGroupSeries({ labels, query, groupByLabel });

  return (
    <div className="vm-legend">
      {!isPredefinedPanel && <LegendConfigs isCompact/>}

      <div>
        {groupSeries.map(({ group, items }) => (
          <LegendGroup
            key={group}
            labels={items}
            group={group}
            onChange={onChange}
          />
        ))}
      </div>
    </div>
  );
};

export default Legend;
