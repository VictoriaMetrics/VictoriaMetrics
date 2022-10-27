import React, { FC, useState } from "react";
import Box from "@mui/material/Box";
import { TopQuery } from "../../../types";
import Typography from "@mui/material/Typography";
import Accordion from "@mui/material/Accordion";
import AccordionSummary from "@mui/material/AccordionSummary";
import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import AccordionDetails from "@mui/material/AccordionDetails";
import Tabs from "@mui/material/Tabs";
import Tab from "@mui/material/Tab";
import TableChartIcon from "@mui/icons-material/TableChart";
import CodeIcon from "@mui/icons-material/Code";
import TopQueryTable from "../TopQueryTable/TopQueryTable";
import JsonView from "../../../components/Views/JsonView";

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
      sx={{
        mt: 2,
        border: "1px solid",
        borderColor: "primary.light",
        boxShadow: "none",
        "&:before": {
          opacity: 0
        }
      }}
    >
      <AccordionSummary
        sx={{
          p: 2,
          bgcolor: "primary.light",
          minHeight: "64px",
          ".MuiAccordionSummary-content": { display: "flex", alignItems: "center" },
        }}
        expandIcon={<ExpandMoreIcon />}
      >
        <Typography
          variant="h6"
          component="h6"
        >
          {title}
        </Typography>
      </AccordionSummary>
      <AccordionDetails sx={{ p: 0 }}>
        <Box width={"100%"}>
          <Box sx={{ borderBottom: 1, borderColor: "divider" }}>
            <Tabs
              value={activeTab}
              onChange={onChangeTab}
              sx={{ minHeight: "0", marginBottom: "-1px" }}
            >
              {tabs.map((title: string, i: number) =>
                <Tab
                  key={title}
                  label={title}
                  aria-controls={`tabpanel-${i}`}
                  id={`${title}_${i}`}
                  iconPosition={"start"}
                  sx={{ minHeight: "41px" }}
                  icon={ i === 0 ? <TableChartIcon /> : <CodeIcon /> }
                />
              )}
            </Tabs>
          </Box>
          {activeTab === 0 && <TopQueryTable
            rows={rows}
            columns={columns}
            defaultOrderBy={defaultOrderBy}
          />}
          {activeTab === 1 && <Box m={2}><JsonView data={rows} /></Box>}
        </Box>
      </AccordionDetails>
      <Box >

      </Box>
    </Accordion>
  );
};

export default TopQueryPanel;
