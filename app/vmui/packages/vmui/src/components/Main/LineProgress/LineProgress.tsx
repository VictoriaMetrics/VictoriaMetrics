import "./style.scss";

type Props = {
  value: number;
  hideValue?: boolean;
}

const LineProgress = ({ value, hideValue }: Props) => (
  <div className="vm-line-progress">
    <div className="vm-line-progress-track">
      <div
        className="vm-line-progress-track__thumb"
        style={{ width: `${value}%` }}
      />
    </div>
    {!hideValue && (
      <span>{value.toFixed(2)}%</span>
    )}
  </div>
);

export default LineProgress;
