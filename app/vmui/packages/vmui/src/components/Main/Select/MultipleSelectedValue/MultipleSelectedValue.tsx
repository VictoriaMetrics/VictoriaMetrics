import { FC, MouseEvent } from "react";
import useDeviceDetect from "../../../../hooks/useDeviceDetect";
import { CloseIcon } from "../../Icons";

interface MultipleSelectedValueProps {
  values: string[]
  onRemoveItem: (val: string) => void
}

const MultipleSelectedValue: FC<MultipleSelectedValueProps> = ({ values, onRemoveItem }) => {
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
