export interface AxisRange {
  [key: string]: [number, number]
}

export interface YaxisState {
  limits: {
    enable: boolean,
    range: AxisRange
  }
}

export interface CustomStep {
    enable: boolean,
    value: number
}

export interface GraphState {
  customStep: CustomStep
  yaxis: YaxisState
}

export type GraphAction =
  | { type: "TOGGLE_ENABLE_YAXIS_LIMITS" }
  | { type: "SET_YAXIS_LIMITS", payload: AxisRange }
  | { type: "TOGGLE_CUSTOM_STEP" }
  | { type: "SET_CUSTOM_STEP", payload: number}

export const initialGraphState: GraphState = {
  customStep: {enable: false, value: 1},
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
    case "TOGGLE_CUSTOM_STEP":
      return {
        ...state,
        customStep: {
          ...state.customStep,
          enable: !state.customStep.enable
        }
      };
    case "SET_CUSTOM_STEP":
      return {
        ...state,
        customStep: {
          ...state.customStep,
          value: action.payload
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
