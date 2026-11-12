import { createContext, FC, useCallback, useContext, useLayoutEffect, useMemo, useState } from "preact/compat";
import { useLocation, useNavigationType, useSearchParams } from "react-router-dom";
import { useTimeState } from "../time/TimeStateContext";
import { useCustomPanelState } from "../customPanel/CustomPanelStateContext";
import { useGraphState } from "../graph/GraphStateContext";
import { DisplayType } from "../../types";
import { getStepFromDuration } from "../../utils/time";
import router from "../../router";

interface StepContextValue {
  calculated: string;
  override: string | null;
  effective: string;
  setOverride: (value: string) => void;
  reset: () => void;
}

const StepContext = createContext<StepContextValue>({} as StepContextValue);

const normalizeStep = (value: string | null): string | null => {
  if (!value) return null;
  if (!/^[0-9]*\.?[0-9]+$/.test(value)) return value;
  return `${value.startsWith(".") ? "0" : ""}${value}s`;
};

export const useStep = (): StepContextValue => useContext(StepContext);

export const StepStateProvider: FC = ({ children }) => {
  const { period: { start, end } } = useTimeState();
  const { displayType } = useCustomPanelState();
  const { isHistogram } = useGraphState();
  const { pathname } = useLocation();
  const navigationType = useNavigationType();
  const [searchParams, setSearchParams] = useSearchParams();
  const fromUrl = normalizeStep(searchParams.get("g0.step_input"));
  const [previous, setPrevious] = useState({ pathname, override: fromUrl });

  // Ordinary page navigation keeps the manual setting. History navigation and
  // explicit URL values restore the setting from that history entry instead.
  const carryOverride = pathname !== previous.pathname && navigationType !== "POP" && !fromUrl;
  const override = carryOverride ? previous.override : fromUrl;
  const isQueryPage = pathname === router.home || pathname === router.query;
  const calculated = getStepFromDuration(
    end - start,
    isQueryPage && isHistogram,
    isQueryPage ? displayType : DisplayType.chart
  );

  const writeOverride = useCallback((value: string | null, replace = false) => {
    const normalized = normalizeStep(value);
    setSearchParams(current => {
      const next = new URLSearchParams(current);
      const keys = Array.from(next.keys()).filter(key => /^g\d+\.step_input$/.test(key));
      keys.forEach(key => normalized ? next.set(key, normalized) : next.delete(key));
      if (normalized) next.set("g0.step_input", normalized);
      return next;
    }, { replace });
  }, [setSearchParams]);

  useLayoutEffect(() => {
    if (pathname !== previous.pathname || override !== previous.override) {
      setPrevious({ pathname, override });
    }
    if (carryOverride && override) writeOverride(override, true);
  }, [pathname, override, previous, carryOverride, writeOverride]);

  const setOverride = useCallback((value: string) => writeOverride(value || null), [writeOverride]);

  const reset = useCallback(() => writeOverride(null), [writeOverride]);

  const contextValue = useMemo(() => ({
    // Automatic step derived from the time range, page, display type, and histogram mode.
    calculated,
    // Manual step from the URL; null means automatic mode.
    override,
    // Shared step used by the header, API requests, and graphs.
    effective: override ?? calculated,
    setOverride,
    reset,
  }), [calculated, override, setOverride, reset]);

  return <StepContext.Provider value={contextValue}>{children}</StepContext.Provider>;
};
