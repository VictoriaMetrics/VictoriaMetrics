import { Logs } from "../../../api/types";

export interface ViewProps {
  data: Logs[];
  settingsRef: React.RefObject<HTMLDivElement>;
}