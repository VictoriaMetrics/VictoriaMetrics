/* eslint max-lines: ["error", {"max": 200}] */                // Complex D3 logic here - file can be larger
import React, {useEffect, useMemo, useRef, useState} from "react";
import {bisector, brushX, pointer as d3Pointer, ScaleLinear, ScaleTime, select as d3Select} from "d3";

interface LineI {
  yScale: ScaleLinear<number, number>;
  xScale: ScaleTime<number, number>;
  datesInChart: Date[];
  setSelection: (from: Date, to: Date) => void;
  onInteraction: (index: number | undefined, y: number | undefined) => void; // key is index. undefined means no interaction
}

export const InteractionArea: React.FC<LineI> = ({yScale, xScale, datesInChart, onInteraction, setSelection}) => {
  const refBrush = useRef<SVGGElement>(null);

  const [currentActivePoint, setCurrentActivePoint] = useState<number>();
  const [currentY, setCurrentY] = useState<number>();
  const [isBrushed, setIsBrushed] = useState(false);

  // eslint-disable-next-line @typescript-eslint/no-explicit-any,@typescript-eslint/explicit-function-return-type
  function brushEnded(this: any, event: any) {
    const selection = event.selection;
    if (selection) {
      if (!event.sourceEvent) return; // see comment in brushstarted
      setIsBrushed(true);
      const [from, to]: [Date, Date] = selection.map((s: number) => xScale.invert(s));
      setSelection(from, to);
      // eslint-disable-next-line @typescript-eslint/no-explicit-any
      d3Select(refBrush.current).call(brush.move as any, null); // clean brush
    } else {
      // end event with empty selection means that we're cancelling brush
      setIsBrushed(false);
    }
  }

  // eslint-disable-next-line @typescript-eslint/no-explicit-any
  const brushStarted = (event: any): void => {
    // first of all: event is a d3 global value that stores current event (sort of).
    // This is weird but this is how d3 works with events.
    //This check is important:
    // Inside brushended - we have .call(brush.move, ...) in order to snap selected range to dates
    // that internally calls brushstarted again. But in this case sourceEvent is null, since the call
    // is programmatic. If we do not need to adjust selected are - no need to have this check (probably)
    if (event.sourceEvent) {
      setCurrentActivePoint(undefined);
    }
  };

  const brush = useMemo(
    () =>
      brushX()
        .extent([
          [0, 0],
          [xScale.range()[1], yScale.range()[0]]
        ])
        .on("end", brushEnded)
        .on("start", brushStarted),
    [brushEnded, xScale, yScale]
  );

  // Needed to clean brush if we need to keep it

  // const resetBrushHandler = useCallback(
  //   (e) => {
  //     const el = e.target as HTMLElement;
  //     if (
  //       el &&
  //       el.tagName !== "rect" &&
  //       e.target.classList.length &&
  //       !e.target.classList.contains("selection")
  //     ) {
  //       // eslint-disable-next-line @typescript-eslint/no-explicit-any
  //       d3Select(refBrush.current).call(brush.move as any, null);
  //     }
  //   },
  //   [brush.move]
  // );

  // useEffect(() => {
  //   window.addEventListener("click", resetBrushHandler);
  //   return () => {
  //     window.removeEventListener("click", resetBrushHandler);
  //   };
  // }, [resetBrushHandler]);

  useEffect(() => {
    const bisect = bisector((d: Date) => d).center;
    const defineActivePoint = (mx: number): void => {
      const date = xScale.invert(mx); // date is a Date object
      const index = bisect(datesInChart, date, 1);
      setCurrentActivePoint(index);
    };

    d3Select(refBrush.current)
      .on("touchmove mousemove", (event) => {
        const coords: [number, number] = d3Pointer(event);
        if (!isBrushed) {
          defineActivePoint(coords[0]);
          setCurrentY(coords[1]);
        }
      })
      .on("mouseout", () => {
        if (!isBrushed) {
          setCurrentActivePoint(undefined);
        }
      });
  }, [xScale, datesInChart, isBrushed]);

  useEffect(() => {
    onInteraction(currentActivePoint, currentY);
  }, [currentActivePoint, currentY, onInteraction]);

  useEffect(() => {
    // eslint-disable-next-line @typescript-eslint/ban-ts-comment
    // @ts-ignore
    brush && xScale && d3Select(refBrush.current).call(brush);
  }, [xScale, brush]);

  return (
    <>
      <g ref={refBrush} />
    </>
  );
};
