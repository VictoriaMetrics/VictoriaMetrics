import React, { FC, useState } from "preact/compat";
import { ArrowDownIcon } from "../Icons";
import "./style.scss";
import { ReactNode } from "react";

interface AccordionProps {
  title: ReactNode
  children: ReactNode
  defaultExpanded?: boolean
}

const Accordion: FC<AccordionProps> = ({
  defaultExpanded = false, children, title,
}) => {
  const [isOpen, setIsOpen] = useState(defaultExpanded);

  return (
    <>
      <header
        className={`vm-accordion-header ${isOpen && "vm-accordion-header_open"}`}
        onClick={() => setIsOpen(prev => !prev)}
      >
        {title}
        <div className={`vm-accordion-header__arrow ${isOpen && "vm-accordion-header__arrow_open"}`}>
          <ArrowDownIcon />
        </div>
      </header>
      {isOpen && (
        <section
          className="vm-accordion-section"
          key="content"
        >
          {children}
        </section>
      )}
    </>
  );
};

export default Accordion;
