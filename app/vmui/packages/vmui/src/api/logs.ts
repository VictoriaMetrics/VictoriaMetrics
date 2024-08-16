export const getLogsUrl = (server: string): string =>
  `${server}/select/logsql/query`;

export const getLogHitsUrl = (server: string): string =>
  `${server}/select/logsql/hits`;
