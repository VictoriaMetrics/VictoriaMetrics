import "./style.scss";
import { Alert as APIAlert } from "../../../types";
import { createSearchParams } from "react-router-dom";
import Button from "../../Main/Button/Button";
import {
  SearchIcon,
} from "../../Main/Icons";

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
    window.open(`/#/?${createSearchParams(params).toString()}`, "_blank", "noopener noreferrer");
  };

  return (
    <div className="vm-explore-alerts-rule-item">
      <table>
        <tbody>
          <tr className="align-end">
            <td colSpan={2}>
              <Button
                size="small"
                variant="outlined"
                color="gray"
                startIcon={<SearchIcon />}
                onClick={openQueryLink}
              >
                <span className="vm-button-text">Show query</span>
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
          {item.labels && (
            <tr>
              <td className="vm-col-md">Labels</td>
              <td>
                <div className="vm-badge-container">
                  {Object.entries(item.labels).map(([name, value]) => (
                    <span
                      key={name}
                      className="vm-badge"
                    >{`${name}: ${value}`}</span>
                  ))}
                </div>
              </td>
            </tr>
          )}
        </tbody>
      </table>
      {item.annotations && (
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
