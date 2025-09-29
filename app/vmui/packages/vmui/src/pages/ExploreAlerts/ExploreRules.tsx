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

  const navigate = useNavigate();
  const location = useLocation();

  useEffect(() => {
    if (!location.hash && groupId) {
      setModalOpen(true);
    } else {
      setModalOpen(false);
    }
  }, [location.hash, groupId]);

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
    } else if (groupId !== "") {
      return (
        <ExploreGroup
          id={groupId}
          onClose={handleClose(`group-${groupId}`)}
        />
      );
    }
  };

  const handleChangeStates = useCallback((title: string) => {
    setStates(getChanges(title, states));
  }, [states]);

  const handleChangeTypes = useCallback((title: string) => {
    setTypes(getChanges(title, types));
  }, [types]);

  const noRuleFound = "No rules found!";

  const handleClose = (id: string) => {
    return () => {
      const newParams = new URLSearchParams(searchParams);
      newParams.delete("group_id");
      newParams.delete("rule_id");
      newParams.delete("alert_id");
      setSearchParams(newParams);
      setModalOpen(false);
      navigate({
        hash: `#${id}`,
      });
    };
  };

  const {
    groups,
    isLoading,
    error,
  } = useFetchGroups({ blockFetch: modalOpen });

  const pageLoaded = !isLoading && !error && !!groups?.length;
  const savedScrollTop = localStorage.getItem("scrollTop");

  useEffect(() => {
    if (!pageLoaded) return;
    if (location.hash) {
      const target = document.querySelector(location.hash);
      if (target) {
        let parent = target.closest("details");
        while (parent) {
          parent.open = true;
          if (!parent?.parentElement) return;
          parent = parent.parentElement.closest("details");
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
  }, [location, savedScrollTop, pageLoaded]);

  const { filteredGroups, allTypes, allStates } = useMemo(
    () => filterGroups(groups || [], types, states, searchInput),
    [groups, types, states, searchInput]
  );

  const selectedTypes = allTypes.size === types.length ? [] : types;
  const selectedStates = allStates.size === states.length ? [] : states;

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
