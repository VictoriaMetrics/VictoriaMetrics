import React, { FC, useCallback, useEffect, useMemo, useState } from "preact/compat";
import { useLocation } from "react-router";
import { useNotifiersSetQueryParams as useSetQueryParams } from "./hooks/useSetQueryParams";
import Spinner from "../../components/Main/Spinner/Spinner";
import Alert from "../../components/Main/Alert/Alert";
import Accordion from "../../components/Main/Accordion/Accordion";
import { useFetchNotifiers } from "./hooks/useFetchNotifiers";
import "./style.scss";
import NotifiersHeader from "../../components/ExploreAlerts/NotifiersHeader";
import NotifierHeader from "../../components/ExploreAlerts/NotifierHeader";
import Notifier from "../../components/ExploreAlerts/Notifier";
import { Notifier as APINotifier, Target as APITarget } from "../../../types";
import { getQueryStringValue } from "../../utils/query-string";

const defaultTypesStr = getQueryStringValue("types", "") as string;
const defaultTypes = defaultTypesStr.split(",").filter(rt => rt) as string[];
const defaultSearchInput = getQueryStringValue("q", "") as string;

const ExploreNotifiers: FC = () => {
  const { notifiers, isLoading: loadingNotifiers, error: errorNotifiers } = useFetchNotifiers();

  const [expandNotifiers, setExpandNotifiers] = useState<Set<string>>([]);
  const [searchInput, setSearchInput] = useState(defaultSearchInput);
  const [types, setTypes] = useState(defaultTypes);

  useSetQueryParams({
    types: types.join(","),
  });

  const { hash } = useLocation();

  const isLoading = useMemo(() => {
    return loadingNotifiers;
  }, [loadingNotifiers]);

  const error = useMemo(() => {
    return errorNotifiers;
  }, [errorNotifiers]);

  useEffect(() => {
    if (!isLoading && notifiers && hash && !error) {
      const target = document.querySelector(hash);
      if (target) {
        let parent = target.closest("details");
        while(parent) {
          parent.open = true;
          parent = parent.parentElement.closest("details");
        }
        target.scrollIntoView();
      }
    }
  }, [hash]);

  const handleChangeSearch = (input: string) => {
    if (!input) {
      setSearchInput("");
    } else {
      setSearchInput(input);
    }
  };

  const handleNotifiersChangeExpand = useCallback((id: string) => (value: boolean) => {
    setExpandNotifiers((prev) => {
      const newExpandNotifiers = new Set(prev);
      if (value) {
        newExpandNotifiers.add(id); 
      } else {
        newExpandNotifiers.delete(id);
      }
      return newExpandNotifiers;
    });
  }, []);

  const allTypes: Set<string> = new Set();
  const filteredNotifiers: APINotifier[] = [];

  const searchRegex = new RegExp(searchInput, "i");

  notifiers.forEach(notifier => {
    const filteredTargets: APITarget[] = [];
    const targets = notifier.targets || [];
    targets.forEach((target) => {
      allTypes.add(notifier.kind);
      if (!types?.length || types.includes(notifier.kind)) {
        if (!searchInput || searchRegex.test(target.address) || searchRegex.test(notifier.kind)) {
          filteredTargets.push(target);
        }
      }
    });
    if (filteredTargets.length) {
      const n = Object.assign({}, notifier);
      n.targets = filteredTargets;
      filteredNotifiers.push(n);
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


  const handleChangeTypes = (title: string) => {
    setTypes(getChanges(title, types));
  };

  return (
    <div className="vm-explore-alerts">
      <NotifiersHeader
        types={types}
        allTypes={Array.from(allTypes)}
        onChangeTypes={handleChangeTypes}
        onChangeSearch={handleChangeSearch}
      />

      {isLoading && <Spinner />}
      {error && <Alert variant="error">{error}</Alert>}
      {!filteredNotifiers.length && <Alert variant="info">No notifiers found!</Alert>}
      <div className="vm-explore-alerts-body">
        {filteredNotifiers.map(notifier => (
          <div
            key={notifier.kind}
            className="vm-explore-alert-group vm-block vm-block_empty-padding"
            id={`notifier-${notifier.kind}`}
          >
            <Accordion
              defaultExpanded={expandNotifiers[notifier.kind]}
              key={`notifier-${notifier.kind}`}
              onChange={handleNotifiersChangeExpand(notifier.kind)}
              title={(
                <NotifierHeader
                  notifier={notifier}
                />
              )}
            >
              <Notifier targets={notifier.targets} />
            </Accordion>
          </div>
        ))}
      </div>
    </div>
  );
};

export default ExploreNotifiers;
