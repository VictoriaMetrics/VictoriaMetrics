import { FC, useMemo } from "preact/compat";
import Select from "../../Main/Select/Select";
import { SearchIcon } from "../../Main/Icons";
import TextField from "../../Main/TextField/TextField";
import "./style.scss";
import classNames from "classnames";
import useDeviceDetect from "../../../hooks/useDeviceDetect";

interface RulesHeaderProps {
  types: string[];
  allTypes: string[];
  allStates: string[];
  states: string[];
  search: string;
  onChangeTypes: (input: string) => void;
  onChangeStates: (input: string) => void;
  onChangeSearch: (input: string) => void;
}

const RulesHeader: FC<RulesHeaderProps> = ({
  types,
  allTypes,
  allStates,
  states,
  search,
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
            itemClassName="vm-badge-menu-item"
            value={states}
            list={allStates}
            label="State"
            placeholder="Please select rule state"
            onChange={onChangeStates}
            noOptionsText={noStateText}
            includeAll
            searchable
          />
        </div>
        <div className="vm-explore-alerts-header-search">
          <TextField
            label="Search"
            value={search}
            placeholder="Filter by rule, name or labels"
            startIcon={<SearchIcon />}
            onChange={onChangeSearch}
          />
        </div>
      </div>
    </>
  );
};

export default RulesHeader;
