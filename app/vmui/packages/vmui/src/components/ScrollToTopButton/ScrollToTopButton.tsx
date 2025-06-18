import { FC, useEffect, useState } from "preact/compat";
import Button from "../Main/Button/Button";
import Tooltip from "../Main/Tooltip/Tooltip";
import { ScrollToTopIcon } from "../Main/Icons";
import classNames from "classnames";
import "./style.scss";
import { useCallback } from "react";

interface ScrollToTopButtonProps {
  className?: string;
}

const ScrollToTopButton: FC<ScrollToTopButtonProps> = ({ className }) => {
  const [isVisible, setIsVisible] = useState(false);

  const checkScrollPosition = () => {
    const scrollPosition = window.pageYOffset || document.documentElement.scrollTop;
    const visibleHeightThreshold = window.innerHeight;

    setIsVisible(scrollPosition > visibleHeightThreshold);
  };

  const scrollToTop = useCallback(() => {
    window.scrollTo({
      top: 0,
      behavior: "smooth"
    });
  }, []);

  useEffect(() => {
    window.addEventListener("scroll", checkScrollPosition);
    checkScrollPosition();
    
    return () => {
      window.removeEventListener("scroll", checkScrollPosition);
    };
  }, []);

  return (
    <div
      className={classNames({
        "vm-scroll-to-top-button": true,
        "vm-scroll-to-top-button_visible": isVisible
      }, className)}
    >
      <Tooltip title="Scroll to top">
        <Button
          variant="contained"
          color="primary"
          onClick={scrollToTop}
          ariaLabel="Scroll to top"
          startIcon={<ScrollToTopIcon />}
        />
      </Tooltip>
    </div>
  );
};

export default ScrollToTopButton;
