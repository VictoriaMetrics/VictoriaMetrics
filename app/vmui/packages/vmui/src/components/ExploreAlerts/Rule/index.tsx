import { FC } from "preact/compat";
import ItemHeader from "../ItemHeader";
import Accordion from "../../Main/Accordion/Accordion";
import "./style.scss";
import { Rule as APIRule } from "../../../types";
import BaseRule from "../BaseRule";

interface RuleProps {
  state: string;
  rule: APIRule;
  expandRules: Set<string>;
  onRulesChange: (a: boolean) => void;
}

const Rule: FC<RuleProps> = ({ state, rule, expandRules, onRulesChange }) => {
  return (
    <div className={`vm-explore-alerts-rule ${state.replace(" ", "-")}`}>
      <Accordion
        defaultExpanded={expandRules.has(rule.id)}
        onChange={onRulesChange}
        key={`rule-${rule.id}`}
        title={<ItemHeader
          entity="rule"
          type={rule.type}
          groupId={rule.group_id}
          alertCount={rule?.alerts?.length || 0}
          id={rule.id}
          name={rule.name}
        />}
      >
        <BaseRule item={rule} />
      </Accordion>
    </div>
  );
};

export default Rule;
