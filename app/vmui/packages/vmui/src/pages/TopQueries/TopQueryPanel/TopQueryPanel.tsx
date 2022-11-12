import React, { FC, useState } from "react";
import { TopQuery } from "../../../types";
// import TopQueryTable from "../TopQueryTable/TopQueryTable";
import JsonView from "../../../components/Views/JsonView/JsonView";
import { ChartIcon, CodeIcon, TableIcon } from "../../../components/Main/Icons";
import Accordion from "../../../components/Main/Accordion/Accordion";
import Tabs from "../../../components/Main/Tabs/Tabs";
import "./style.scss";

export interface TopQueryPanelProps {
  rows: TopQuery[],
  title?: string,
  columns: {title?: string, key: (keyof TopQuery)}[],
  defaultOrderBy?: keyof TopQuery,
}
const tabs = ["table", "JSON"].map((t, i) => ({
  value: String(i),
  label: t,
  icon: i === 0 ? <TableIcon /> : <CodeIcon />
}));

const TopQueryPanel: FC<TopQueryPanelProps> = ({ rows, title, columns, defaultOrderBy }) => {

  const [activeTab, setActiveTab] = useState(0);

  const handleChangeTab = (val: string) => {
    setActiveTab(+val);
  };

  return (
    <div className="vm-top-queries-panel vm-block">

      <div className="vm-top-queries-panel-header vm-table-header">
        <h5 className="vvm-table-header__title">{title}</h5>
        <div className="vm-table-header__tabs">
          <Tabs
            activeItem={String(activeTab)}
            items={tabs}
            onChange={handleChangeTab}
          />
        </div>
      </div>

      <div>
        {/*{activeTab === 0 && <TopQueryTable*/}
        {/*  rows={rows}*/}
        {/*  columns={columns}*/}
        {/*  defaultOrderBy={defaultOrderBy}*/}
        {/*/>}*/}
        {activeTab === 0 && <div>table</div>}
        {activeTab === 1 && <div><JsonView data={rows} /></div>}
      </div>
    </div>
  );
};

export default TopQueryPanel;
