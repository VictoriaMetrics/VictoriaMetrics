import { useMemo } from "preact/compat";
import "./style.scss";
import { Group as APIGroup } from "../../../types";
import dayjs from "dayjs";
import { formatDuration } from "../helpers";
import Badges, { BadgeColor } from "../Badges";

interface BaseGroupProps {
  group: APIGroup;
}

const BaseGroup = ({ group }: BaseGroupProps) => {
  const groupLabels = group?.labels || {};
  const groupLabelsItems = useMemo(() => {
    return Object.fromEntries(Object.entries(groupLabels).map(([name, value]) => [name, {
      color: "passive" as BadgeColor,
      value: value,
    }]));
  }, [groupLabels]);

  const groupParams = group?.params || [];
  const groupParamsItems = useMemo(() => {
    return Object.fromEntries(groupParams.map(value => [value, {
      color: "passive" as BadgeColor,
    }]));
  }, [groupParams]);

  const groupHeaders = group?.headers || [];
  const groupHeadersItems = useMemo(() => {
    return Object.fromEntries(groupHeaders.map(value => [value, {
      color: "passive" as BadgeColor,
    }]));
  }, [groupHeaders]);

  const groupNotifierHeaders = group?.notifier_headers || [];
  const groupNotifierHeadersItems = useMemo(() => {
    return Object.fromEntries(groupNotifierHeaders.map(value => [value, {
      color: "passive" as BadgeColor,
    }]));
  }, [groupNotifierHeaders]);
  return (
    <div className="vm-explore-alerts-group">
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
          {!!Object.keys(groupLabels).length && (
            <tr>
              <td className="vm-col-md">Labels</td>
              <td>
                <Badges
                  items={groupLabelsItems}
                />
              </td>
            </tr>
          )}
          {!!groupParams.length && (
            <tr>
              <td className="vm-col-md">Params</td>
              <td>
                <Badges
                  items={groupParamsItems}
                />
              </td>
            </tr>
          )}
          {!!groupHeaders.length && (
            <tr>
              <td className="vm-col-md">Headers</td>
              <td>
                <Badges
                  items={groupHeadersItems}
                />
              </td>
            </tr>
          )}
          {!!groupNotifierHeaders.length && (
            <tr>
              <td className="vm-col-md">Notifier headers</td>
              <td>
                <Badges
                  items={groupNotifierHeadersItems}
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
