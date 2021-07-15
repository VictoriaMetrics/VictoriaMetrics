import React from "react";

interface LineI {
  height: number;
  x: number | undefined;
}

export const InteractionLine: React.FC<LineI> = ({height, x}) => {
  return <>{x && <line x1={x} y1="0" x2={x} y2={height} stroke="black" strokeDasharray="4" />}</>;
};
