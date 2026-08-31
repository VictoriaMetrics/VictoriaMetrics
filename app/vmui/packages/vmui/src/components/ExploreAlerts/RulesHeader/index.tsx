import { useMemo } from "preact/compat";
import Select from "../../Main/Select/Select";
import { SearchIcon } from "../../Main/Icons";
import TextField from "../../Main/TextField/TextField";
import "./style.scss";
import classNames from "classnames";
import useDeviceDetect from "../../../hooks/useDeviceDetect";

interface RulesHeaderProps {
  types: string[];
  allRuleTypes: string[];
  allStates: string[];
  states: string[];
  sources: string[];
  allSources: string[];
  search: string;
  onChangeRuleType: (input: string) => void;
  onChangeStates: (input: string) => void;
  onChangeSource: (input: string) => void;
  onChangeSearch: (input: string) => void;
}

const RulesHeader = ({
  types,
  allRuleTypes,
  allStates,
  states,
  sources,
  allSources,
  search,
  onChangeRuleType,
  onChangeStates,
  onChangeSource,
  onChangeSearch,
}: RulesHeaderProps) => {
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
        {allSources.length > 1 && (
          <div className="vm-explore-alerts-header__vmalert_source">
            <Select
              value={sources}
              list={allSources}
              label="Source"
              placeholder="Please select vmalert source"
              onChange={onChangeSource}
              includeAll
              searchable
              closeOnSelect
            />
          </div>
        )}
        <div className="vm-explore-alerts-header__rule_type">
          <Select
            value={types}
            list={allRuleTypes}
            label="Rule type"
            placeholder="Please select rule type"
            onChange={onChangeRuleType}
            includeAll
            searchable
            closeOnSelect
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
        </div>
      </div>
    </>
  );
};

export default RulesHeader;
