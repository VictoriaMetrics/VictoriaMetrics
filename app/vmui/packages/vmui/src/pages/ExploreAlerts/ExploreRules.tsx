import React, { FC, useCallback, useEffect, useMemo, useState } from "preact/compat";
import { useLocation } from "react-router";
import { useRulesSetQueryParams as useSetQueryParams } from "./hooks/useSetQueryParams";
import Spinner from "../../components/Main/Spinner/Spinner";
import Alert from "../../components/Main/Alert/Alert";
import Accordion from "../../components/Main/Accordion/Accordion";
import { useFetchGroups } from "./hooks/useFetchGroups";
import "./style.scss";
import RulesHeader from "../../components/ExploreAlerts/RulesHeader";
import GroupHeader from "../../components/ExploreAlerts/GroupHeader";
import Rule from "../../components/ExploreAlerts/Rule";
import { Group, Rule as APIRule } from "../../../types";
import { getQueryStringValue } from "../../utils/query-string";

const defaultRuleTypesStr = getQueryStringValue("rule_types", "") as string;
const defaultRuleTypes = defaultRuleTypesStr.split(",").filter(rt => rt) as string[];
const defaultStatesStr = getQueryStringValue("state", "") as string;
const defaultStates = defaultStatesStr.split(",").filter(s => s) as string[];
const defaultSearchInput = getQueryStringValue("q", "") as string;

const ExploreRules: FC = () => {
  const { groups, isLoading: loadingGroups, error: errorGroups } = useFetchGroups();

  const [expandGroups, setExpandGroups] = useState<Set<string>>([]);
  const [expandRules, setExpandRules] = useState<Set<string>>([]);
  const [searchInput, setSearchInput] = useState(defaultSearchInput);
  const [ruleTypes, setRuleTypes] = useState(defaultRuleTypes);
  const [states, setStates] = useState(defaultStates);

  useSetQueryParams({
    rule_types: ruleTypes.join(","),
    states: states.join(","),
  });

  const { hash } = useLocation();

  const isLoading = useMemo(() => {
    return loadingGroups;
  }, [loadingGroups]);

  const error = useMemo(() => {
    return errorGroups;
  }, [errorGroups]);

  if (!isLoading && groups && hash && !error) {
    useEffect(() => {
      const target = document.querySelector(hash);
      if (target) {
        let parent = target.closest("details");
        while(parent) {
          parent.open = true;
          parent = parent.parentElement.closest("details");
        }
        target.scrollIntoView();
      }
    }, [hash]);
  }

  const handleChangeSearch = (input: string) => {
    if (!input) {
      setSearchInput("");
    } else {
      setSearchInput(input);
    }
  };

  const handleGroupChangeExpand = useCallback((id: string) => (value: boolean) => {
    setExpandGroups((prev) => {
      const newExpandGroups = new Set(prev);
      if (value) {
        newExpandGroups.add(id); 
      } else {
        newExpandGroups.delete(id);
      }
      return newExpandGroups;
    });
  }, []);

  const handleRuleChangeExpand = useCallback((id: string) => (value: boolean) => {
    setExpandRules((prev) => {
      const newExpandRules = new Set(prev);
      if (value) {
        newExpandRules.add(id);
      } else {
        newExpandRules.delete(id);
      }
      return newExpandRules;
    });           
  }, []);

  const allRuleTypes: Set<string> = new Set();
  const allStates: Set<string> = new Set();
  const filteredGroups: Group[] = [];

  const getState = (rule: APIRule) => {
    let state = rule.state || "ok";
    if (!rule.lastSamples && !rule.lastSeriesFetched) {
      state = "no match";
    } else if (rule.health != "ok") {
      state = "unhealthy";
    }
    return state;
  };

  const searchRegex = new RegExp(searchInput, "i");

  groups.forEach((group) => {
    const filteredRules: APIRule[] = [];
    const statesPerGroup: Record<string, number> = {};
    group.rules.forEach((rule) => {
      const ruleType = rule.type.charAt(0).toUpperCase() + rule.type.slice(1);
      allRuleTypes.add(ruleType);
      if (!ruleTypes?.length || ruleTypes.includes(ruleType)) {
        const state = getState(rule);
        const stateName = state.charAt(0).toUpperCase() + state.slice(1);
        allStates.add(stateName);
        if (!states?.length || states.includes(stateName)) {
          filteredRules.push(rule);
          if (!searchInput || searchRegex.test(rule.name) || searchRegex.test(group.name)) {
            if (state === "no match" || state === "unhealthy" || state === "firing") {
              if (stateName in statesPerGroup) {
                statesPerGroup[stateName]++;
              } else {
                statesPerGroup[stateName] = 1;
              }
            }
          }
        }
      }
    });
    if (filteredRules.length) {
      const g = Object.assign({}, group);
      g.rules = filteredRules;
      g.states = statesPerGroup;
      filteredGroups.push(g);
    }
  });

  const getChanges = (title: string, prevValues: string[]): string[] => {
    let newValues = new Set();
    if (title !== "All") {
      newValues = new Set(prevValues);
      if (newValues.has(title)) {
        newValues.delete(title);
      } else {
        newValues.add(title);
      }
    }
    return Array.from(newValues);
  };

  const handleChangeStates = (title: string) => {
    setStates(getChanges(title, states));
  };

  const handleChangeRuleTypes = (title: string) => {
    setRuleTypes(getChanges(title, ruleTypes));
  };

  return (
    <div className="vm-explore-alerts">
      <RulesHeader
        ruleTypes={ruleTypes}
        allRuleTypes={Array.from(allRuleTypes)}
        states={states}
        allStates={Array.from(allStates)}
        onChangeRuleTypes={handleChangeRuleTypes}
        onChangeStates={handleChangeStates}
        onChangeSearch={handleChangeSearch}
      />

      {isLoading && <Spinner />}
      {error && <Alert variant="error">{error}</Alert>}
      {!filteredGroups.length && <Alert variant="info">No rules found!</Alert>}
      <div className="vm-explore-alerts-body">
        {filteredGroups.map(group => (
          <div
            key={group.id}
            className="vm-explore-alert-group vm-block vm-block_empty-padding"
            id={`group-${group.id}`}
          >
            <Accordion
              defaultExpanded={expandGroups[group.id]}
              key={`group-${group.id}`}
              onChange={handleGroupChangeExpand(group.id)}
              title={(
                <GroupHeader
                  group={group}
                />
              )}
            >
              <div className="vm-explore-alerts-rules">
                {group.rules.map(rule => (
                  <Rule
                    key={`rule-${rule.id}`}
                    rule={rule}
                    expandRules={expandRules}
                    onRulesChange={handleRuleChangeExpand}
                    state={getState(rule)}
                  />
                ))}
              </div>
            </Accordion>
          </div>
        ))}
      </div>
    </div>
  );
};

export default ExploreRules;
