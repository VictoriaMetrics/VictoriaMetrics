import React, { FC, useState } from "preact/compat";
import { BorderLinearProgressWithLabel } from "../../Main/BorderLineProgress/BorderLinearProgress";
import Trace from "../Trace";
import { ArrowUpIcon, PlusCircleFillIcon } from "../../Main/Icons";

interface RecursiveProps {
  trace: Trace;
  totalMsec: number;
}

interface OpenLevels {
  [x: number]: boolean
}

const NestedNav: FC<RecursiveProps> = ({ trace, totalMsec })  => {
  const [openLevels, setOpenLevels] = useState({} as OpenLevels);

  const handleListClick = (level: number) => () => {
    setOpenLevels((prevState:OpenLevels) => {
      return { ...prevState, [level]: !prevState[level] };
    });
  };
  const hasChildren = trace.children && trace.children.length;
  const progress = trace.duration / totalMsec * 100;
  return (
    <div>
      <div onClick={handleListClick(trace.idValue)}>
        <div>
          {hasChildren ? <div>
            {openLevels[trace.idValue] ?
              <ArrowUpIcon /> :
              <PlusCircleFillIcon />}
          </div>: null}
          <div>
            <div>
              <BorderLinearProgressWithLabel
                value={progress}
              />
            </div>
            <div>
              {trace.message}
              {`duration: ${trace.duration} ms`}
            </div>
          </div>
        </div>
      </div>
      <>
        <div>
          <div>
            {hasChildren ?
              trace.children.map((trace) => <NestedNav
                key={trace.duration}
                trace={trace}
                totalMsec={totalMsec}
              />) : null}
          </div>
        </div>
      </>
    </div>
  );
};

export default NestedNav;
