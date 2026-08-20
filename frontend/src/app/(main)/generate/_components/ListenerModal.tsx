"use client";

import { useEffect, useMemo } from "react";
import { z } from "zod";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { FieldError } from "@/components/ui/field-error";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Dialog, DialogContent, DialogHeader, DialogTitle, DialogFooter } from "@/components/ui/dialog";
import { useI18n } from "@/lib/i18n";
import { useForm } from "@/lib/hooks/useForm";

export type ListenerForm = {
  name: string;
  ltype: string;
  host: string;
  port: string;
  proto: string;
};

const emptyListenerForm = (): ListenerForm => ({
  name: "",
  ltype: "http",
  host: "",
  port: "8080",
  proto: "http",
});

export default function ListenerModal({
  show,
  initial,
  onSubmit,
  onClose,
}: {
  show: boolean;
  initial: ListenerForm;
  onSubmit: (form: ListenerForm) => void;
  onClose: () => void;
}) {
  const { t } = useI18n();

  const schema = useMemo(
    () =>
      z.object({
        name: z.string().trim().min(1, t("generate.toast.listener_name_required")),
        ltype: z.string().min(1),
        host: z.string().trim().min(1, t("generate.toast.host_required")),
        port: z
          .string()
          .trim()
          .min(1, t("generate.toast.port_required"))
          .refine(
            (v) => /^\d+$/.test(v) && Number(v) >= 1 && Number(v) <= 65535,
            t("listeners.port_invalid"),
          ),
        proto: z.string(),
      }),
    [t],
  );

  const {
    values,
    errors,
    touched,
    isSubmitting,
    isValid,
    handleChange,
    handleBlur,
    setFieldValue,
    handleSubmit,
    resetForm,
  } = useForm<ListenerForm>({
    initialValues: initial,
    schema,
    onSubmit: async (vals) => {
      onSubmit(vals);
    },
  });

  useEffect(() => {
    if (!show) {
      resetForm(emptyListenerForm());
    }
  }, [show, resetForm]);

  const onLTypeChange = (val: string | null) => {
    const v = val ?? "http";
    setFieldValue("ltype", v);
  };

  return (
    <Dialog open={show} onOpenChange={onClose}>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>{t("generate.listener_new")}</DialogTitle>
        </DialogHeader>
        <form onSubmit={handleSubmit} noValidate>
        <div className="space-y-3">
          <div>
            <span className="block text-xs font-semibold text-muted-foreground mb-1">{t("generate.listener_name_aria")}</span>
            <Input aria-label={t("generate.listener_name_aria")} name="input-0" autoFocus value={values.name} onChange={handleChange("name")} onBlur={handleBlur("name")}
              aria-invalid={!!(touched.name && errors.name)}
              aria-describedby={errors.name ? "gen-listener-name-error" : undefined} />
            {touched.name && <FieldError id="gen-listener-name-error">{errors.name}</FieldError>}
          </div>
          <div>
            <span className="block text-xs font-semibold text-muted-foreground mb-1">{t("generate.listener_type")}</span>
            <Select value={values.ltype} onValueChange={onLTypeChange}>
              <SelectTrigger className="w-full">
                <SelectValue />
              </SelectTrigger>
              <SelectContent>
                <SelectItem value="http">http</SelectItem>
                <SelectItem value="tcp">tcp</SelectItem>
                <SelectItem value="dns">dns</SelectItem>
              </SelectContent>
            </Select>
          </div>
          <div>
            <span className="block text-xs font-semibold text-muted-foreground mb-1">{t("generate.listener_domain")}</span>
            <Input aria-label={t("generate.listener_domain_aria")} name="input-2" value={values.host} onChange={handleChange("host")} onBlur={handleBlur("host")}
              aria-invalid={!!(touched.host && errors.host)}
              aria-describedby={errors.host ? "gen-listener-host-error" : undefined} />
            {touched.host && <FieldError id="gen-listener-host-error">{errors.host}</FieldError>}
          </div>
          <div>
            <span className="block text-xs font-semibold text-muted-foreground mb-1">{t("generate.listener_port")}</span>
            <Input aria-label={t("generate.listener_port_aria")} name="input-3" type="number" min="1" max="65535" value={values.port} onChange={handleChange("port")} onBlur={handleBlur("port")}
              aria-invalid={!!(touched.port && errors.port)}
              aria-describedby={errors.port ? "gen-listener-port-error" : undefined} />
            {touched.port && <FieldError id="gen-listener-port-error">{errors.port}</FieldError>}
          </div>
          {values.ltype !== "dns" && (
            <div>
              <span className="block text-xs font-semibold text-muted-foreground mb-1">{t("generate.listener_protocol")}</span>
              <Input aria-label={t("generate.listener_protocol_aria")} name="input-4" value={values.proto} onChange={handleChange("proto")} placeholder="http/https/tcp/tls" />
            </div>
          )}
        </div>
        <DialogFooter>
          <Button type="button" variant="outline" onClick={onClose}>{t("common.cancel")}</Button>
          <Button type="submit" disabled={isSubmitting || !isValid}>{t("common.confirm")}</Button>
        </DialogFooter>
        </form>
      </DialogContent>
    </Dialog>
  );
}