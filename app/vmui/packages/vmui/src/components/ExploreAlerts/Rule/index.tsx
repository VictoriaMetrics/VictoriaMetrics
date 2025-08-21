import { FC } from "preact/compat";
import ItemHeader from "../ItemHeader";
import Accordion from "../../Main/Accordion/Accordion";
import "./style.scss";
import { Rule as APIRule } from "../../../types";
import BaseRule from "../BaseRule";

interface RuleProps {
  states: Record<string, number>;
  rule: APIRule;
}

const Rule: FC<RuleProps> = ({ states, rule }) => {
  const state = Object.keys(states).length > 0 ? Object.keys(states)[0] : "ok";
  return (
    <div className={`vm-explore-alerts-rule vm-badge-item ${state.replace(" ", "-")}`}>
      <Accordion
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
