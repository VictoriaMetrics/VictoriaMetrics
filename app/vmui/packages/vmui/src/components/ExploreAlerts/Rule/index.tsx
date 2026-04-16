import { FC } from "preact/compat";
import ItemHeader from "../ItemHeader";
import Accordion from "../../Main/Accordion/Accordion";
import "./style.scss";
import { Rule as APIRule } from "../../../types";
import BaseRule from "../BaseRule";
import Tooltip from "../../Main/Tooltip/Tooltip";
import { ChartIcon } from "../../Main/Icons";
import router from "../../../router";

interface RuleProps {
  states: Record<string, number>;
  rule: APIRule;
  topQueriesSet?: Set<string>;
}

const normalizeQuery = (q: string): string => q.replace(/\s+/g, " ").trim();

const Rule: FC<RuleProps> = ({ states, rule, topQueriesSet }) => {
  const state = Object.keys(states).length > 0 ? Object.keys(states)[0] : "ok";
  const isInTopQueries = topQueriesSet ? topQueriesSet.has(normalizeQuery(rule.query)) : false;

  const topQueriesUrl = `${router.topQueries}?topN=100&maxLifetime=10m&query=${encodeURIComponent(rule.query)}`;

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
          topQueriesUrl={isInTopQueries ? topQueriesUrl : undefined}
        />}
      >
        <BaseRule item={rule} />
      </Accordion>
    </div>
  );
};

export default Rule;
