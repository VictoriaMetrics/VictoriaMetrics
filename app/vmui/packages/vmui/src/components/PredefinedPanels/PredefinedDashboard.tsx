import React, {FC} from "preact/compat";
import {DashboardSettings} from "../../types";
import Box from "@mui/material/Box";
import Accordion from "@mui/material/Accordion";
import AccordionSummary from "@mui/material/AccordionSummary";
import AccordionDetails from "@mui/material/AccordionDetails";
import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import Typography from "@mui/material/Typography";
import PredefinedPanels from "./PredefinedPanels";

export interface PredefinedDashboardProps extends DashboardSettings {
  index: number;
}

const PredefinedDashboard: FC<PredefinedDashboardProps> = ({index, name, panels}) => {

  return <Accordion defaultExpanded={!index}>
    <AccordionSummary sx={{px: 3}} expandIcon={<ExpandMoreIcon />}>
      <Box display="flex" alignItems="center">
        {name && <Typography variant="subtitle1" fontWeight="bold" sx={{mr: 2}}>{name}</Typography>}
        <Typography variant="body2" fontStyle="italic">({panels.length} panels)</Typography>
      </Box>
    </AccordionSummary>
    <AccordionDetails sx={{display: "grid", gridGap: "10px"}}>
      {panels.map((p, i) =>
        <PredefinedPanels key={i} title={p.title} description={p.description} unit={p.unit} expr={p.expr}/>)}
    </AccordionDetails>
  </Accordion>;
};

export default PredefinedDashboard;