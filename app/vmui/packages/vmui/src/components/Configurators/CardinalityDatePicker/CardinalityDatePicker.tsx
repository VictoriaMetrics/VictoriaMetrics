import React, { FC } from "preact/compat";
import DatePicker from "../../Main/DatePicker/DatePicker";
import { useCardinalityState, useCardinalityDispatch } from "../../../state/cardinality/CardinalityStateContext";

const CardinalityDatePicker: FC = () => {
  const { date } = useCardinalityState();
  const cardinalityDispatch = useCardinalityDispatch();

  return (
    <DatePicker
      date={date}
      onChange={(val) => cardinalityDispatch({ type: "SET_DATE", payload: val })}
    />
  );
};

export default CardinalityDatePicker;
