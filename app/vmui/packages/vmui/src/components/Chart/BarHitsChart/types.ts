export enum GRAPH_STYLES {
  BAR = "Bars",
  LINE = "Lines",
  LINE_STEPPED = "Stepped lines",
  POINTS = "Points",
}

export interface GraphOptions {
  graphStyle: GRAPH_STYLES;
  stacked: boolean;
  fill: boolean;
}
