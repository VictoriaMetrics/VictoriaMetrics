import { FC, useMemo } from "preact/compat";
import Select from "../../Main/Select/Select";
import { SearchIcon } from "../../Main/Icons";
import TextField from "../../Main/TextField/TextField";
import "./style.scss";
import classNames from "classnames";
import useDeviceDetect from "../../../hooks/useDeviceDetect";

interface RulesHeaderProps {
  ruleTypes: string[]
  ruleTypeFilter: string
  allRuleTypes: string[]
  allStates: string[]
  states: string[]
  onChangeRuleTypes: (input: string) => void
  onChangeStates: (input: string) => void
  onChangeSearch: (input: string) => void
}

const RulesHeader: FC<RulesHeaderProps> = ({
  ruleTypes,
  ruleTypeFilter,
  allRuleTypes,
  allStates,
  states,
  onChangeRuleTypes,
  onChangeStates,
  onChangeSearch,
}) => {
  const noStateText = useMemo(() => ruleTypes.length ? "" : "No states. Please select rule states", [ruleTypes]);
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
        {ruleTypeFilter === "" && (
          <div className="vm-explore-alerts-header__rule_type">
            <Select
              value={ruleTypes}
              list={allRuleTypes}
              label="Rules type"
              placeholder="Please select rule type"
              onChange={onChangeRuleTypes}
              autofocus={!!ruleTypes.length && !isMobile}
              includeAll
              searchable
            />
          </div>
        )}
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
            startIcon={<SearchIcon/>}
            onChange={onChangeSearch}
          />
        </div>
      </div>
    </>
  );
};

export default RulesHeader;
