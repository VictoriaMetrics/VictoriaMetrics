import React, { FC, useState } from "react";
import { TopQuery } from "../../../types";
// import TopQueryTable from "../TopQueryTable/TopQueryTable";
import JsonView from "../../../components/Views/JsonView/JsonView";
import { CodeIcon, TableIcon } from "../../../components/Main/Icons";
import Accordion from "../../../components/Main/Accordion/Accordion";

export interface TopQueryPanelProps {
  rows: TopQuery[],
  title?: string,
  columns: {title?: string, key: (keyof TopQuery)}[],
  defaultOrderBy?: keyof TopQuery,
}
const tabs = ["table", "JSON"];

const TopQueryPanel: FC<TopQueryPanelProps> = ({ rows, title, columns, defaultOrderBy }) => {

  const [activeTab, setActiveTab] = useState(0);

  const onChangeTab = (e: React.SyntheticEvent, val: number) => {
    setActiveTab(val);
  };

  return (
    <Accordion
      defaultExpanded={true}
      title={<h3>{title}</h3>}
    >
      <div>
        <div>
          {/*TODO add tabs*/}
          <div>
            {tabs.map((title: string, i: number) =>
              <div
                key={title}
                id={`${title}_${i}`}
              >
                { i === 0 ? <TableIcon /> : <CodeIcon />}
                {title}
              </div>
            )}
          </div>
        </div>
        {/*{activeTab === 0 && <TopQueryTable*/}
        {/*  rows={rows}*/}
        {/*  columns={columns}*/}
        {/*  defaultOrderBy={defaultOrderBy}*/}
        {/*/>}*/}
        {activeTab === 1 && <div><JsonView data={rows} /></div>}
      </div>
    </Accordion>
  );
};

export default TopQueryPanel;
