import { FC, useMemo } from "preact/compat";
import Select from "../../Main/Select/Select";
import { SearchIcon, CollapseIcon, ExpandIcon } from "../../Main/Icons";
import Button from "../../Main/Button/Button";
import Tooltip from "../../Main/Tooltip/Tooltip";
import TextField from "../../Main/TextField/TextField";
import "./style.scss";
import classNames from "classnames";
import useDeviceDetect from "../../../hooks/useDeviceDetect";

interface RulesHeaderProps {
  types: string[];
  allTypes: string[];
  allStates: string[];
  states: string[];
  expanded: boolean;
  toggleExpand: () => void;
  onChangeTypes: (input: string) => void;
  onChangeStates: (input: string) => void;
  onChangeSearch: (input: string) => void;
}

const RulesHeader: FC<RulesHeaderProps> = ({
  types,
  allTypes,
  allStates,
  states,
  expanded,
  toggleExpand,
  onChangeTypes,
  onChangeStates,
  onChangeSearch,
}) => {
  const noStateText = useMemo(
    () => (types.length ? "" : "No states. Please select rule states"),
    [types],
  );
  const { isMobile } = useDeviceDetect();

  return (
    <>
      <div
        className={classNames({
          "vm-explore-alerts-header": true,
          "vm-explore-alerts-header_mobile": isMobile,
          "vm-block": true,
          "vm-block_mobile": isMobile,
        })}
      >
        <div className="vm-explore-alerts-header__rule_type">
          <Select
            value={types}
            list={allTypes}
            label="Rules type"
            placeholder="Please select rule type"
            onChange={onChangeTypes}
            autofocus={!!types.length && !isMobile}
            includeAll
            searchable
          />
        </div>
        <div className="vm-explore-alerts-header__state">
          <Select
            value={states}
            list={allStates}
            label="State"
            placeholder="Please rule state"
            onChange={onChangeStates}
            noOptionsText={noStateText}
            includeAll
            searchable
          />
        </div>
        <div className="vm-explore-alerts-header-search">
          <TextField
            label="Search"
            placeholder="Fitler by rule, name or labels"
            startIcon={<SearchIcon />}
            onChange={onChangeSearch}
          />
        </div>
        <Tooltip title={expanded ? "Collapse All" : "Expand All"}>
          <Button
            variant="text"
            color="gray"
            startIcon={expanded ? <CollapseIcon/> : <ExpandIcon/> }
            onClick={toggleExpand}
            ariaLabel={expanded ? "Collapse All" : "Expand All"}
          />
        </Tooltip>
      </div>
    </>
  );
};

export default RulesHeader;
