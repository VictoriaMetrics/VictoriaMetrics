import React, { FC, useState } from "preact/compat";
import Button from "../Main/Button/Button";
import TextField from "../Main/TextField/TextField";
import Modal from "../Main/Modal/Modal";
import Spinner from "../Main/Spinner/Spinner";
import { DownloadIcon, ErrorIcon } from "../Main/Icons";
import useBoolean from "../../hooks/useBoolean";
import useDeviceDetect from "../../hooks/useDeviceDetect";
import { useAppState } from "../../state/common/StateContext";
import classNames from "classnames";
import "./style.scss";
import { useQueryState } from "../../state/query/QueryStateContext";
import { useTimeState } from "../../state/time/TimeStateContext";
import { getStepFromDuration } from "../../utils/time";

const AnomalyConfig: FC = () => {
  const { serverUrl } = useAppState();
  const { isMobile } = useDeviceDetect();

  const {
    value: isModalOpen,
    setTrue: setOpenModal,
    setFalse: setCloseModal,
  } = useBoolean(false);

  const { query } = useQueryState();
  const { period } = useTimeState();
  const [isLoading, setIsLoading] = useState(false);
  const [textConfig, setTextConfig] = useState<string>("");
  const [downloadUrl, setDownloadUrl] = useState<string>("");
  const [error, setError] = useState<string>("");

  const fetchConfig = async () => {
    setIsLoading(true);
    try {
      const queryParam = encodeURIComponent(query[0] || "");
      const stepParam = encodeURIComponent(period.step || getStepFromDuration(period.end - period.start, false));

      const url = `${serverUrl}/api/vmanomaly/config.yaml?query=${queryParam}&step=${stepParam}`;
      const response = await fetch(url);
      const contentType = response.headers.get("Content-Type");
      if (!response.ok) {
        const bodyText = await response.text();
        setError(` ${response.status} ${response.statusText}: ${bodyText}`);
      } else if (contentType == "application/yaml") {
        const blob = await response.blob();
        const yamlAsString = await blob.text();
        setTextConfig(yamlAsString);
        setDownloadUrl(URL.createObjectURL(blob));
      } else {
        setError("Response Content-Type is not YAML, does `Server URL` point to VMAnomaly server?");
      }
    } catch (error) {
      console.error(error);
      setError(String(error));
    }
    setIsLoading(false);
  };

  const handleOpenModal = () => {
    setOpenModal();
    setError("");
    URL.revokeObjectURL(downloadUrl);
    setTextConfig("");
    setDownloadUrl("");
    fetchConfig();
  };

  return (
    <>
      <Button
        color="secondary"
        variant="outlined"
        onClick={handleOpenModal}
      >
        Open Config
      </Button>
      {isModalOpen && (
        <Modal
          title="Download config"
          onClose={setCloseModal}
        >
          <div
            className={classNames({
              "vm-anomaly-config": true,
              "vm-anomaly-config_mobile": isMobile,
            })}
          >
            {isLoading && (
              <Spinner
                containerStyles={{ position: "relative" }}
                message={"Loading config..."}
              />
            )}
            {!isLoading && error && (
              <div className="vm-anomaly-config-error">
                <div className="vm-anomaly-config-error__icon"><ErrorIcon/></div>
                <h3 className="vm-anomaly-config-error__title">Cannot download config</h3>
                <p className="vm-anomaly-config-error__text">{error}</p>
              </div>
            )}
            {!isLoading && textConfig && (
              <TextField
                value={textConfig}
                label={"config.yaml"}
                type="textarea"
                disabled={true}
              />
            )}
            <div className="vm-anomaly-config-footer">
              {downloadUrl && (
                <a
                  href={downloadUrl}
                  download={"config.yaml"}
                >
                  <Button
                    variant="contained"
                    startIcon={<DownloadIcon/>}
                  >
                    download
                  </Button>
                </a>
              )}
            </div>
          </div>
        </Modal>
      )}
    </>
  );
};

export default AnomalyConfig;
