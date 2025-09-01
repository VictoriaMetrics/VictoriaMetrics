import dayjs from "dayjs";

export const formatDuration = (raw: number) => {
  const duration = dayjs.duration(Math.round(raw * 1000));
  const fmt = [];
  if (duration.get("day")) fmt.push("D[d]");
  if (duration.get("hour")) fmt.push("H[h]");
  if (duration.get("minute")) fmt.push("m[m]");
  if (duration.get("millisecond")) {
    fmt.push("s.SSS[s]");
  } else if (!fmt.length || duration.get("second")) {
    fmt.push("s[s]");
  }
  return duration.format(fmt.join(" "));
};
