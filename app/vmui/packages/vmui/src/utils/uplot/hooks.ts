import uPlot from "uplot";

export const delHooks = (u: uPlot) => {
  Object.keys(u.hooks).forEach(hook => {
    u.hooks[hook as keyof uPlot.Hooks.Arrays] = [];
  });
};
