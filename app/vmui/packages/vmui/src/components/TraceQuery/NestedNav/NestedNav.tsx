import React, { FC, useState } from "preact/compat";
import LineProgress from "../../Main/LineProgress/LineProgress";
import Trace from "../Trace";
import { ArrowDownIcon } from "../../Main/Icons";
import "./style.scss";
import classNames from "classnames";
import { useAppState } from "../../../state/common/StateContext";

interface RecursiveProps {
  trace: Trace;
  totalMsec: number;
}

interface OpenLevels {
  [x: number]: boolean
}

const NestedNav: FC<RecursiveProps> = ({ trace, totalMsec })  => {
  const { isDarkTheme } = useAppState();
  const [openLevels, setOpenLevels] = useState({} as OpenLevels);

  const handleListClick = (level: number) => () => {
    setOpenLevels((prevState:OpenLevels) => {
      return { ...prevState, [level]: !prevState[level] };
    });
  };
  const hasChildren = trace.children && !!trace.children.length;
  const progress = trace.duration / totalMsec * 100;

  return (
    <div
      className={classNames({
        "vm-nested-nav": true,
        "vm-nested-nav_dark": isDarkTheme,
      })}
    >
      <div
        className="vm-nested-nav-header"
        onClick={handleListClick(trace.idValue)}
      >
        {hasChildren && (
          <div
            className={classNames({
              "vm-nested-nav-header__icon": true,
              "vm-nested-nav-header__icon_open": openLevels[trace.idValue]
            })}
          >
            <ArrowDownIcon />
          </div>
        )}
        <div className="vm-nested-nav-header__progress">
          <LineProgress value={progress}/>
        </div>
        <div className="vm-nested-nav-header__message">
          {trace.message}
        </div>
        <div className="vm-nested-nav-header__duration">
          {`duration: ${trace.duration} ms`}
        </div>
      </div>
      {openLevels[trace.idValue] && <div>
        {hasChildren && trace.children.map((trace) => (
          <NestedNav
            key={trace.duration}
            trace={trace}
            totalMsec={totalMsec}
          />
        ))}
      </div>}
    </div>
  );
};

export default NestedNav;
