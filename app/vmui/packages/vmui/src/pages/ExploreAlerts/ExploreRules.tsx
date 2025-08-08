import { FC, useEffect, useMemo, useState } from "preact/compat";
import { useLocation, useSearchParams } from "react-router";
import { useRulesSetQueryParams as useSetQueryParams } from "./hooks/useSetQueryParams";
import Spinner from "../../components/Main/Spinner/Spinner";
import Alert from "../../components/Main/Alert/Alert";
import Accordion from "../../components/Main/Accordion/Accordion";
import { useFetchGroups } from "./hooks/useFetchGroups";
import "./style.scss";
import RulesHeader from "../../components/ExploreAlerts/RulesHeader";
import GroupHeader from "../../components/ExploreAlerts/GroupHeader";
import Rule from "../../components/ExploreAlerts/Rule";
import ExploreRule from "../../pages/ExploreAlerts/ExploreRule";
import ExploreAlert from "../../pages/ExploreAlerts/ExploreAlert";
import { Group, Rule as APIRule } from "../../types";
import { getQueryStringValue } from "../../utils/query-string";
import { getState, getChanges, getFromStorage } from "./helpers";

const defaultTypesStr = getQueryStringValue("types", "") as string;
const defaultTypes = defaultTypesStr.split("&").filter((rt) => rt) as string[];
const defaultStatesStr = getQueryStringValue("states", "") as string;
const defaultStates = defaultStatesStr.split("&").filter((s) => s) as string[];
const defaultSearchInput = getQueryStringValue("search", "") as string;

