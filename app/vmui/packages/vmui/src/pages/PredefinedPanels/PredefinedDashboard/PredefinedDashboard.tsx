import React, { FC, useEffect, useMemo, useState } from "preact/compat";
import { CSSProperties } from "react";
import { MouseEvent as ReactMouseEvent } from "react";
import { DashboardRow } from "../../../types";
import Box from "@mui/material/Box";
import Accordion from "@mui/material/Accordion";
import AccordionSummary from "@mui/material/AccordionSummary";
import AccordionDetails from "@mui/material/AccordionDetails";
import Grid from "@mui/material/Grid";
import ExpandMoreIcon from "@mui/icons-material/ExpandMore";
import Typography from "@mui/material/Typography";
import PredefinedPanel from "../PredefinedPanel/PredefinedPanel";
import Alert from "@mui/material/Alert";
import useResize from "../../../hooks/useResize";

export interface PredefinedDashboardProps extends DashboardRow {
  filename: string;
  index: number;
}

const resizerStyle: CSSProperties = {
  position: "absolute",
  top: 0,
  bottom: 0,
  width: "10px",
  opacity: 0,
  cursor: "ew-resize",
};

const PredefinedDashboard: FC<PredefinedDashboardProps> = ({ index, title, panels, filename }) => {

  const windowSize = useResize(document.body);
  const sizeSection = useMemo(() => {
    return windowSize.width / 12;
  }, [windowSize]);

  const [panelsWidth, setPanelsWidth] = useState<number[]>([]);

  useEffect(() => {
    setPanelsWidth(panels.map(p => p.width || 12));
  }, [panels]);

  const [resize, setResize] = useState({ start: 0, target: 0, enable: false });

  const handleMouseMove = (e: MouseEvent) => {
    if (!resize.enable) return;
    const { start } = resize;
    const sectionCount = Math.ceil((start - e.clientX)/sizeSection);
    if (Math.abs(sectionCount) >= 12) return;
    const width = panelsWidth.map((p, i) => {
      return p - (i === resize.target ? sectionCount : 0);
    });
    setPanelsWidth(width);
  };

  const handleMouseDown = (e: ReactMouseEvent<HTMLButtonElement, MouseEvent>, i: number) => {
    setResize({
      start: e.clientX,
      target: i,
      enable: true,
    });
  };
  const handleMouseUp = () => {
    setResize({
      ...resize,
      enable: false
    });
  };

  useEffect(() => {
    window.addEventListener("mousemove", handleMouseMove);
    window.addEventListener("mouseup", handleMouseUp);
    return () => {
      window.removeEventListener("mousemove", handleMouseMove);
      window.removeEventListener("mouseup", handleMouseUp);
    };
  }, [resize]);

  return <Accordion
    defaultExpanded={!index}
    sx={{ boxShadow: "none" }}
  >
    <AccordionSummary
      sx={{ px: 3, bgcolor: "primary.light" }}
      aria-controls={`panel${index}-content`}
      id={`panel${index}-header`}
      expandIcon={<ExpandMoreIcon />}
    >
      <Box
        display="flex"
        alignItems="center"
        width={"100%"}
      >
        {title && <Typography
          variant="h6"
          fontWeight="bold"
          sx={{ mr: 2 }}
        >{title}</Typography>}
        {panels && <Typography
          variant="body2"
          fontStyle="italic"
        >({panels.length} panels)</Typography>}
      </Box>
    </AccordionSummary>
    <AccordionDetails sx={{ display: "grid", gridGap: "10px" }}>
      <Grid
        container
        spacing={2}
      >
        {Array.isArray(panels) && !!panels.length
          ? panels.map((p, i) =>
            <Grid
              key={i}
              item
              xs={panelsWidth[i]}
              sx={{ transition: "200ms" }}
            >
              <Box
                position={"relative"}
                height={"100%"}
              >
                <PredefinedPanel
                  title={p.title}
                  description={p.description}
                  unit={p.unit}
                  expr={p.expr}
                  alias={p.alias}
                  filename={filename}
                  showLegend={p.showLegend}
                />
                <button
                  style={{ ...resizerStyle, right: 0 }}
                  onMouseDown={(e) => handleMouseDown(e, i)}
                />
              </Box>
            </Grid>)
          : <Alert
            color="error"
            severity="error"
            sx={{ m: 4 }}
          >
            <code>&quot;panels&quot;</code> not found. Check the configuration file <b>{filename}</b>.
          </Alert>
        }
      </Grid>
    </AccordionDetails>
  </Accordion>;
};

export default PredefinedDashboard;
