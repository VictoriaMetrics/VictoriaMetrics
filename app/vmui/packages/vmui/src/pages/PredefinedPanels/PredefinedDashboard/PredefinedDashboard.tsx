import React, { FC, useEffect, useMemo, useState } from "preact/compat";
import { MouseEvent as ReactMouseEvent, useCallback } from "react";
import { DashboardRow } from "../../../types";
import PredefinedPanel from "../PredefinedPanel/PredefinedPanel";
import Accordion from "../../../components/Main/Accordion/Accordion";
import "./style.scss";
import classNames from "classnames";
import Alert from "../../../components/Main/Alert/Alert";
import useWindowSize from "../../../hooks/useWindowSize";
import useEventListener from "../../../hooks/useEventListener";

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

  const windowSize = useWindowSize();
  const sizeSection = useMemo(() => {
    return windowSize.width / 12;
  }, [windowSize]);

  const [expanded, setExpanded] = useState(!index);
  const [panelsWidth, setPanelsWidth] = useState<number[]>([]);

  useEffect(() => {
    setPanelsWidth(panels && panels.map(p => p.width || 12));
  }, [panels]);

  const [resize, setResize] = useState({ start: 0, target: 0, enable: false });

  const handleMouseMove = useCallback((e: MouseEvent) => {
    if (!resize.enable) return;
    const { start } = resize;
    const sectionCount = Math.ceil((start - e.clientX)/sizeSection);
    if (Math.abs(sectionCount) >= 12) return;
    const width = panelsWidth.map((p, i) => p - (i === resize.target ? sectionCount : 0));
    setPanelsWidth(width);
  }, [resize, sizeSection]);

  const handleMouseDown = (e: ReactMouseEvent<HTMLButtonElement, MouseEvent>, i: number) => {
    setResize({
      start: e.clientX,
      target: i,
      enable: true,
    });
  };

  const handleMouseUp = useCallback(() => {
    setResize({
      ...resize,
      enable: false
    });
  }, [resize]);

  const handleChangeExpanded = (val: boolean) => setExpanded(val);

  const createHandlerResize = (index: number) => (e: ReactMouseEvent<HTMLButtonElement>) => {
    handleMouseDown(e, index);
  };

  useEventListener("mousemove", handleMouseMove);
  useEventListener("mouseup", handleMouseUp);

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
              className="vm-predefined-dashboard-panels-panel vm-block vm-block_empty-padding"
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
                onMouseDown={createHandlerResize(i)}
                aria-label="resize the panel"
              />
            </div>
          )
          : <div className="vm-predefined-dashboard-panels-panel__alert">
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
