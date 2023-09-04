import React, { ComponentProps, FC } from "react";

type Props = { children: JSX.Element };

export const combineComponents = (...components: FC<Props>[]): FC<Props> => {
  return components.reduce(
    (AccumulatedComponents, CurrentComponent) => {
      // eslint-disable-next-line react/display-name
      return ({ children }: ComponentProps<FC<Props>>): JSX.Element => (
        <AccumulatedComponents>
          <CurrentComponent>{children}</CurrentComponent>
        </AccumulatedComponents>
      );
    },
    ({ children }) => <>{children}</>,
  );
};
