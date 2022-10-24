export interface AxisRange {
  [key: string]: [number, number]
}

export interface YaxisState {
  limits: {
    enable: boolean,
    range: AxisRange
  }
}

export interface GraphState {
  customStep: number
  yaxis: YaxisState
}

export type GraphAction =
  | { type: "TOGGLE_ENABLE_YAXIS_LIMITS" }
  | { type: "SET_YAXIS_LIMITS", payload: AxisRange }
  | { type: "SET_CUSTOM_STEP", payload: number}

export const initialGraphState: GraphState = {
  customStep: 1,
  yaxis: {
    limits: {enable: false, range: {"1": [0, 0]}}
  }
};

export function reducer(state: GraphState, action: GraphAction): GraphState {
  switch (action.type) {
    case "TOGGLE_ENABLE_YAXIS_LIMITS":
      return {
        ...state,
        yaxis: {
          ...state.yaxis,
          limits: {
            ...state.yaxis.limits,
            enable: !state.yaxis.limits.enable
          }
        }
      };
    case "SET_CUSTOM_STEP":
      return {
        ...state,
        customStep: action.payload
      };
    case "SET_YAXIS_LIMITS":
      return {
        ...state,
        yaxis: {
          ...state.yaxis,
          limits: {
            ...state.yaxis.limits,
            range: action.payload
          }
        }
      };
    default:
      throw new Error();
  }
}
