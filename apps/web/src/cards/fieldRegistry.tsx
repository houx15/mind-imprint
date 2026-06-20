import type { ComponentType } from "react";
import type { FieldType } from "@mind-imprint/contracts";
import type { FieldProps } from "./fields/types";
import { TextField } from "./fields/TextField";
import { TextAreaField } from "./fields/TextAreaField";
import { SingleChoiceField } from "./fields/SingleChoiceField";
import { MultiChoiceField } from "./fields/MultiChoiceField";
import { RatingField } from "./fields/RatingField";

// Partial for now; repeatable_group + link_check added in Task 8.
export const fieldRegistry: Partial<Record<FieldType, ComponentType<FieldProps<any>>>> = {
  text: TextField,
  textarea: TextAreaField,
  single_choice: SingleChoiceField,
  multi_choice: MultiChoiceField,
  rating: RatingField,
};
