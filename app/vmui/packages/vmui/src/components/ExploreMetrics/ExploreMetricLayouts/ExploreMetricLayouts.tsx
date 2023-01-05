import React, { FC } from "preact/compat";
import "./style.scss";
import { sizeVariants } from "../ExploreMetricItem/ExploreMetricItem";
import classNames from "classnames";
import { DoneIcon } from "../../Main/Icons";

interface ExploreMetricLayoutsProps {
  value: string
  onChange: (id: string) => void
}

const ExploreMetricLayouts: FC<ExploreMetricLayoutsProps> = ({
  value,
  onChange
}) => {

  const createHandlerClick = (id: string) => () => {
    onChange(id);
  };

  return (
    <div className="vm-explore-metrics-layouts">
      {sizeVariants.map(variant => (
        <div
          className={classNames({
            "vm-list-item": true,
            "vm-list-item_multiselect": true,
            "vm-list-item_multiselect_selected": variant.id === value
          })}
          key={variant.id}
          onClick={createHandlerClick(variant.id)}
        >
          {variant.id === value && <DoneIcon/>}
          <span>{variant.id}</span>
        </div>
      ))}
    </div>
  );
};

export default ExploreMetricLayouts;
