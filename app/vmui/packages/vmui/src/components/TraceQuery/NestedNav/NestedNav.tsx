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
import { humanizeSeconds } from "../../../utils/time";

interface RecursiveProps {
  isRoot?: boolean;
  trace: Trace;
  totalMsec: number;
}

interface OpenLevels {
  [x: number]: boolean
}

const NestedNav: FC<RecursiveProps> = ({ isRoot, trace, totalMsec })  => {
  const { isDarkTheme } = useAppState();
  const { isMobile } = useDeviceDetect();
  const [openLevels, setOpenLevels] = useState({} as OpenLevels);
  const messageRef = useRef<HTMLDivElement>(null);

  const [isExpanded, setIsExpanded] = useState(false);
  const [showFullMessage, setShowFullMessage] = useState(false);

  const duration = humanizeSeconds(trace.duration / 1000) || `${trace.duration}ms`;

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

  const hasChildren = trace.children && !!trace.children.length;
  const progress = trace.duration / totalMsec * 100;
  const handleListClick = (level: number) => () => {
    if (!hasChildren) return;
    setOpenLevels((prevState:OpenLevels) => {
      return { ...prevState, [level]: !prevState[level] };
    });
  };

  return (
    <div
      className={classNames({
        "vm-nested-nav": true,
        "vm-nested-nav_root": isRoot,
        "vm-nested-nav_dark": isDarkTheme,
        "vm-nested-nav_mobile": isMobile,
      })}
    >
      <div
        className={classNames({
          "vm-nested-nav-header": true,
          "vm-nested-nav-header_open": openLevels[trace.idValue],
        })}
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
            {`duration: ${duration}`}
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
      {openLevels[trace.idValue] && (
        <div className="vm-nested-nav__childrens">
          {hasChildren && trace.children.map((trace) => (
            <NestedNav
              key={trace.duration}
              trace={trace}
              totalMsec={totalMsec}
            />
          ))}
        </div>
      )}
    </div>
  );
};

export default NestedNav;
