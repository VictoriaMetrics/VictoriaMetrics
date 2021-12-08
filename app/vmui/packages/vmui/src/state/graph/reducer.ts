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
    yaxis: YaxisState
}

export type GraphAction =
    | { type: "TOGGLE_ENABLE_YAXIS_LIMITS" }
    | { type: "SET_YAXIS_LIMITS", payload: {[key: string]: [number, number]} }

export const initialGraphState: GraphState = {
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
