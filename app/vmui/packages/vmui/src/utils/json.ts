export const parseLineToJSON = (line: string) => {
  try {
    return JSON.parse(line);
  } catch (e) {
    return null;
  }
};
