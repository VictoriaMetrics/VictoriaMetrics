import dayjs from "dayjs";
import timezone from "dayjs/plugin/timezone";
import duration from "dayjs/plugin/duration";
import relativeTime from "dayjs/plugin/relativeTime";
import utc from "dayjs/plugin/utc";

dayjs.extend(timezone);
dayjs.extend(duration);
dayjs.extend(utc);
dayjs.extend(relativeTime);
