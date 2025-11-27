import { FC, useEffect, useMemo, useState, useCallback } from "preact/compat";
import Button from "../../components/Main/Button/Button";
import { ArrowDownIcon } from "../../components/Main/Icons";
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
import { getChanges } from "./helpers";
import debounce from "lodash.debounce";
import { getStates } from "../../components/ExploreAlerts/helpers";

const defaultRuleType = getQueryStringValue("type", "") as string;
const defaultStatesStr = getQueryStringValue("states", "") as string;
const defaultStates = defaultStatesStr.split("&").filter((s) => s) as string[];
const defaultSearchInput = getQueryStringValue("search", "") as string;
const TYPE_STATES: Record<string, string[]> = {
  alert:  ["inactive", "firing", "nomatch", "pending", "unhealthy"],
  record: ["unhealthy", "nomatch", "ok"],
};

const ExploreRules: FC = () => {
  const startToken = getQueryStringValue("start_token", "") as string;
  const groupId = getQueryStringValue("group_id", "") as string;
  const ruleId = getQueryStringValue("rule_id", "") as string;
  const alertId = getQueryStringValue("alert_id", "") as string;

  const [searchInput, setSearchInput] = useState(defaultSearchInput);
  const [ruleType, setRuleType] = useState(defaultRuleType);
  const [states, setStates] = useState(defaultStates);
  const [modalOpen, setModalOpen] = useState(false);
  const [searchParams, setSearchParams] = useSearchParams();

  useEffect(() => {
    setModalOpen(!!groupId);
  }, [groupId]);

  useSetQueryParams({
    rule_type: ruleType,
    states: states.join("&"),
    search: searchInput,
    group_id: groupId,
    alert_id: alertId,
    rule_id: ruleId,
  });

  const handleChangeSearch = useCallback((input: string) => {
    setSearchInput(input || "");
  }, [searchInput]);

  const getModal = () => {
    if (ruleId) {
      return (
        <ExploreRule
          groupId={groupId}
          id={ruleId}
          mode={ruleId ? "rule" : "alert"}
          onClose={handleClose(groupId)}
        />
      );
    } else if (alertId) {
      return (
        <ExploreAlert
          groupId={groupId}
          id={alertId}
          mode={ruleId ? "rule" : "alert"}
          onClose={handleClose(groupId)}
        />
      );
    } else if (groupId) {
      return (
        <ExploreGroup
          id={groupId}
          onClose={handleClose(groupId)}
        />
      );
    }
  };

  const noRuleFound = "No rules found!";

  const handleClose = (groupId: string) => {
    return () => {
      const newParams = new URLSearchParams(searchParams);
      newParams.delete("group_id");
      newParams.delete("rule_id");
      newParams.delete("alert_id");
      newParams.set("start_token", groupId);
      setSearchParams(newParams);
      setModalOpen(false);
    };
  };

  const resetStartToken = () => {
    const newParams = new URLSearchParams(searchParams);
    newParams.delete("start_token");
    setSearchParams(newParams);
  };

  const {
    groups,
    isLoading,
    error,
    hasMoreNext,
    hasMoreBefore,
    loadGroups,
  } = useFetchGroups({ blockFetch: modalOpen, startToken, search: searchInput, ruleType, states });

  const allRuleTypes = Object.keys(TYPE_STATES);
  const allStates = useMemo(
    () => Array.from(ruleType === "" ? new Set(Object.values(TYPE_STATES).flat()) : TYPE_STATES[ruleType]),
    [ruleType]
  );
  const selectedRuleTypes = [ruleType].filter(Boolean);
  useEffect(() => {
    if (!states.every(v => allStates.includes(v))) {
      setStates([]);
    }
  }, [states, allStates]);
  const selectedStates = allStates.length === states.length ? [] : states;

  const handleChangeStates = useCallback((title: string) => {
    setStates(getChanges(title, selectedStates));
  }, [states]);

  const handleChangeRuleType = useCallback((title: string) => {
    const changes = getChanges(title, selectedRuleTypes);
    setRuleType(changes.length && changes.length !== allRuleTypes.length ? changes[0] : "");
  }, [ruleType]);

  return (
    <>
      {modalOpen && getModal()}
      {(!modalOpen || !!allStates?.length) && (
        <div className="vm-explore-alerts">
          <RulesHeader
            types={selectedRuleTypes}
            allRuleTypes={allRuleTypes}
            states={selectedStates}
            allStates={allStates}
            search={searchInput}
            onChangeRuleType={handleChangeRuleType}
            onChangeStates={handleChangeStates}
            onChangeSearch={debounce(handleChangeSearch, 500)}
          />
          {hasMoreBefore && !!groups.length && !error && (
            <div
              className="vm-explore-alerts-load vm-explore-alerts-load-before"
            >
              {isLoading ? <Spinner /> : (
                <Button
                  size="small"
                  color="gray"
                  variant="outlined"
                  startIcon={<ArrowDownIcon />}
                  onClick={resetStartToken}
                >Scroll to top</Button>
              )}
            </div>
          )}
          <div className="vm-explore-alerts-body">
            {(
              error && <Alert variant="error">{error}</Alert>
            ) || (
              !groups.length && <Alert variant="info">{noRuleFound}</Alert>
            ) || (
              groups.map((group) => (
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
              ))
            )}
            {hasMoreNext && !error && (
              <div
                className="vm-explore-alerts-load vm-explore-alerts-load-after"
              > 
                {isLoading ? <Spinner /> : (
                  <Button
                    size="small"
                    color="gray" 
                    variant="outlined"
                    startIcon={<ArrowDownIcon />}
                    onClick={() => loadGroups()}
                  >Load next</Button>
                )} 
              </div>
            )}
          </div>
        </div>
      )}
    </>
  );
};

export default ExploreRules;
