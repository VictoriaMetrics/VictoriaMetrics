import "./style.scss";
import { Alert as APIAlert } from "../../../types";
import { Link, createSearchParams, useNavigate } from "react-router-dom";
import { SearchIcon } from "../../Main/Icons";
import useDeviceDetect from "../../../hooks/useDeviceDetect";
import Tooltip from "../../Main/Tooltip/Tooltip";
import Button from "../../Main/Button/Button";

interface BaseAlertProps {
  item: APIAlert;
}

const BaseAlert = ({ item }: BaseAlertProps) => {
  const query = item?.expression;

  const navigate = useNavigate();
  const { isMobile } = useDeviceDetect();

  const openQueryLink = (time: string) => {
    const params = {
      "g0.expr": query,
      "g0.end_time": time
    };
    return () => {
      navigate({
        pathname: "/",
        search: `?${createSearchParams(params).toString()}`,
      });
    };
  };

  return (
    <div className="vm-explore-alerts-rule-item">
      <table>
        <tbody>
          <tr>
            <td className="vm-col-small">Name</td>
            <td>{item.name}</td>
          </tr>
          <tr>
            <td className="vm-col-small">Group</td>
            <td>
              <Link to={`/rules#group-${item.group_id}`}>{item.group_id}</Link>
            </td>
          </tr>
          <tr>
            <td className="vm-col-small">Query</td>
            <td>
              <pre>
                {isMobile ? (
                  <div>
                    <Button
                      variant="outlined"
                      color="gray"
                      startIcon={<SearchIcon />}
                      onClick={openQueryLink("")}
                    />
                  </div>
                ) : (
                  <div>
                    <Tooltip title="Open in VMUI">
                      <Button
                        variant="outlined"
                        color="gray"
                        startIcon={<SearchIcon />}
                        onClick={openQueryLink("")}
                      />
                    </Tooltip>
                  </div>
                )}
                <code className="language-promql">{query}</code>
              </pre>
            </td>
          </tr>
          {item.labels && (
            <tr>
              <td className="vm-col-small">Labels</td>
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
                  <td className="vm-col-small">{name}</td>
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
