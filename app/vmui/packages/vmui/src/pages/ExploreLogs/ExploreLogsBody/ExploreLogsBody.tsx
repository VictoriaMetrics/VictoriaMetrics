import { FC, useRef } from "preact/compat";
import { CodeIcon, ListIcon, TableIcon } from "../../../components/Main/Icons";
import Tabs from "../../../components/Main/Tabs/Tabs";
import "./style.scss";
import classNames from "classnames";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import { Logs } from "../../../api/types";
import useStateSearchParams from "../../../hooks/useStateSearchParams";
import useSearchParamsFromObject from "../../../hooks/useSearchParamsFromObject";
import LineLoader from "../../../components/Main/LineLoader/LineLoader";
import GroupView from "./views/GroupView/GroupView";
import TableView from "./views/TableView/TableView";
import JsonView from "./views/JsonView/JsonView";

export interface ExploreLogBodyProps {
  data: Logs[];
  isLoading: boolean;
}

enum DisplayType {
  group = "group",
  table = "table",
  json = "json",
}

const tabs = [
  { label: "Group", value: DisplayType.group, icon: <ListIcon />, Component: GroupView },
  { label: "Table", value: DisplayType.table, icon: <TableIcon />, Component: TableView },
  { label: "JSON", value: DisplayType.json, icon: <CodeIcon />, Component: JsonView },
];

const ExploreLogsBody: FC<ExploreLogBodyProps> = ({ data, isLoading }) => {
  const { isMobile } = useDeviceDetect();
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();
  const [activeTab, setActiveTab] = useStateSearchParams(DisplayType.group, "view");
  const settingsRef = useRef<HTMLDivElement>(null);

  const handleChangeTab = (view: string) => {
    setActiveTab(view as DisplayType);
    setSearchParamsFromKeys({ view });
  };

  const ActiveTabComponent = tabs.find(tab => tab.value === activeTab)?.Component;

  return (
    <div
      className={classNames({
        "vm-explore-logs-body": true,
        "vm-block": true,
        "vm-block_mobile": isMobile,
      })}
    >
      {isLoading && <LineLoader />}
      <div
        className={classNames({
          "vm-explore-logs-body-header": true,
          "vm-section-header": true,
          "vm-explore-logs-body-header_mobile": isMobile,
        })}
      >
        <div
          className={classNames({
            "vm-section-header__tabs": true,
            "vm-explore-logs-body-header__tabs_mobile": isMobile,
          })}
        >
          <Tabs
            activeItem={String(activeTab)}
            items={tabs}
            onChange={handleChangeTab}
          />
          <div className="vm-explore-logs-body-header__log-info">
            Total logs returned: <b>{data.length}</b>
          </div>
        </div>
        <div
          className="vm-explore-logs-body-header__settings"
          ref={settingsRef}
        />
      </div>

      <div
        className={classNames({
          "vm-explore-logs-body__table": true,
          "vm-explore-logs-body__table_mobile": isMobile,
        })}
      >
        {ActiveTabComponent &&
            <ActiveTabComponent
              data={data}
              settingsRef={settingsRef}
            />}
      </div>
    </div>
  );
};

export default ExploreLogsBody;
