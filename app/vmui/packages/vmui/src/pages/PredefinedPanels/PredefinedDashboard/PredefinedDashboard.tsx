import React, { FC, useEffect, useMemo, useState } from "preact/compat";
import { CSSProperties } from "react";
import { MouseEvent as ReactMouseEvent } from "react";
import { DashboardRow } from "../../../types";
import PredefinedPanel from "../PredefinedPanel/PredefinedPanel";
import useResize from "../../../hooks/useResize";
import Accordion from "../../../components/Main/Accordion/Accordion";
import "./style.scss";
import classNames from "classnames";
import Alert from "../../../components/Main/Alert/Alert";

export interface PredefinedDashboardProps extends DashboardRow {
  filename: string;
  index: number;
}

const PredefinedDashboard: FC<PredefinedDashboardProps> = ({
  index,
  title,
  panels,
  filename
}) => {

  const windowSize = useResize(document.body);
  const sizeSection = useMemo(() => {
    return windowSize.width / 12;
  }, [windowSize]);

  const [expanded, setExpanded] = useState(!index);
  const [panelsWidth, setPanelsWidth] = useState<number[]>([]);
  console.log(panelsWidth);

  useEffect(() => {
    setPanelsWidth(panels && panels.map(p => p.width || 12));
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

  const handleChangeExpanded = (val: boolean) => setExpanded(val);

  useEffect(() => {
    window.addEventListener("mousemove", handleMouseMove);
    window.addEventListener("mouseup", handleMouseUp);
    return () => {
      window.removeEventListener("mousemove", handleMouseMove);
      window.removeEventListener("mouseup", handleMouseUp);
    };
  }, [resize]);

  const HeaderAccordion = () => (
    <div
      className={classNames({
        "vm-predefined-dashboard-header": true,
        "vm-predefined-dashboard-header_open": expanded
      })}
    >
      {(title || filename) && <span className="vm-predefined-dashboard-header__title">
        {title || `${index+1}. ${filename}`}
      </span>}
      {panels && <span className="vm-predefined-dashboard-header__count">({panels.length} panels)</span>}
    </div>
  );

  return <div className="vm-predefined-dashboard">
    <Accordion
      defaultExpanded={expanded}
      onChange={handleChangeExpanded}
      title={<HeaderAccordion/>}
    >
      <div className="vm-predefined-dashboard-panels">
        {Array.isArray(panels) && !!panels.length
          ? panels.map((p, i) =>
            <div
              className="vm-predefined-dashboard-panels-panel"
              style={{ gridColumn: `span ${panelsWidth[i]}` }}
              key={i}
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
                className="vm-predefined-dashboard-panels-panel__resizer"
                onMouseDown={(e) => handleMouseDown(e, i)}
              />
            </div>
          )
          : <div style={{ gridColumn: "span 12" }}>
            <Alert variant="error">
              <code>&quot;panels&quot;</code> not found. Check the configuration file <b>{filename}</b>.
            </Alert>
          </div>
        }
      </div>
    </Accordion>
  </div>;
};

export default PredefinedDashboard;