const ExploreRules: FC = () => {
  const groupId = getQueryStringValue("group_id", "") as string;
  const ruleId = getQueryStringValue("rule_id", "") as string;
  const alertId = getQueryStringValue("alert_id", "") as string;

  const [expandGroups, setExpandGroups] = useState<Set<string>>(getFromStorage("expandGroups"));
  const [expandRules, setExpandRules] = useState<Set<string>>(getFromStorage("expandRules"));
  const [searchInput, setSearchInput] = useState(defaultSearchInput);
  const [types, setTypes] = useState(defaultTypes);
  const [states, setStates] = useState(defaultStates);
  const [modalOpen, setModalOpen] = useState(false);
  const [searchParams, setSearchParams] = useSearchParams();

  const { hash } = useLocation();

  if (!hash && groupId && (ruleId || alertId)) {
    setModalOpen(true);
  } else {
    setModalOpen(false);
  }

  useSetQueryParams({
    types: types.join("&"),
    states: states.join("&"),
    search: searchInput,
    group_id: groupId,
    alert_id: alertId,
    rule_id: ruleId,
  });

  const handleChangeSearch = (input: string) => {
    if (!input) {
      setSearchInput("");
    } else {
      setSearchInput(input);
    }
  };

  const handleGroupChangeExpand = (id: string) => {
    return (value: boolean) => {
      const newExpand = new Set<string>([...Array.from(expandGroups)]);
      if (!value) {
        newExpand.add(id);
      } else {
        newExpand.delete(id);
      }
      window.localStorage.setItem("expandGroups", Array.from(newExpand).join(","));
      setExpandGroups(newExpand);
    };
  };

  const handleRuleChangeExpand = (id: string) => {
    return (value: boolean) => {
      const newExpand = new Set<string>([...Array.from(expandRules)]);
      if (!value) {
        newExpand.add(id);
      } else {
        newExpand.delete(id);
      }
      window.localStorage.setItem("expandRules", Array.from(newExpand).join(","));
      setExpandRules(newExpand);
    };
  };

  const handleChangeStates = (title: string) => {
    setStates(getChanges(title, states));
  };

  const handleChangeTypes = (title: string) => {
    setTypes(getChanges(title, types));
  };

  const noRuleFound = "No rules found!";

  const handleClose = () => {
    searchParams.delete("group_id");
    searchParams.delete("rule_id");
    searchParams.delete("alert_id");
    setSearchParams(searchParams);
    setModalOpen(false);
  };

  if (modalOpen) {
    const id = ruleId !== "" ? ruleId : alertId;
    if (ruleId !== "") {
      return (
        <ExploreRule
          groupId={groupId}
          id={id}
          mode={ruleId !== "" ? "rule" : "alert"}
          onClose={handleClose}
        />
      );
    } else {
      return (
        <ExploreAlert
          groupId={groupId}
          id={id}
          mode={ruleId !== "" ? "rule" : "alert"}
          onClose={handleClose}
        />
      );
    }
  } else {
    const {
      groups,
      isLoading: loadingGroups,
      error: errorGroups,
    } = useFetchGroups();

    const isLoading = useMemo(() => {
      return loadingGroups;
    }, [loadingGroups]);

    const error = useMemo(() => {
      return errorGroups;
    }, [errorGroups]);

    const location = useLocation();

    if (!isLoading && !error && groups?.length) {
      if (hash) {
        useEffect(() => {
          const target = document.querySelector(hash);
          if (target) {
            let parent = target.closest("details");
            while (parent) {
              parent.open = true;
              parent = parent!.parentElement!.closest("details");
            }
            target.scrollIntoView();
          }
        }, [hash, location]);
      }

      useEffect(() => {
        const savedScrollTop = localStorage.getItem("scrollTop");
        if (savedScrollTop) {
          window.scrollTo(0, parseInt(savedScrollTop));
        }
        const handleBeforeUnload = () => {
          localStorage.setItem("scrollTop", (window.scrollY || 0).toString());
        };
        window.addEventListener("beforeunload", handleBeforeUnload);
        return () => {
          window.removeEventListener("beforeunload", handleBeforeUnload);
        };
      }, []);
    }

    const toggleExpand = () => {
      const expand = expandGroups.size == 0;
      const newExpand = expand ? groups.map(group => group.id) : [];
      window.localStorage.setItem("expandGroups", newExpand.join(","));
      setExpandGroups(new Set(newExpand));
    };

    const allTypes: Set<string> = new Set();
    const allStates: Set<string> = new Set();
    const filteredGroups: Group[] = [];

    groups.forEach((group) => {
      const filteredRules: APIRule[] = [];
      const statesPerGroup: Record<string, number> = {};
      group.rules.forEach((rule) => {
        const ruleType = rule.type.charAt(0).toUpperCase() + rule.type.slice(1);
        allTypes.add(ruleType);
        if (types?.length && !types.includes(ruleType)) return;

        const state = getState(rule);
        const stateName = state.charAt(0).toUpperCase() + state.slice(1);
        allStates.add(stateName);
        if (states?.length && !states.includes(stateName)) return;

        if (
          searchInput &&
          !rule.name.includes(searchInput) &&
          !group.name.includes(searchInput)
        )
          return;

        filteredRules.push(rule);
        if (state !== "no match" && state !== "unhealthy" && state !== "firing")
          return;

        if (stateName in statesPerGroup) {
          statesPerGroup[stateName]++;
        } else {
          statesPerGroup[stateName] = 1;
        }
      });
      if (filteredRules.length) {
        const g = Object.assign({}, group);
        g.rules = filteredRules;
        g.states = statesPerGroup;
        filteredGroups.push(g);
      }
    });

    return (
      <div className="vm-explore-alerts">
        <RulesHeader
          types={types}
          allTypes={Array.from(allTypes)}
          states={states}
          allStates={Array.from(allStates)}
          onChangeTypes={handleChangeTypes}
          onChangeStates={handleChangeStates}
          onChangeSearch={handleChangeSearch}
          expanded={expandGroups.size > 0}
          toggleExpand={toggleExpand}
        />
        {(isLoading && <Spinner />) || (error && <Alert variant="error">{error}</Alert>) || (
          !filteredGroups.length && <Alert variant="info">{noRuleFound}</Alert>
        ) || (
          <div className="vm-explore-alerts-body">
            {filteredGroups.map((group) => (
              <div
                key={group.id}
                className="vm-explore-alert-group vm-block vm-block_empty-padding"
              >
                <Accordion
                  defaultExpanded={expandGroups.has(group.id)}
                  key={`group-${group.id}`}
                  id={`group-${group.id}`}
                  onChange={handleGroupChangeExpand(group.id)}
                  title={<GroupHeader group={group} />}
                >
                  <div className="vm-explore-alerts-rules">
                    {group.rules.map((rule) => (
                      <Rule
                        key={`rule-${rule.id}`}
                        rule={rule}
                        expandRules={expandRules}
                        onRulesChange={handleRuleChangeExpand(rule.id)}
                        state={getState(rule)}
                      />
                    ))}
                  </div>
                </Accordion>
              </div>
            ))}
          </div>
        )}
      </div>
    );
  }
};

export default ExploreRules;
