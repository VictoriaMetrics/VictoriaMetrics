import "./style.scss";
import { Group as APIGroup } from "../../../types";
import dayjs from "dayjs";
import { formatDuration } from "../helpers";
import Badges from "../Badges";

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
              <td>{formatDuration(group.interval)}</td>
            </tr>
          )}
          {!!group.lastEvaluation && (
            <tr>
              <td className="vm-col-md">Last evaluation</td>
              <td>{dayjs(group.lastEvaluation).format("DD MMM YYYY HH:mm:ss")}</td>
            </tr>
          )}
          {!!group.eval_offset && (
            <tr>
              <td className="vm-col-md">Eval offset</td>
              <td>{formatDuration(group.eval_offset)}</td>
            </tr>
          )}
          {!!group.eval_delay && (
            <tr>
              <td className="vm-col-md">Eval delay</td>
              <td>{formatDuration(group.eval_delay)}</td>
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
                <Badges
                  items={Object.fromEntries(Object.entries(group.labels).map(([name, value]) => [name, {
                    color: "passive",
                    value: value,
                  }]))}
                />
              </td>
            </tr>
          )}
          {!!group?.params?.length && (
            <tr>
              <td className="vm-col-md">Params</td>
              <td>
                <Badges
                  items={Object.fromEntries(group.params.map(value => [value, {
                    color: "passive",
                  }]))}
                />
              </td>
            </tr>
          )}
          {!!group?.headers?.length && (
            <tr>
              <td className="vm-col-md">Headers</td>
              <td>
                <Badges
                  items={Object.fromEntries(group.headers.map(value => [value, {
                    color: "passive",
                  }]))}
                />
              </td>
            </tr>
          )}
          {!!group?.notifier_headers?.length && (
            <tr>
              <td className="vm-col-md">Notifier headers</td>
              <td>
                <Badges
                  items={Object.fromEntries(group.notifier_headers.map(value => [value, {
                    color: "passive",
                  }]))}
                />
              </td>
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
};

export default BaseGroup;
