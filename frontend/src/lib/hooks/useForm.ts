"use client";

import { useState, useCallback, useMemo } from "react";
import type { ZodSchema, ZodError } from "zod";
import { useErrorToast } from "@/lib/hooks/useErrorToast";

interface UseFormOptions<T extends Record<string, unknown>> {
  initialValues: T;
  schema: ZodSchema<T>;
  onSubmit: (values: T) => Promise<void>;
}

interface UseFormReturn<T extends Record<string, unknown>> {
  values: T;
  errors: Partial<Record<keyof T, string>>;
  touched: Partial<Record<keyof T, boolean>>;
  isSubmitting: boolean;
  isValid: boolean;
  handleChange: (field: keyof T) => (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>) => void;
  handleBlur: (field: keyof T) => () => void;
  handleSubmit: (e: React.FormEvent) => Promise<void>;
  setFieldValue: (field: keyof T, value: unknown) => void;
  resetForm: (values?: T) => void;
}

export function useForm<T extends Record<string, unknown>>({
  initialValues: initial,
  schema,
  onSubmit,
}: UseFormOptions<T>): UseFormReturn<T> {
  const [values, setValues] = useState<T>(initial);
  const [errors, setErrors] = useState<Partial<Record<keyof T, string>>>({});
  const [touched, setTouched] = useState<Partial<Record<keyof T, boolean>>>({});
  const [isSubmitting, setIsSubmitting] = useState(false);
  const { toastError } = useErrorToast();

  const validate = useCallback(
    (vals: T): Partial<Record<keyof T, string>> => {
      const result = schema.safeParse(vals);
      if (result.success) return {};
      const fieldErrors: Partial<Record<keyof T, string>> = {};
      for (const issue of (result as { error: ZodError }).error.issues) {
        const path = issue.path[0] as keyof T;
        if (!fieldErrors[path]) fieldErrors[path] = issue.message;
      }
      return fieldErrors;
    },
    [schema],
  );

  const isValid = useMemo(() => Object.keys(validate(values)).length === 0, [validate, values]);

  const handleChange = useCallback(
    (field: keyof T) =>
      (e: React.ChangeEvent<HTMLInputElement | HTMLSelectElement | HTMLTextAreaElement>) => {
        const input = e.target;
        let newVal: unknown;
        if (input instanceof HTMLInputElement && input.type === "checkbox") {
          newVal = input.checked;
        } else if (input instanceof HTMLInputElement && input.type === "number") {
          newVal = input.value === "" ? "" : Number(input.value) || input.value;
        } else {
          newVal = input.value;
        }
        setValues((prev) => ({ ...prev, [field]: newVal }));
      },
    [],
  );

  const handleBlur = useCallback(
    (field: keyof T) => () => {
      setTouched((prev) => ({ ...prev, [field]: true }));
      const result = schema.safeParse(values);
      if (!result.success) {
        const issue = (result as { error: ZodError }).error.issues.find(
          (i) => i.path[0] === field,
        );
        setErrors((prev) => ({ ...prev, [field]: issue?.message }));
      } else {
        setErrors((prev) => ({ ...prev, [field]: undefined }));
      }
    },
    [schema, values],
  );

  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();
      const allErrors = validate(values);
      setErrors(allErrors);
      setTouched(
        Object.keys(values).reduce(
          (acc, key) => ({ ...acc, [key]: true }),
          {} as Partial<Record<keyof T, boolean>>,
        ),
      );
      if (Object.keys(allErrors).length > 0) return;
      setIsSubmitting(true);
      try {
        await onSubmit(values);
      } catch (err) {
        toastError(err, "common.submit_failed");
      } finally {
        setIsSubmitting(false);
      }
    },
    [validate, values, onSubmit, toastError],
  );

  const setFieldValue = useCallback((field: keyof T, value: unknown) => {
    setValues((prev) => ({ ...prev, [field]: value }));
  }, []);

  const resetForm = useCallback(
    (values?: T) => {
      setValues(values ?? initial);
      setErrors({});
      setTouched({});
    },
    [initial],
  );

  return {
    values,
    errors,
    touched,
    isSubmitting,
    isValid,
    handleChange,
    handleBlur,
    handleSubmit,
    setFieldValue,
    resetForm,
  };
}
