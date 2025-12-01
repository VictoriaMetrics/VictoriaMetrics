import { FC, useState } from "preact/compat";
import { useNotifiersSetQueryParams as useSetQueryParams } from "./hooks/useSetQueryParams";
import Spinner from "../../components/Main/Spinner/Spinner";
import Alert from "../../components/Main/Alert/Alert";
import Accordion from "../../components/Main/Accordion/Accordion";
import { useFetchNotifiers } from "./hooks/useFetchNotifiers";
import "./style.scss";
import NotifiersHeader from "../../components/ExploreAlerts/NotifiersHeader";
import NotifierHeader from "../../components/ExploreAlerts/NotifierHeader";
import Target from "../../components/ExploreAlerts/Target";
import { Notifier as APINotifier, Target as APITarget } from "../../types";
import { getQueryStringValue } from "../../utils/query-string";
import { getChanges } from "./helpers";
import debounce from "lodash.debounce";

const defaultKindsStr = getQueryStringValue("kinds", "") as string;
const defaultKinds = defaultKindsStr.split("&").filter((rt) => rt) as string[];
const defaultSearchInput = getQueryStringValue("search", "") as string;

const ExploreNotifiers: FC = () => {
  const {
    notifiers,
    isLoading,
    error,
  } = useFetchNotifiers();

  const [searchInput, setSearchInput] = useState(defaultSearchInput);
  const [kinds, setKinds] = useState(defaultKinds);

  useSetQueryParams({
    kinds: kinds.join("&"),
    search: searchInput,
  });

  const handleChangeSearch = (input: string) => {
    if (!input) {
      setSearchInput("");
    } else {
      setSearchInput(input);
    }
  };

  const allKinds: Set<string> = new Set();
  const filteredNotifiers: APINotifier[] = [];

  notifiers.forEach((notifier) => {
    const filteredTargets: APITarget[] = [];
    const targets = notifier.targets || [];
    targets.forEach((target) => {
      allKinds.add(notifier.kind);
      if (kinds?.length && !kinds.includes(notifier.kind)) return;
      if (
        searchInput &&
        !target.address.toLowerCase().includes(searchInput.toLowerCase()) &&
        !notifier.kind.toLowerCase().includes(searchInput.toLowerCase())
      )
        return;
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
        search={searchInput}
        onChangeKinds={handleChangeKinds}
        onChangeSearch={debounce(handleChangeSearch, 500)}
      />
      {(isLoading && <Spinner />) || (error && <Alert variant="error">{error}</Alert>) || (
        !filteredNotifiers.length && <Alert variant="info">No notifiers found!</Alert>
      ) || (
        <div className="vm-explore-alerts-body">
          {filteredNotifiers.map((notifier) => (
            <div
              key={notifier.kind}
              className="vm-explore-alert-group vm-block vm-block_empty-padding"
            >
              <Accordion
                key={`notifier-${notifier.kind}`}
                id={`notifier-${notifier.kind}`}
                title={<NotifierHeader notifier={notifier} />}
              >
                <div className="vm-explore-alerts-items">
                  {notifier.targets.map((target) => (
                    <Target
                      key={`target-${target.address}`}
                      target={target}
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
};

export default ExploreNotifiers;
