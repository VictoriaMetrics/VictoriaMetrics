import "./style.scss";
import { Alert as APIAlert } from "../../../types";
import { createSearchParams } from "react-router-dom";
import Button from "../../Main/Button/Button";
import Badges from "../Badges";
import {
  SearchIcon,
} from "../../Main/Icons";
import dayjs from "dayjs";

interface BaseAlertProps {
  item: APIAlert;
}

const BaseAlert = ({ item }: BaseAlertProps) => {
  const query = item?.expression;

  const openQueryLink = () => {
    const params = {
      "g0.expr": query,
      "g0.end_time": ""
    };
    window.open(`#/?${createSearchParams(params).toString()}`, "_blank", "noopener noreferrer");
  };

  return (
    <div className="vm-explore-alerts-alert-item">
      <table>
        <tbody>
          <tr>
            <td
              style={{ "text-align": "end" }}
              colSpan={2}
            >
              <Button
                size="small"
                variant="outlined"
                color="gray"
                startIcon={<SearchIcon />}
                onClick={openQueryLink}
              >
                <span className="vm-button-text">Run query</span>
              </Button>
            </td>
          </tr>
          <tr>
            <td className="vm-col-md">Query</td>
            <td>
              <pre>
                <code className="language-promql">{query}</code>
              </pre>
            </td>
          </tr>
          <tr>
            <td className="vm-col-md">Active at</td>
            <td>{dayjs(item.activeAt).format("DD MMM YYYY HH:mm:ss")}</td>
          </tr>
          {!!Object.keys(item?.labels || {}).length && (
            <tr>
              <td className="vm-col-md">Labels</td>
              <td>
                <Badges
                  items={Object.fromEntries(Object.entries(item.labels).map(([name, value]) => [name, {
                    color: "passive",
                    value: value,
                  }]))}
                />
              </td>
            </tr>
          )}
        </tbody>
      </table>
      {!!Object.keys(item.annotations || {}).length && (
        <>
          <span className="title">Annotations</span>
          <table>
            <tbody>
              {Object.entries(item.annotations || {}).map(([name, value]) => (
                <tr key={name}>
                  <td className="vm-col-md">{name}</td>
                  <td>{value}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
    </div>
  );
};

export default BaseAlert;
