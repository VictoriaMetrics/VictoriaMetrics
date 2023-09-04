import React, { FC, useEffect, useRef, useState } from "preact/compat";
import { MouseEvent } from "react";
import LineProgress from "../../Main/LineProgress/LineProgress";
import Trace from "../Trace";
import { ArrowDownIcon } from "../../Main/Icons";
import "./style.scss";
import classNames from "classnames";
import { useAppState } from "../../../state/common/StateContext";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import Button from "../../Main/Button/Button";

interface RecursiveProps {
  trace: Trace;
  totalMsec: number;
}

interface OpenLevels {
  [x: number]: boolean
}

const NestedNav: FC<RecursiveProps> = ({ trace, totalMsec })  => {
  const { isDarkTheme } = useAppState();
  const { isMobile } = useDeviceDetect();
  const [openLevels, setOpenLevels] = useState({} as OpenLevels);
  const messageRef = useRef<HTMLDivElement>(null);

  const [isExpanded, setIsExpanded] = useState(false);
  const [showFullMessage, setShowFullMessage] = useState(false);

  useEffect(() => {
    if (!messageRef.current) return;
    const contentElement = messageRef.current;
    const child = messageRef.current.children[0];
    const { height } = child.getBoundingClientRect();
    setIsExpanded(height > contentElement.clientHeight);
  }, [trace]);

  const handleClickShowMore = (e: MouseEvent<HTMLButtonElement>) => {
    e.stopPropagation();
    setShowFullMessage(prev => !prev);
  };

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
        "vm-nested-nav_mobile": isMobile,
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
        <div
          className={classNames({
            "vm-nested-nav-header__message": true,
            "vm-nested-nav-header__message_show-full": showFullMessage,
          })}
          ref={messageRef}
        >
          <span>{trace.message}</span>
        </div>
        <div className="vm-nested-nav-header-bottom">
          <div className="vm-nested-nav-header-bottom__duration">
            {`duration: ${trace.duration} ms`}
          </div>
          {(isExpanded || showFullMessage) && (
            <Button
              variant="text"
              size="small"
              onClick={handleClickShowMore}
            >
              {showFullMessage ? "Hide" : "Show full query"}
            </Button>
          )}
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
