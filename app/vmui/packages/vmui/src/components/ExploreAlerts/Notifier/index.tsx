import React, { FC } from "preact/compat";
import "./style.scss";
import { Target } from "../../../types";

interface NotifierProps {
  targets: Target[]
}

const Notifier: FC<NotifierProps> = ({
  targets,
}) => {

  return (
    <div className="vm-explore-alerts-notifier">
      <table>
        <thead>
          <tr>
            <th className="col-small">Address</th>
            <th>Labels</th>
          </tr>
        </thead>
        <tbody>
          {targets.map(target => (
            <tr key={target.address}>
              <td className="col-small">{target.address}</td>
              <td className="badge-container">
                {Object.entries(target.labels || {}).map(([name, value]) => (
                  <span
                    className="badge"
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
