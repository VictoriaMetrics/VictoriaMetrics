import React, {FC} from "preact/compat";
import {DashboardRow} from "../../types";
import Box from "@mui/material/Box";
import Accordion from "@mui/material/Accordion";
import AccordionSummary from "@mui/material/AccordionSummary";
import AccordionDetails from "@mui/material/AccordionDetails";
import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import Typography from "@mui/material/Typography";
import PredefinedPanels from "./PredefinedPanels";

export interface PredefinedDashboardProps extends DashboardRow {
  index: number;
}

const PredefinedDashboard: FC<PredefinedDashboardProps> = ({index, title, panels}) => {

  return <Accordion defaultExpanded={!index} sx={{boxShadow: "none"}}>
    <AccordionSummary
      sx={{px: 3, bgcolor: "rgba(227, 242, 253, 0.6)"}}
      aria-controls={`panel${index}-content`}
      id={`panel${index}-header`}
      expandIcon={<ExpandMoreIcon />}
    >
      <Box display="flex" alignItems="center" width={"100%"}>
        {title && <Typography variant="h6" fontWeight="bold" sx={{mr: 2}}>{title}</Typography>}
        <Typography variant="body2" fontStyle="italic">({panels.length} panels)</Typography>
      </Box>
    </AccordionSummary>
    <AccordionDetails sx={{display: "grid", gridGap: "10px"}}>
      {panels.map((p, i) =>
        <PredefinedPanels key={i}
          title={p.title}
          description={p.description}
          unit={p.unit}
          expr={p.expr}
          showLegend={p.showLegend}/>
      )}
    </AccordionDetails>
  </Accordion>;
};

export default PredefinedDashboard;