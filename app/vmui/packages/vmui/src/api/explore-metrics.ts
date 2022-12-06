import { TimeParams } from "../types";

export const getJobsUrl = (server: string, period: TimeParams): string =>
  `${server}/api/v1/label/job/values?start=${period.start}&end=${period.end}`;

export const getInstancesUrl = (server: string, period: TimeParams, job: string): string =>
  `${server}/api/v1/label/instance/values?match[]={job="${job}"}&start=${period.start}&end=${period.end}`;

export const getNamesUrl = (server: string, job: string): string =>
  `${server}/api/v1/label/__name__/values?match[]={job="${job}"}`;
