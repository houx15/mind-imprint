import type { ComponentType } from "react";
import type { FieldType } from "@mind-imprint/contracts";
import type { FieldProps } from "./fields/types";
import { TextField } from "./fields/TextField";
import { TextAreaField } from "./fields/TextAreaField";
import { SingleChoiceField } from "./fields/SingleChoiceField";
import { MultiChoiceField } from "./fields/MultiChoiceField";
import { RatingField } from "./fields/RatingField";
import { LinkCheckField } from "./fields/LinkCheckField";
import { RepeatableGroupField } from "./fields/RepeatableGroupField";

export const fieldRegistry: Record<FieldType, ComponentType<FieldProps<any>>> = {
  text: TextField,
  textarea: TextAreaField,
  single_choice: SingleChoiceField,
  multi_choice: MultiChoiceField,
  rating: RatingField,
  link_check: LinkCheckField,
  repeatable_group: RepeatableGroupField,
};
