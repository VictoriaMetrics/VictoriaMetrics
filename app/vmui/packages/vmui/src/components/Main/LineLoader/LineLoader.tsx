import { FC } from "preact/compat";
import "./style.scss";

const LineLoader: FC = () => {
  return (
    <div className="vm-line-loader">
      <div className="vm-line-loader__background"></div>
      <div className="vm-line-loader__line"></div>
    </div>
  );
};

export default LineLoader;
