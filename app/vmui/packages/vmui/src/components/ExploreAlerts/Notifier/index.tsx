import { FC } from "preact/compat";
import "./style.scss";
import { Target as APITarget } from "../../../types";

interface NotifierProps {
  targets: APITarget[]
}

const Notifier: FC<NotifierProps> = ({
  targets,
}) => {

  return (
    <div className="vm-explore-alerts-notifier">
      <table>
        <thead>
          <tr>
            <th className="vm-col-small">Address</th>
            <th>Labels</th>
          </tr>
        </thead>
        <tbody>
          {targets.map(target => (
            <tr key={target.address}>
              <td className="vm-col-small">{target.address}</td>
              <td className="vm-badge-container">
                {Object.entries(target.labels || {}).map(([name, value]) => (
                  <span
                    className="vm-badge"
                    key={name}
                  >{`${name}: ${value}`}</span>
                ))}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
};

export default Notifier;
