export type FieldProps<F> = {
  field: F;
  value: unknown;
  onChange: (value: unknown) => void;
};
