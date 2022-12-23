import { TimeParams } from "../types";

export const getJobsUrl = (server: string, period: TimeParams): string =>
  `${server}/api/v1/label/job/values?start=${period.start}&end=${period.end}`;

export const getInstancesUrl = (server: string, period: TimeParams, job: string): string => {
  const match = `{job=${JSON.stringify(job)}}`;
  return `${server}/api/v1/label/instance/values?match[]=${encodeURIComponent(match)}&start=${period.start}&end=${period.end}`;
};

export const getNamesUrl = (server: string, period: TimeParams, job: string, instance: string): string => {
  const filters = Object.entries({ job, instance })
    .filter(val => val[1])
    .map(([key, val]) => `${key}=${JSON.stringify(val)}`)
    .join(",");
  const match = `{${filters}}`;
  return `${server}/api/v1/label/__name__/values?match[]=${encodeURIComponent(match)}&start=${period.start}&end=${period.end}`;
};
