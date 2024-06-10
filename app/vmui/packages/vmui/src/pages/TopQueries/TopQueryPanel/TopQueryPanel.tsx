import React, { FC, useState } from "react";
import { TopQuery } from "../../../types";
import JsonView from "../../../components/Views/JsonView/JsonView";
import { CodeIcon, TableIcon } from "../../../components/Main/Icons";
import Tabs from "../../../components/Main/Tabs/Tabs";
import TopQueryTable from "../TopQueryTable/TopQueryTable";
import "./style.scss";
import classNames from "classnames";
import useDeviceDetect from "../../../hooks/useDeviceDetect";

export interface TopQueryPanelProps {
  rows: TopQuery[],
  title?: string,
  columns: {title?: string, key: (keyof TopQuery), sortBy?: (keyof TopQuery)}[],
  defaultOrderBy?: keyof TopQuery,
}
const tabs = ["table", "JSON"].map((t, i) => ({
  value: String(i),
  label: t,
  icon: i === 0 ? <TableIcon /> : <CodeIcon />
}));

const TopQueryPanel: FC<TopQueryPanelProps> = ({ rows, title, columns, defaultOrderBy }) => {
  const { isMobile } = useDeviceDetect();
  const [activeTab, setActiveTab] = useState(0);

  const handleChangeTab = (val: string) => {
    setActiveTab(+val);
  };

  return (
    <div
      className={classNames({
        "vm-top-queries-panel": true,
        "vm-block": true,
        "vm-block_mobile": isMobile,
      })}
    >
      <div
        className={classNames({
          "vm-top-queries-panel-header": true,
          "vm-section-header": true,
          "vm-top-queries-panel-header_mobile": isMobile,
        })}
      >
        <h5
          className={classNames({
            "vm-section-header__title": true,
            "vm-section-header__title_mobile": isMobile,
          })}
        >{title}</h5>
        <div className="vm-section-header__tabs">
          <Tabs
            activeItem={String(activeTab)}
            items={tabs}
            onChange={handleChangeTab}
          />
        </div>
      </div>

      <div
        className={classNames({
          "vm-top-queries-panel__table": true,
          "vm-top-queries-panel__table_mobile": isMobile,
        })}
      >
        {activeTab === 0 && (
          <TopQueryTable
            rows={rows}
            columns={columns}
            defaultOrderBy={defaultOrderBy}
          />
        )}
        {activeTab === 1 && <JsonView data={rows} />}
      </div>
    </div>
  );
};

export default TopQueryPanel;
