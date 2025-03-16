import React, { FC, useMemo } from "preact/compat";
import Switch from "../../../../Main/Switch/Switch";
import { LegendDisplayType, useLegendView } from "../hooks/useLegendView";
import { useHideDuplicateFields } from "../hooks/useHideDuplicateFields";
import { useShowStats } from "../hooks/useShowStats";
import TextField from "../../../../Main/TextField/TextField";
import { useLegendFormat } from "../hooks/useLegendFormat";
import { WITHOUT_GROUPING } from "../../../../../constants/logs";
import Select from "../../../../Main/Select/Select";
import { useLegendGroup } from "../hooks/useLegendGroup";
import "./style.scss";
import { MetricResult } from "../../../../../api/types";
import classNames from "classnames";
import Button from "../../../../Main/Button/Button";
import { SettingsIcon } from "../../../../Main/Icons";
import { useGraphDispatch } from "../../../../../state/graph/GraphStateContext";

type Props = {
  data?: MetricResult[]
  isCompact?: boolean
}

const LegendConfigs: FC<Props> = ({ data, isCompact }) => {
  const { isTableView, onChange: onChangeView } = useLegendView();
  const { hideDuplicates, onChange: onChangeDuplicates } = useHideDuplicateFields();
  const { hideStats, onChange: onChangeStats } = useShowStats();
  const { format, onChange: onChangeFormat, onApply: onApplyFormat } = useLegendFormat();
  const { groupByLabel, onChange: onChangeGroup } = useLegendGroup();
  const graphDispatch = useGraphDispatch();

  const uniqueFields = useMemo(() => {
    if (!data || !data.length) return [];
    const fields = data.flatMap(d => Object.keys(d.metric));
    return Array.from(new Set(fields));
  }, [data]);

  const handleChangeView = (val: boolean) => {
    const value = val ? LegendDisplayType.table : LegendDisplayType.lines;
    onChangeView(value);
  };

  const handleOpenSettings = () => {
    graphDispatch({ type: "SET_OPEN_SETTINGS", payload: true });
  };

  return (
    <div
      className={classNames({
        "vm-legend-configs": true,
        "vm-legend-configs_compact": isCompact,
      })}
    >
      <div className="vm-legend-configs-item vm-legend-configs-item_switch">
        <span className="vm-legend-configs-item__label">Table View</span>
        <Switch
          label={`${isCompact ? "Table view" : isTableView ? "Enabled" : "Disabled"}`}
          value={isTableView}
          onChange={handleChangeView}
        />
        <span className="vm-legend-configs-item__info">
          Switches between table and lines view.
        </span>
      </div>

      <div className="vm-legend-configs-item vm-legend-configs-item_switch">
        <span className="vm-legend-configs-item__label">Common Labels</span>
        <Switch
          label={`${isCompact ? "Common labels" : hideDuplicates ? "Hide" : "Show"}`}
          value={!hideDuplicates}
          onChange={onChangeDuplicates}
        />
        <span className="vm-legend-configs-item__info">
          Shows or hides labels that are the same for all series.
        </span>
      </div>

      <div className="vm-legend-configs-item vm-legend-configs-item_switch">
        <span className="vm-legend-configs-item__label">Statistics</span>
        <Switch
          label={`${isCompact ? "Statistics" : hideStats ? "Hide" : "Show"}`}
          value={!hideStats}
          onChange={onChangeStats}
        />
        <span className="vm-legend-configs-item__info">
          Displays min, median, and max values.
        </span>
      </div>

      {isCompact && (
        <Button
          size="small"
          variant="text"
          startIcon={<SettingsIcon/>}
          onClick={handleOpenSettings}
        >
          Settings
        </Button>
      )}

      {!isCompact && (
        <>
          <div className="vm-legend-configs-item">
            <TextField
              label="Custom Legend Format"
              placeholder={"{{label_name}}"}
              value={format}
              onChange={onChangeFormat}
              onBlur={onApplyFormat}
              onEnter={onApplyFormat}
            />
            <span className="vm-legend-configs-item__info vm-legend-configs-item__info_input">
          Customize legend labels with text and &#123;&#123;label_name&#125;&#125; placeholders.
            </span>
          </div>

          <div className="vm-legend-configs-item">
            <Select
              label="Group Legend By"
              value={groupByLabel}
              list={[WITHOUT_GROUPING, ...uniqueFields]}
              placeholder={WITHOUT_GROUPING}
              onChange={onChangeGroup}
              searchable
            />
            <span className="vm-legend-configs-item__info">
          Choose a label to group the legend. By default, legends are grouped by query.
            </span>
          </div>
        </>
      )}
    </div>
  );
};

export default LegendConfigs;
