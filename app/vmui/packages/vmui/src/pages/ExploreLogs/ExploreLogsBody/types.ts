import { Logs } from "../../../api/types";
import { RefObject } from "react";

export interface ViewProps {
  data: Logs[];
  settingsRef: RefObject<HTMLDivElement | null>;
}
