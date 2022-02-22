import React, {FC} from "preact/compat";
import {DashboardRow} from "../../types";
import Box from "@mui/material/Box";
import Accordion from "@mui/material/Accordion";
import AccordionSummary from "@mui/material/AccordionSummary";
import AccordionDetails from "@mui/material/AccordionDetails";
import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import Typography from "@mui/material/Typography";
import PredefinedPanels from "./PredefinedPanels";
import Alert from "@mui/material/Alert";

export interface PredefinedDashboardProps extends DashboardRow {
  filename: string;
  index: number;
}

const PredefinedDashboard: FC<PredefinedDashboardProps> = ({index, title, panels, filename}) => {

  return <Accordion defaultExpanded={!index} sx={{boxShadow: "none"}}>
    <AccordionSummary
      sx={{px: 3, bgcolor: "rgba(227, 242, 253, 0.6)"}}
      aria-controls={`panel${index}-content`}
      id={`panel${index}-header`}
      expandIcon={<ExpandMoreIcon />}
    >
      <Box display="flex" alignItems="center" width={"100%"}>
        {title && <Typography variant="h6" fontWeight="bold" sx={{mr: 2}}>{title}</Typography>}
        {panels && <Typography variant="body2" fontStyle="italic">({panels.length} panels)</Typography>}
      </Box>
    </AccordionSummary>
    <AccordionDetails sx={{display: "grid", gridGap: "10px"}}>
      {Array.isArray(panels) && !!panels.length
        ? panels.map((p, i) => <PredefinedPanels key={i}
          title={p.title}
          description={p.description}
          unit={p.unit}
          expr={p.expr}
          filename={filename}
          showLegend={p.showLegend}/>)
        : <Alert color="error" severity="error" sx={{m: 4}}>
          <code>&quot;panels&quot;</code> not found. Check the configuration file <b>{filename}</b>.
        </Alert>
      }
    </AccordionDetails>
  </Accordion>;
};

export default PredefinedDashboard;