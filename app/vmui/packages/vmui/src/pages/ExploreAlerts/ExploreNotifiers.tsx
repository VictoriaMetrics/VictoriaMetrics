import { FC, useCallback, useEffect, useMemo, useState } from "preact/compat";
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
import { Notifier as APINotifier, Target as APITarget } from "../../types";
import { getQueryStringValue } from "../../utils/query-string";
import { getChanges } from "./helpers";

const defaultKindsStr = getQueryStringValue("kinds", "") as string;
const defaultKinds = defaultKindsStr.split("&").filter(rt => rt) as string[];
const defaultSearchInput = getQueryStringValue("search", "") as string;

const ExploreNotifiers: FC = () => {
  const { notifiers, isLoading: loadingNotifiers, error: errorNotifiers } = useFetchNotifiers();

  const [expandNotifiers, setExpandNotifiers] = useState<Set<string>>(new Set<string>());
  const [searchInput, setSearchInput] = useState(defaultSearchInput);
  const [kinds, setKinds] = useState(defaultKinds);

  useSetQueryParams({
    kinds: kinds.join("&"),
    search: searchInput,
  });

  const { hash } = useLocation();

  const isLoading = useMemo(() => {
    return loadingNotifiers;
  }, [loadingNotifiers]);

  const error = useMemo(() => {
    return errorNotifiers;
  }, [errorNotifiers]);

  useEffect(() => {
    if (isLoading || !notifiers || !hash || error) return;

    const target = document.querySelector(hash);
    if (!target) return;

    let parent = target.closest("details");
    while(parent) {
      parent.open = true;
      parent = parent!.parentElement!.closest("details");
    }

    target.scrollIntoView();
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

  const allKinds: Set<string> = new Set();
  const filteredNotifiers: APINotifier[] = [];

  const searchRegex = new RegExp(searchInput, "i");

  notifiers.forEach(notifier => {
    const filteredTargets: APITarget[] = [];
    const targets = notifier.targets || [];
    targets.forEach((target) => {
      allKinds.add(notifier.kind);
      if (kinds?.length && !kinds.includes(notifier.kind)) return;
      if (searchInput && !searchRegex.test(target.address) && !searchRegex.test(notifier.kind)) return;
      filteredTargets.push(target);
    });
    if (filteredTargets.length) {
      const n = Object.assign({}, notifier);
      n.targets = filteredTargets;
      filteredNotifiers.push(n);
    }
  });

  const handleChangeKinds = (title: string) => {
    setKinds(getChanges(title, kinds));
  };

  return (
    <div className="vm-explore-alerts">
      <NotifiersHeader
        kinds={kinds}
        allKinds={Array.from(allKinds)}
        onChangeKinds={handleChangeKinds}
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
              defaultExpanded={expandNotifiers.has(notifier.kind)}
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
