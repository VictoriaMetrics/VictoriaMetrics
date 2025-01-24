import React, { FC, useMemo, useRef, useState } from "preact/compat";
import classNames from "classnames";
import { Series } from "uplot";
import { MouseEvent } from "react";
import { LegendLogHits } from "../../../../api/types";
import { getStreamPairs } from "../../../../utils/logs";
import { formatNumberShort } from "../../../../utils/math";
import Popper from "../../../Main/Popper/Popper";
import useBoolean from "../../../../hooks/useBoolean";
import LegendHitsMenu from "../LegendHitsMenu/LegendHitsMenu";

interface Props {
  legend: LegendLogHits;
  series: Series[];
  onRedrawGraph: () => void;
  onApplyFilter: (value: string) => void;
}

const BarHitsLegendItem: FC<Props> = ({ legend, series, onRedrawGraph, onApplyFilter }) => {
  const {
    value: openContextMenu,
    setTrue: handleOpenContextMenu,
    setFalse: handleCloseContextMenu,
  } = useBoolean(false);

  const legendRef = useRef<HTMLDivElement>(null);
  const [clickPosition, setClickPosition] = useState<{ top: number; left: number } | null>(null);

  const targetSeries = useMemo(() => series.find(s => s.label === legend.label), [series]);

  const fields = useMemo(() => getStreamPairs(legend.label), [legend.label]);

  const label = fields.join(", ");
  const totalShortFormatted = formatNumberShort(legend.total);

  const handleClickByStream = (e: MouseEvent<HTMLDivElement>) => {
    if (!targetSeries) return;

    if (e.metaKey || e.ctrlKey) {
      targetSeries.show = !targetSeries.show;
    } else {
      const isOnlyTargetVisible = series.every(s => s === targetSeries || !s.show);
      series.forEach(s => {
        s.show = isOnlyTargetVisible || (s === targetSeries);
      });
    }

    onRedrawGraph();
  };

  const handleContextMenu = (e: MouseEvent<HTMLDivElement>) => {
    e.preventDefault();
    setClickPosition({ top: e.clientY, left: e.clientX });
    handleOpenContextMenu();
  };

  return (
    <div
      ref={legendRef}
      className={classNames({
        "vm-bar-hits-legend-item": true,
        "vm-bar-hits-legend-item_other": legend.isOther,
        "vm-bar-hits-legend-item_hide": !targetSeries?.show,
      })}
      onClick={handleClickByStream}
      onContextMenu={handleContextMenu}
    >
      <div
        className="vm-bar-hits-legend-item__marker"
        style={{ backgroundColor: `${legend.stroke}` }}
      />
      <div className="vm-bar-hits-legend-item__label">{label}</div>
      <span className="vm-bar-hits-legend-item__total">({totalShortFormatted})</span>
      <Popper
        placement="fixed"
        open={openContextMenu}
        buttonRef={legendRef}
        placementPosition={clickPosition}
        onClose={handleCloseContextMenu}
      >
        <LegendHitsMenu
          legend={legend}
          fields={fields}
          onApplyFilter={onApplyFilter}
          onClose={handleCloseContextMenu}
        />
      </Popper>
    </div>
  );
};

export default BarHitsLegendItem;
