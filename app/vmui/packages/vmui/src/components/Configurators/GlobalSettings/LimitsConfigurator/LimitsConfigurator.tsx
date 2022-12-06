import React, { FC, useState } from "preact/compat";
import { DisplayType, ErrorTypes, SeriesLimits } from "../../../../types";
import TextField from "../../../Main/TextField/TextField";
import Tooltip from "../../../Main/Tooltip/Tooltip";
import { InfoIcon, RestartIcon } from "../../../Main/Icons";
import Button from "../../../Main/Button/Button";
import { DEFAULT_MAX_SERIES } from "../../../../constants/graph";
import "./style.scss";

export interface ServerConfiguratorProps {
  limits: SeriesLimits
  onChange: (limits: SeriesLimits) => void
  onEnter: () => void
}

const fields: {label: string, type: DisplayType}[] = [
  { label: "Graph", type: "chart" },
  { label: "JSON", type: "code" },
  { label: "Table", type: "table" }
];

const LimitsConfigurator: FC<ServerConfiguratorProps> = ({ limits, onChange , onEnter }) => {

  const [error, setError] = useState({
    table: "",
    chart: "",
    code: ""
  });

  const handleChange = (val: string, type: DisplayType) => {
    const value = val || "";
    setError(prev => ({ ...prev, [type]: +value < 0 ? ErrorTypes.positiveNumber : "" }));
    onChange({
      ...limits,
      [type]: !value ? Infinity : value
    });
  };

  const handleReset = () => {
    onChange(DEFAULT_MAX_SERIES);
  };

  const createChangeHandler = (type: DisplayType) =>  (val: string) => {
    handleChange(val, type);
  };

  return (
    <div className="vm-limits-configurator">
      <div className="vm-server-configurator__title">
        Series limits by tabs
        <Tooltip title="To disable limits set to 0">
          <Button
            variant="text"
            color="primary"
            size="small"
            startIcon={<InfoIcon/>}
          />
        </Tooltip>
        <div className="vm-limits-configurator-title__reset">
          <Button
            variant="text"
            color="primary"
            size="small"
            startIcon={<RestartIcon/>}
            onClick={handleReset}
          >
            Reset
          </Button>
        </div>
      </div>
      <div className="vm-limits-configurator__inputs">
        {fields.map(f => (
          <TextField
            key={f.type}
            label={f.label}
            value={limits[f.type]}
            error={error[f.type]}
            onChange={createChangeHandler(f.type)}
            onEnter={onEnter}
            type="number"
          />
        ))}
      </div>
    </div>
  );
};

export default LimitsConfigurator;
