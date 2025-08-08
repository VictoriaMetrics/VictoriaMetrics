import "./style.scss";
import { Group as APIGroup } from "../../../types";
import dayjs from "dayjs";

interface BaseGroupProps {
  group: APIGroup;
}

const BaseGroup = ({ group }: BaseGroupProps) => {
  return (
    <div className="vm-explore-alerts-group">
      <div></div>
      <table>
        <tbody>
          {!!group.interval && (
            <tr>
              <td className="vm-col-md">Interval</td>
              <td>{dayjs.duration(group.interval, "seconds").format("H[h] m[m] s[s]")}</td>
            </tr>
          )}
          {!!group.lastEvaluation && (
            <tr>
              <td className="vm-col-md">Last evaluation</td>
              <td>{group.lastEvaluation}</td>
            </tr>
          )}
          {!!group.eval_offset && (
            <tr>
              <td className="vm-col-md">Eval offset</td>
              <td>{dayjs.duration(group.eval_offset, "seconds").format("H[h] m[m] s[s]")}</td>
            </tr>
          )}
          {!!group.eval_delay && (
            <tr>
              <td className="vm-col-md">Eval delay</td>
              <td>{dayjs.duration(group.eval_delay, "seconds").format("H[h] m[m] s[s]")}</td>
            </tr>
          )}
          {!!group.file && (
            <tr>
              <td className="vm-col-md">File</td>
              <td>{group.file}</td>
            </tr>
          )}
          {!!group.concurrency && (
            <tr>
              <td className="vm-col-md">Concurrency</td>
              <td>{group.concurrency}</td>
            </tr>
          )}
          {!!group?.labels?.length && (
            <tr>
              <td className="vm-col-md">Labels</td>
              <td>
                <div className="vm-badge-container">
                  {Object.entries(group.labels).map(([name, value]) => (
                    <span
                      key={name}
                      className="vm-badge"
                    >{`${name}: ${value}`}</span>
                  ))}
                </div>
              </td>
            </tr>
          )}
          {!!group?.params?.length && (
            <tr>
              <td className="vm-col-md">Params</td>
              <td>
                <div className="vm-badge-container">
                  {group.params.map(param => (
                    <span
                      key={param}
                      className="vm-badge"
                    >{`${param}`}</span>
                  ))}
                </div>
              </td>
            </tr>
          )}
          {!!group?.headers?.length && (
            <tr>
              <td className="vm-col-md">Headers</td>
              <td>
                <div className="vm-badge-container">
                  {group.headers.map(header => (
                    <span
                      key={header}
                      className="vm-badge"
                    >{`${header}`}</span>
                  ))}
                </div>
              </td>
            </tr>
          )}
          {!!group?.notifier_headers?.length && (
            <tr>
              <td className="vm-col-md">Notifier headers</td>
              <td>
                <div className="vm-badge-container">
                  {group.notifier_headers.map(header => (
                    <span
                      key={header}
                      className="vm-badge"
                    >{`${header}`}</span>
                  ))}
                </div>
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
};

export default BaseGroup;
