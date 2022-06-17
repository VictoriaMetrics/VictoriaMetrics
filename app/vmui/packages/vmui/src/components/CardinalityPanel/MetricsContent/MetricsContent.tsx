import {FC} from "react";
import {Box, Grid, Tab, Tabs, Typography} from "@mui/material";
import TableChartIcon from "@mui/icons-material/TableChart";
import ShowChartIcon from "@mui/icons-material/ShowChart";
import TabPanel from "../../TabPanel/TabPanel";
import EnhancedTable from "../../Table/Table";
import TableCells from "../TableCells/TableCells";
import BarChart from "../../BarChart/BarChart";
import {barOptions} from "../../BarChart/consts";
import React, {SyntheticEvent} from "react";
import {Data, HeadCell} from "../../Table/types";
import {MutableRef} from "preact/hooks";

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
      <Grid container spacing={2} sx={{px: 2}}>
        <Grid item xs={12} md={12} lg={12}>
          <Typography gutterBottom variant="h5" component="h5">{sectionTitle}</Typography>
          <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
            <Tabs
              value={activeTab}
              onChange={onChange} aria-label="basic tabs example">
              {tabs.map((title: string, i: number) =>
                <Tab
                  key={title}
                  label={title}
                  aria-controls={`tabpanel-${i}`}
                  id={tabId}
                  iconPosition={"start"}
                  icon={ i === 0 ? <TableChartIcon /> : <ShowChartIcon /> } />
              )}
            </Tabs>
          </Box>
          {tabs.map((_,idx) =>
            <div
              ref={chartContainer}
              style={{width: "100%", paddingRight: idx !== 0 ? "40px" : 0 }} key={`chart-${idx}`}>
              <TabPanel value={activeTab} index={idx}>
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
        </Grid>
      </Grid>
    </>
  );
};

export default MetricsContent;
