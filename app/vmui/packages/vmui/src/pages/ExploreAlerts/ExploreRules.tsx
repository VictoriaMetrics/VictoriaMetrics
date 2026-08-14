import { FC, useEffect, useMemo, useState, useCallback } from "preact/compat";
import { useSearchParams } from "react-router";
import { useRulesSetQueryParams as useSetQueryParams } from "./hooks/useSetQueryParams";
import Spinner from "../../components/Main/Spinner/Spinner";
import Alert from "../../components/Main/Alert/Alert";
import Accordion from "../../components/Main/Accordion/Accordion";
import { useFetchGroups } from "./hooks/useFetchGroups";
import "./style.scss";
import RulesHeader from "../../components/ExploreAlerts/RulesHeader";
import Pagination from "../../components/ExploreAlerts/Pagination";
import GroupHeader from "../../components/ExploreAlerts/GroupHeader";
import Rule from "../../components/ExploreAlerts/Rule";
import ExploreRule from "../../pages/ExploreAlerts/ExploreRule";
import ExploreAlert from "../../pages/ExploreAlerts/ExploreAlert";
import ExploreGroup from "../../pages/ExploreAlerts/ExploreGroup";
import { getQueryStringValue } from "../../utils/query-string";
import { getChanges } from "./helpers";
import debounce from "lodash.debounce";
import { getStates, getVMAlertSource, GROUP_SOURCE_PARAM } from "../../components/ExploreAlerts/helpers";
import { useAppState } from "../../state/common/StateContext";

const defaultRuleType = getQueryStringValue("type", "") as string;
const defaultStatesStr = getQueryStringValue("states", "") as string;
const defaultStates = defaultStatesStr.split("&").filter((s) => s) as string[];
const defaultSearchInput = getQueryStringValue("search", "") as string;
const defaultSource = getQueryStringValue("vmalert_source", "") as string;
const TYPE_STATES: Record<string, string[]> = {
  alert:  ["inactive", "firing", "nomatch", "pending", "unhealthy"],
  record: ["unhealthy", "nomatch", "ok"],
};

