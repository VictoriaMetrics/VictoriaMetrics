import React, { FC, useState, useEffect } from "preact/compat";
import { ArrowDownIcon } from "../Icons";
import "./style.scss";
import { ReactNode } from "react";

interface AccordionProps {
  title: ReactNode
  children: ReactNode
  defaultExpanded?: boolean
  onChange?: (value: boolean) => void
}

const Accordion: FC<AccordionProps> = ({
  defaultExpanded = false,
  onChange,
  title,
  children
}) => {
  const [isOpen, setIsOpen] = useState(defaultExpanded);

  const toggleOpen = (event: { currentTarget: { open: boolean } }) => {
    const selection = window.getSelection();
    if (selection && selection.toString()) {
      event.preventDefault();
      return; // If the text is selected, cancel the execution of toggle.
    }
    onChange && onChange(event.currentTarget.open);
    setIsOpen(event.currentTarget.open);
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
