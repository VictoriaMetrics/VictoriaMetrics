import React, { FC, useEffect, useMemo, useState } from "preact/compat";
import { CSSProperties } from "react";
import { MouseEvent as ReactMouseEvent } from "react";
import { DashboardRow } from "../../../types";
import PredefinedPanel from "../PredefinedPanel/PredefinedPanel";
import useResize from "../../../hooks/useResize";
import Accordion from "../../../components/Main/Accordion/Accordion";

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
    title={(
      <div>
        {title && (
          <span>
            {title}
          </span>
        )}
        {panels && (
          <span>
            ({panels.length} panels)
          </span>
        )}
      </div>
    )}
  >
    <div>
      {Array.isArray(panels) && !!panels.length
        ? panels.map((p, i) =>
          <div key={i}>
            <div>
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
            </div>
          </div>)
        : <>
          {/*<Alert*/}
          {/*  color="error"*/}
          {/*  severity="error"*/}
          {/*  sx={{ m: 4 }}*/}
          {/*>*/}
          {/*  <code>&quot;panels&quot;</code> not found. Check the configuration file <b>{filename}</b>.*/}
          {/*</Alert>*/}
        </>
      }
    </div>
  </Accordion>;
};

export default PredefinedDashboard;
