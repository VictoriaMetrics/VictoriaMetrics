import { FC, useState, useEffect } from "preact/compat";
import classNames from "classnames";
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

    setIsOpen((prev) => {
      const newState = !prev;
      onChange && onChange(newState);
      return newState;
    });
  };

  useEffect(() => {
    setIsOpen(defaultExpanded);
  }, [defaultExpanded]);

  return (
    <>
      <header
        className={classNames({
          "vm-accordion-header": true,
          "vm-accordion-header_open": isOpen,
        })}
        onClick={toggleOpen}
        id={id}
      >
        {title}
        <div
          className={classNames({
            "vm-accordion-header__arrow": true,
            "vm-accordion-header__arrow_open": isOpen,
          })}
        >
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
