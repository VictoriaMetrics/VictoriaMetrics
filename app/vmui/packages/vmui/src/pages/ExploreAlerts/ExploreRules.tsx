import { FC, useEffect, useMemo, useState, useCallback } from "preact/compat";
import { useNavigate, useLocation, useSearchParams } from "react-router";
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
import ExploreGroup from "../../pages/ExploreAlerts/ExploreGroup";
import { Group, Rule as APIRule } from "../../types";
import { getQueryStringValue } from "../../utils/query-string";
import { getState, getStates, getChanges, getFromStorage } from "./helpers";
import debounce from "lodash.debounce";

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

  const navigate = useNavigate();
  const { hash } = useLocation();

  if (!hash && groupId) {
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

  const handleChangeSearch = useCallback((input: string) => {
    if (!input) {
      setSearchInput("");
    } else {
      setSearchInput(input);
    }
  }, [searchInput]);

  const handleGroupChangeExpand = useCallback((id: string) => {
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
  }, [expandGroups]);

  const getModal = () => {
    if (ruleId !== "") {
      return (
        <ExploreRule
          groupId={groupId}
          id={ruleId}
          mode={ruleId !== "" ? "rule" : "alert"}
          onClose={handleClose(`rule-${ruleId}`)}
        />
      );
    } else if (alertId !== "") {
      return (
        <ExploreAlert
          groupId={groupId}
          id={alertId}
          mode={ruleId !== "" ? "rule" : "alert"}
          onClose={handleClose(`alert-${alertId}`)}
        />
      );
    } else {
      return (
        <ExploreGroup
          id={groupId}
          onClose={handleClose(`group-${groupId}`)}
        />
      );
    }
  };

  const handleRuleChangeExpand = useCallback((id: string) => {
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
  }, [expandRules]);

  const handleChangeStates = useCallback((title: string) => {
    setStates(getChanges(title, states));
  }, [states]);

  const handleChangeTypes = useCallback((title: string) => {
    setTypes(getChanges(title, types));
  }, [types]);

  const noRuleFound = "No rules found!";

  const handleClose = (id: string) => {
    return () => {
      searchParams.delete("group_id");
      searchParams.delete("rule_id");
      searchParams.delete("alert_id");
      setSearchParams(searchParams);
      setModalOpen(false);
      navigate({
        hash: `#${id}`,
      });
    };
  };

  const allTypes: Set<string> = new Set();
  const allStates: Set<string> = new Set();
  const filteredGroups: Group[] = [];

  const {
    groups,
    isLoading: loadingGroups,
    error: errorGroups,
  } = useFetchGroups({ blockFetch: modalOpen });

  const isLoading = useMemo(() => {
    return loadingGroups;
  }, [loadingGroups]);

  const error = useMemo(() => {
    return errorGroups;
  }, [errorGroups]);

  const location = useLocation();

  if (!isLoading && !error && groups?.length) {
    const savedScrollTop = localStorage.getItem("scrollTop");
    useEffect(() => {
      if (hash) {
        const target = document.querySelector(hash);
        if (target) {
          let parent = target.closest("details");
          while (parent) {
            parent.open = true;
            parent = parent!.parentElement!.closest("details");
          }
          target.scrollIntoView();
        }
      } else {
        if (savedScrollTop) {
          window.scrollTo(0, parseInt(savedScrollTop));
        }
        const updateScrollPosition = () => {
          localStorage.setItem("scrollTop", (window.scrollY || 0).toString());
        };
        window.addEventListener("scroll", updateScrollPosition);
        return () => {
          window.removeEventListener("scroll", updateScrollPosition);
        };
      }
    }, [hash, location, savedScrollTop]);
  }

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
        !rule.name.toLowerCase().includes(searchInput.toLowerCase()) &&
        !group.name.toLowerCase().includes(searchInput.toLowerCase()) &&
        !group.file.toLowerCase().includes(searchInput.toLowerCase())
      )
        return;

      filteredRules.push(rule);
      if (state !== "no match" && state !== "unhealthy" && state !== "firing")
        return;

      const count = state === "firing" ? rule?.alerts?.length : 1;
      if (stateName in statesPerGroup) {
        statesPerGroup[stateName] += count;
      } else {
        statesPerGroup[stateName] = count;
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
    <>
      {modalOpen && getModal()}
      {(!modalOpen || !!allStates?.size) && (
        <div className="vm-explore-alerts">
          <RulesHeader
            types={types}
            allTypes={Array.from(allTypes)}
            states={states}
            allStates={Array.from(allStates)}
            onChangeTypes={handleChangeTypes}
            onChangeStates={handleChangeStates}
            onChangeSearch={debounce(handleChangeSearch, 500)}
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
                    <div className="vm-explore-alerts-items">
                      {group.rules.map((rule) => (
                        <Rule
                          key={`rule-${rule.id}`}
                          rule={rule}
                          expandRules={expandRules}
                          onRulesChange={handleRuleChangeExpand(rule.id)}
                          states={getStates(rule)}
                        />
                      ))}
                    </div>
                  </Accordion>
                </div>
              ))}
            </div>
          )}
        </div>
      )}
    </>
  );
};

export default ExploreRules;
