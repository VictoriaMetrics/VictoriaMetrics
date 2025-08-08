import { FC } from "preact/compat";
import ItemHeader from "../ItemHeader";
import Accordion from "../../Main/Accordion/Accordion";
import "./style.scss";
import { Rule as APIRule } from "../../../types";
import BaseRule from "../BaseRule";

interface RuleProps {
  states: Record<string, number>;
  rule: APIRule;
  expandRules: Set<string>;
  onRulesChange: (a: boolean) => void;
}

const Rule: FC<RuleProps> = ({ states, rule, expandRules, onRulesChange }) => {
  const state = Object.keys(states).length > 0 ? Object.keys(states)[0] : "ok";
  return (
    <div className={`vm-explore-alerts-rule vm-badge-item ${state.replace(" ", "-")}`}>
      <Accordion
        defaultExpanded={expandRules.has(rule.id)}
        onChange={onRulesChange}
        key={`rule-${rule.id}`}
        title={<ItemHeader
          entity="rule"
          type={rule.type}
          groupId={rule.group_id}
          states={states}
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
