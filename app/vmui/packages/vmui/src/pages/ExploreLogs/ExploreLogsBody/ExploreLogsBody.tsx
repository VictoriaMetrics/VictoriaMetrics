import { FC, useRef } from "preact/compat";
import {
  CodeIcon,
  ListIcon,
  TableIcon,
  PlayIcon,
  VisibilityOffIcon,
  VisibilityIcon
} from "../../../components/Main/Icons";
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
import LiveTailingView from "./views/LiveTailingView/LiveTailingView";
import Tooltip from "../../../components/Main/Tooltip/Tooltip";
import Button from "../../../components/Main/Button/Button";
import { useSearchParams } from "react-router-dom";
import Alert from "../../../components/Main/Alert/Alert";

export interface ExploreLogBodyProps {
  data: Logs[];
  isLoading: boolean;
}

enum DisplayType {
  group = "group",
  table = "table",
  json = "json",
  liveTailing = "liveTailing",
}

const tabs = [
  { label: "Group", value: DisplayType.group, icon: <ListIcon/>, Component: GroupView },
  { label: "Table", value: DisplayType.table, icon: <TableIcon/>, Component: TableView },
  { label: "JSON", value: DisplayType.json, icon: <CodeIcon/>, Component: JsonView },
  { label: "Live", value: DisplayType.liveTailing, icon: <PlayIcon/>, Component: LiveTailingView },
];

const ExploreLogsBody: FC<ExploreLogBodyProps> = ({ data, isLoading }) => {
  const { isMobile } = useDeviceDetect();
  const [searchParams, setSearchParams] = useSearchParams();
  const { setSearchParamsFromKeys } = useSearchParamsFromObject();
  const [activeTab, setActiveTab] = useStateSearchParams(DisplayType.group, "view");
  const settingsRef = useRef<HTMLDivElement>(null);

  const [hideLogs, setHideLogs] = useStateSearchParams(false, "hide_logs");

  const toggleHideLogs = () => {
    setHideLogs(prev => {
      const newVal = !prev;
      newVal ? searchParams.set("hide_logs", "true") : searchParams.delete("hide_logs");
      setSearchParams(searchParams);
      return newVal;
    });
  };

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
      {isLoading && <LineLoader/>}
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
          {activeTab !== DisplayType.liveTailing && (
            <div className="vm-explore-logs-body-header__log-info">
              Total logs returned: <b>{data.length}</b>
            </div>
          )}
        </div>
        <div
          className="vm-explore-logs-body-header__settings"
          ref={settingsRef}
        />
        <Tooltip title={hideLogs ? "Show Logs" : "Hide Logs"}>
          <Button
            variant="text"
            color="primary"
            startIcon={hideLogs ? <VisibilityOffIcon/> : <VisibilityIcon/>}
            onClick={toggleHideLogs}
            ariaLabel="settings"
          />
        </Tooltip>
      </div>

      <div
        className={classNames({
          "vm-explore-logs-body__table": true,
          "vm-explore-logs-body__table_hide": hideLogs,
          "vm-explore-logs-body__table_mobile": isMobile,
        })}
      >
        {hideLogs && (
          <Alert variant="info">Logs are hidden. Updates paused.</Alert>
        )}

        {!hideLogs && ActiveTabComponent &&
          <ActiveTabComponent
            data={data}
            settingsRef={settingsRef}
          />
        }
      </div>
    </div>
  );
};

export default ExploreLogsBody;
