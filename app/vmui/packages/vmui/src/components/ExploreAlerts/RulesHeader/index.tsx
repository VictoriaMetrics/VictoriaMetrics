import { useMemo } from "preact/compat";
import Select from "../../Main/Select/Select";
import { RefreshIcon, SearchIcon } from "../../Main/Icons";
import TextField from "../../Main/TextField/TextField";
import "./style.scss";
import classNames from "classnames";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import Button from "../../Main/Button/Button";
import { useTimeDispatch } from "../../../state/time/TimeStateContext";

interface RulesHeaderProps {
  types: string[];
  allRuleTypes: string[];
  allStates: string[];
  states: string[];
  search: string;
  onChangeRuleType: (input: string) => void;
  onChangeStates: (input: string) => void;
  onChangeSearch: (input: string) => void;
}

const RulesHeader = ({
  types,
  allRuleTypes,
  allStates,
  states,
  search,
  onChangeRuleType,
  onChangeStates,
  onChangeSearch,
}: RulesHeaderProps) => {
  const noStateText = useMemo(
    () => (types.length ? "" : "No states. Please select rule states"),
    [types],
  );
  const { isMobile } = useDeviceDetect();
  const dispatch = useTimeDispatch();

  const handleRefresh = () => {
    dispatch({ type: "RUN_QUERY" });
  };

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
            list={allRuleTypes}
            label="Rule type"
            placeholder="Please select rule type"
            onChange={onChangeRuleType}
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
            placeholder="Filter by group or rule name"
            startIcon={<SearchIcon />}
            onChange={onChangeSearch}
          />
          <div>
            <Button
              variant="text"
              onClick={handleRefresh}
              startIcon={<RefreshIcon />}
              ariaLabel="Refresh"
            >
              Refresh
            </Button>
          </div>
        </div>
      </div>
    </>
  );
};

export default RulesHeader;
