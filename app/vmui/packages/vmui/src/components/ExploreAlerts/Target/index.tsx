import { FC, useMemo } from "preact/compat";
import "./style.scss";
import { Target as APITarget } from "../../../types";
import Alert from "../../Main/Alert/Alert";
import Accordion from "../../Main/Accordion/Accordion";
import Badges, { BadgeColor } from "../Badges";

interface TargetProps {
  target: APITarget;
}

const Target: FC<TargetProps> = ({ target }) => {
  const state = target?.lastError ? "unhealthy" : "ok";
  const targetLabels = target?.labels || {};
  const badgesItems = useMemo(() => {
    return Object.fromEntries(Object.entries(targetLabels).map(([name, value]) => [name, {
      value: value,
      color: "passive" as BadgeColor,
    }]));
  }, [targetLabels]);
  return (
    <div className={`vm-explore-alerts-target vm-badge-item ${state.replace(" ", "-")}`}>
      {(!!target?.labels?.length || !!target?.lastError) ? (
        <Accordion
          key={`target-${target.address}`}
          title={(
            <div className="vm-explore-alerts-target-header__name">{target.address}</div>
          )}
        >
          <div className="vm-explore-alerts-target-item">
            <table>
              <tbody>
                {!!Object.keys(targetLabels).length && (
                  <tr>      
                    <td className="vm-col-md">Labels</td>
                    <td>
                      <Badges
                        items={badgesItems}
                      />
                    </td>
                  </tr>
                )}
                {!!target.lastError && (
                  <tr>
                    <td className="vm-col-md">Last error</td>
                    <td>
                      <Alert variant="error">{target.lastError}</Alert>
                    </td>
                  </tr>
                )}
              </tbody>
            </table>
          </div>
        </Accordion>
      ) : (
        <span>{target.address}</span>
      )}
    </div>
  );
};

export default Target;
