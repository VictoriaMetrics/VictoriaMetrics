import React, { FC } from "preact/compat";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";
import { CloseIcon } from "../../Icons";
import { MouseEvent } from "react";

interface MultipleSelectedValueProps {
  values: string[]
  onRemoveItem: (val: string) => void
}

const MultipleSelectedValue: FC<MultipleSelectedValueProps> = ({ values, onRemoveItem }) => {
  const { isMobile } = useDeviceDetect();

  const createHandleClick = (value: string) => (e: MouseEvent) => {
    onRemoveItem(value);
    e.stopPropagation();
  };

  if (isMobile) {
    return (
      <span className="vm-select-input-content__counter">
        selected {values.length}
      </span>
    );
  }

  return <>
    {values.map(item => (
      <div
        className="vm-select-input-content__selected"
        key={item}
      >
        <span>{item}</span>
        <div onClick={createHandleClick(item)}>
          <CloseIcon/>
        </div>
      </div>
    ))}
  </>;
};

export default MultipleSelectedValue;
