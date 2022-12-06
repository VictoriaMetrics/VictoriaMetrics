import dayjs from "dayjs";
import timezone from "dayjs/plugin/timezone";
import duration from "dayjs/plugin/duration";
import utc from "dayjs/plugin/utc";

dayjs.extend(timezone);
dayjs.extend(duration);
dayjs.extend(utc);
