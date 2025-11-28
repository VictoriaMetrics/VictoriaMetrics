import { FC, useEffect, useMemo, useState, useCallback } from "preact/compat";
import { useSearchParams } from "react-router";
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
import { getQueryStringValue } from "../../utils/query-string";
import { getStates, getChanges, filterGroups } from "./helpers";
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

  const [searchInput, setSearchInput] = useState(defaultSearchInput);
  const [types, setTypes] = useState(defaultTypes);
  const [states, setStates] = useState(defaultStates);
  const [modalOpen, setModalOpen] = useState(true);
  const [searchParams, setSearchParams] = useSearchParams();

  useEffect(() => {
    setModalOpen(!!groupId);
  }, [groupId]);

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

  const getModal = () => {
    if (ruleId) {
      return (
        <ExploreRule
          groupId={groupId}
          id={ruleId}
          mode={ruleId ? "rule" : "alert"}
          onClose={handleClose}
        />
      );
    } else if (alertId) {
      return (
        <ExploreAlert
          groupId={groupId}
          id={alertId}
          mode={ruleId ? "rule" : "alert"}
          onClose={handleClose}
        />
      );
    } else if (groupId) {
      return (
        <ExploreGroup
          id={groupId}
          onClose={handleClose}
        />
      );
    }
  };

  const noRuleFound = "No rules found!";

  const handleClose = () => {
    const newParams = new URLSearchParams(searchParams);
    newParams.delete("group_id");
    newParams.delete("rule_id");
    newParams.delete("alert_id");
    setSearchParams(newParams);
    setModalOpen(false);
  };

  const {
    groups,
    isLoading,
    error,
  } = useFetchGroups({ blockFetch: modalOpen });

  const { filteredGroups, allTypes, allStates } = useMemo(
    () => filterGroups(groups || [], types, states, searchInput),
    [groups, types, states, searchInput]
  );

  if (!types.every(v => allTypes.has(v))) {
    setTypes([]);
  }
  const selectedTypes = allTypes.size === types.length ? [] : types;

  if (!states.every(v => allStates.has(v))) {
    setStates([]);
  }
  const selectedStates = allStates.size === states.length ? [] : states;

  const handleChangeStates = useCallback((title: string) => {
    setStates(getChanges(title, selectedStates));
  }, [states]);

  const handleChangeTypes = useCallback((title: string) => {
    setTypes(getChanges(title, selectedTypes));
  }, [types]);

  return (
    <>
      {modalOpen && getModal()}
      {(!modalOpen || !!allStates?.size) && (
        <div className="vm-explore-alerts">
          <RulesHeader
            types={selectedTypes}
            allTypes={Array.from(allTypes)}
            states={selectedStates}
            allStates={Array.from(allStates)}
            search={searchInput}
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
                    key={`group-${group.id}`}
                    id={`group-${group.id}`}
                    title={<GroupHeader group={group} />}
                  >
                    <div className="vm-explore-alerts-items">
                      {group.rules.map((rule) => (
                        <Rule
                          key={`rule-${rule.id}`}
                          rule={rule}
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
