import { FC, useState, useEffect } from "preact/compat";
import { JSX } from "preact";
import { ArrowDownIcon } from "../Icons";
import "./style.scss";
import { ReactNode } from "react";

interface AccordionProps {
  id?: string
  title: ReactNode
  children: ReactNode
  defaultExpanded?: boolean
  onChange?: (value: boolean) => void
}

const Accordion: FC<AccordionProps> = ({
  defaultExpanded = false,
  onChange,
  title,
  children,
  id,
}) => {
  const [isOpen, setIsOpen] = useState(defaultExpanded);

  const toggleOpen = (event: JSX.TargetedMouseEvent<HTMLElement>) => {
    const selection = window.getSelection();
    if ((event.target as HTMLElement).closest("button")) {
      event.preventDefault();
      return; // If the text is selected, cancel the execution of toggle.
    }
    if (selection && selection.toString()) {
      event.preventDefault();
      return; // If the text is selected, cancel the execution of toggle.
    }
    const details = event.currentTarget.parentElement as HTMLDetailsElement;
    onChange && onChange(details.open);
    setIsOpen(details.open);
  };

  useEffect(() => {
    setIsOpen(defaultExpanded);
  }, [defaultExpanded]);

  return (
    <>
      <details
        className="vm-accordion-section"
        key="content"
        open={isOpen}
        id={id}
      >
        <summary
          className="vm-accordion-header"
          onClick={toggleOpen}
        >
          {title}
          <div className="vm-accordion-header__arrow">
            <ArrowDownIcon />
          </div>
        </summary>
        {children}
      </details>
    </>
  );
};

export default Accordion;
