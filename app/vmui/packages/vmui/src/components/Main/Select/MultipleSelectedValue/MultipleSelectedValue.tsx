import { FC } from "preact/compat";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";
import { CloseIcon } from "../../Icons";
import { MouseEvent } from "react";

interface MultipleSelectedValueProps {
  values: string[]
  itemClassName?: string
  onRemoveItem: (val: string) => void
}

const MultipleSelectedValue: FC<MultipleSelectedValueProps> = ({ values, itemClassName, onRemoveItem }) => {
  const { isMobile } = useDeviceDetect();

  const createHandleClick = (value: string) => (e: MouseEvent<HTMLDivElement>) => {
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
        className={`vm-select-input-content__selected ${itemClassName} ${item.toLowerCase().replace(" ", "-")}`}
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
