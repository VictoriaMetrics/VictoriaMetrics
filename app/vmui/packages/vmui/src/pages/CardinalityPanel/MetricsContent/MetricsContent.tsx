import { FC } from "react";
import TabPanel from "../../../components/Main/TabPanel/TabPanel";
import EnhancedTable from "../../../components/Main/Table/Table";
import TableCells from "../TableCells/TableCells";
import BarChart from "../../../components/Chart/BarChart/BarChart";
import { barOptions } from "../../../components/Chart/BarChart/consts";
import React, { SyntheticEvent } from "react";
import { Data, HeadCell } from "../../../components/Main/Table/types";
import { MutableRef } from "preact/hooks";

interface MetricsProperties {
  rows: Data[];
  activeTab: number;
  onChange: (e: SyntheticEvent, newValue: number) => void;
  onActionClick: (e: SyntheticEvent) => void;
  tabs: string[];
  chartContainer: MutableRef<HTMLDivElement> | undefined;
  totalSeries: number,
  tabId: string;
  sectionTitle: string;
  tableHeaderCells: HeadCell[];
}

const MetricsContent: FC<MetricsProperties> = ({
  rows,
  activeTab,
  onChange,
  tabs,
  chartContainer,
  totalSeries,
  tabId,
  onActionClick,
  sectionTitle,
  tableHeaderCells,
}) => {
  const tableCells = (row: Data) => (
    <TableCells
      row={row}
      totalSeries={totalSeries}
      onActionClick={onActionClick}
    />
  );
  return (
    <>
      <div>
        <div>
          <h5>{sectionTitle}</h5>
          <div>
            {/* TODO add tabs */}
            {/*<div*/}
            {/*  value={activeTab}*/}
            {/*  onChange={onChange}*/}
            {/*  aria-label="basic tabs example"*/}
            {/*>*/}
            {/*  {tabs.map((title: string, i: number) =>*/}
            {/*    <Tab*/}
            {/*      key={title}*/}
            {/*      label={title}*/}
            {/*      aria-controls={`tabpanel-${i}`}*/}
            {/*      id={tabId}*/}
            {/*      iconPosition={"start"}*/}
            {/*      icon={ i === 0 ? <TableIcon /> : <ChartIcon /> }*/}
            {/*    />*/}
            {/*  )}*/}
            {/*</div>*/}
          </div>
          {tabs.map((_,idx) =>
            <div
              ref={chartContainer}
              style={{ width: "100%", paddingRight: idx !== 0 ? "40px" : 0 }}
              key={`chart-${idx}`}
            >
              <TabPanel
                value={activeTab}
                index={idx}
              >
                {activeTab === 0 ? <EnhancedTable
                  rows={rows}
                  headerCells={tableHeaderCells}
                  defaultSortColumn={"value"}
                  tableCells={tableCells}
                />: <BarChart
                  data={[
                    // eslint-disable-next-line @typescript-eslint/ban-ts-comment
                    // @ts-ignore
                    rows.map((v) => v.name),
                    rows.map((v) => v.value),
                    rows.map((_, i) => i % 12 == 0 ? 1 : i % 10 == 0 ? 2 : 0),
                  ]}
                  container={chartContainer?.current || null}
                  configs={barOptions}
                />}
              </TabPanel>
            </div>
          )}
        </div>
      </div>
    </>
  );
};

export default MetricsContent;