const ExploreRules: FC = () => {
  const pageNum = getQueryStringValue("page_num", "1") as string;
  const groupId = getQueryStringValue("group_id", "") as string;
  const ruleId = getQueryStringValue("rule_id", "") as string;
  const alertId = getQueryStringValue("alert_id", "") as string;
  // groupSource is set by links to a group, rule or alert. It routes the detail request
  // to the owning vmalert without touching the source filter. See GROUP_SOURCE_PARAM.
  const groupSource = getQueryStringValue(GROUP_SOURCE_PARAM, "") as string;

  const [searchInput, setSearchInput] = useState(defaultSearchInput);
  const [ruleType, setRuleType] = useState(defaultRuleType);
  const [states, setStates] = useState(defaultStates);
  const [source, setSource] = useState(defaultSource);
  const [modalOpen, setModalOpen] = useState(false);
  const [searchParams, setSearchParams] = useSearchParams();
  const { appConfig } = useAppState();

  useEffect(() => {
    setModalOpen(!!groupId);
  }, [groupId]);

  useSetQueryParams({
    type: ruleType,
    states: states.join("&"),
    search: searchInput,
    vmalert_source: source,
    group_id: groupId,
    alert_id: alertId,
    rule_id: ruleId,
  });

  const handleChangeSearch = useCallback((input: string) => {
    const newParams = new URLSearchParams(searchParams);
    newParams.set("page_num", "1");
    setSearchParams(newParams);
    setSearchInput(input || "");
  }, [searchInput, searchParams]);

  const getModal = () => {
    if (ruleId) {
      return (
        <ExploreRule
          groupId={groupId}
          id={ruleId}
          mode={ruleId ? "rule" : "alert"}
          source={selectedGroupSource}
          onClose={handleClose}
        />
      );
    } else if (alertId) {
      return (
        <ExploreAlert
          groupId={groupId}
          id={alertId}
          mode={ruleId ? "rule" : "alert"}
          source={selectedGroupSource}
          onClose={handleClose}
        />
      );
    } else if (groupId) {
      return (
        <ExploreGroup
          id={groupId}
          source={selectedGroupSource}
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
    // The routing hint belongs to the closed modal, not to the rules list.
    newParams.delete(GROUP_SOURCE_PARAM);
    setSearchParams(newParams);
    setModalOpen(false);
  };

  const onPageChange = (num: number) => {
    return () => {
      const newParams = new URLSearchParams(searchParams);
      newParams.set("page_num", num.toString());
      setSearchParams(newParams);
    };
  };

  const allRuleTypes = Object.keys(TYPE_STATES);
  const allStates = useMemo(
    () => Array.from(ruleType === "" ? new Set(Object.values(TYPE_STATES).flat()) : TYPE_STATES[ruleType] || []),
    [ruleType]
  );
  const selectedRuleTypes = [ruleType].filter(Boolean);
  // allSources is set by vmselect only when more than a single -vmalert.proxyURL is configured.
  const allSources = useMemo(() => appConfig?.vmalert?.sources || [], [appConfig]);
  const selectedSources = [source].filter(Boolean);
  useEffect(() => {
    if (!states.every(v => allStates.includes(v))) {
      setStates([]);
    }
  }, [states, allStates]);
  useEffect(() => {
    if (source && allSources.length && !allSources.includes(source)) {
      setSource("");
    }
  }, [source, allSources]);

  const pageNumInt: number = Math.max(1, parseInt(pageNum, 10) || 1);
  const {
    groups,
    isLoading,
    error,
    warnings,
    pageInfo,
  } = useFetchGroups({ blockFetch: modalOpen, search: searchInput, ruleType, states, source, pageNum: pageNumInt, onPageChange });

  // Group, rule and alert details are owned by the vmalert, which returned the group,
  // so the detail requests must be routed to it instead of being fanned out to all
  // the vmalerts at -vmalert.proxyURL.
  //
  // The loaded group is the most reliable source. groupSource covers direct links, where
  // the groups aren't loaded yet, and the source filter covers links created before
  // GROUP_SOURCE_PARAM was introduced.
  const selectedGroupSource = useMemo(
    () => getVMAlertSource(groups.find((g) => g.id === groupId)) || groupSource || source,
    [groups, groupId, groupSource, source],
  );

  const handleChangeStates = useCallback((title: string) => {
    const newParams = new URLSearchParams(searchParams);
    newParams.set("page_num", "1");
    setSearchParams(newParams);
    const changes = getChanges(title, states);
    setStates(changes.length === allStates.length ? [] : changes);
  }, [states, searchParams]);

  const handleChangeRuleType = useCallback((title: string) => {
    const newParams = new URLSearchParams(searchParams);
    newParams.set("page_num", "1");
    setSearchParams(newParams);
    // Only a single rule type can be selected - see the comment at handleChangeSource.
    setRuleType(title === "All" || title === ruleType ? "" : title);
  }, [ruleType, searchParams]);

  const handleChangeSource = useCallback((title: string) => {
    const newParams = new URLSearchParams(searchParams);
    newParams.set("page_num", "1");
    setSearchParams(newParams);
    // Only a single source can be selected, so the clicked one replaces the previous
    // selection. Clicking "All" or the already selected source clears the filter.
    // getChanges() isn't used here, since it toggles a multi-select set: clicking another
    // source would either select both (and reset to "All" when all of them are selected)
    // or keep the previous selection, making it impossible to switch between sources.
    setSource(title === "All" || title === source ? "" : title);
  }, [source, searchParams]);

  return (
    <>
      {modalOpen && getModal()}
      {(!modalOpen || !!allStates?.length) && (
        <div className="vm-explore-alerts">
          <RulesHeader
            types={selectedRuleTypes}
            allRuleTypes={allRuleTypes}
            states={states}
            allStates={allStates}
            sources={selectedSources}
            allSources={allSources}
            search={searchInput}
            onChangeRuleType={handleChangeRuleType}
            onChangeStates={handleChangeStates}
            onChangeSource={handleChangeSource}
            onChangeSearch={debounce(handleChangeSearch, 500)}
          />
          {warnings.map((warning) => (
            <Alert
              key={warning}
              variant="warning"
            >{warning}</Alert>
          ))}
          <Pagination
            page={pageInfo.page}
            totalPages={pageInfo.total_pages}
            pageRules={groups.reduce((total, g) => total + g?.rules.length, 0)}
            pageGroups={groups.length}
            totalRules={pageInfo.total_rules}
            totalGroups={pageInfo.total_groups}
            onPageChange={onPageChange}
          />
          {(isLoading && <Spinner />) || (error && <Alert variant="error">{error}</Alert>) || (
            !groups.length && <Alert variant="info">{noRuleFound}</Alert>
          ) || (
            <div className="vm-explore-alerts-body">
              {groups.map((group) => (
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
                          group={group}
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
