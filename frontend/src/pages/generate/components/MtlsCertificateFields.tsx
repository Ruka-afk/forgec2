import { KeyRound } from "lucide-react";
import { Textarea } from "@/components/ui/textarea";
import { useI18n } from "@/lib/i18n";

export interface MtlsMaterial {
  cert: string;
  key: string;
  ca: string;
}

interface MtlsCertificateFieldsProps {
  value: MtlsMaterial;
  onChange: (next: MtlsMaterial) => void;
}

/**
 * Client credentials for the mTLS beacon transport. The three PEM values are
 * embedded into the build (base64, inside the encrypted config blob) so the
 * agent can authenticate against a listener that requires a client
 * certificate; the CA bundle additionally pins the server chain. The mirror
 * of the server-side pair rule is surfaced inline so an incomplete pair is
 * caught before a request is sent.
 */
export function MtlsCertificateFields({ value, onChange }: MtlsCertificateFieldsProps) {
  const { t } = useI18n();
  const pairIncomplete = (value.cert.trim() !== "") !== (value.key.trim() !== "");

  return (
    <div className="space-y-2">
      <div className="flex items-center gap-1.5 text-xs font-semibold text-muted-foreground">
        <KeyRound className="size-3.5" />
        {t("generate.mtls_title")}
      </div>
      <p className="text-(--fs-micro-sm) text-muted-foreground">{t("generate.mtls_hint")}</p>
      <div>
        <span className="mb-1.5 block text-xs font-semibold text-muted-foreground">{t("generate.mtls_cert")}</span>
        <Textarea
          aria-label={t("generate.mtls_cert")}
          value={value.cert}
          onChange={(e) => onChange({ ...value, cert: e.target.value })}
          placeholder="-----BEGIN CERTIFICATE-----"
          className="min-h-20 font-mono text-xs"
        />
      </div>
      <div>
        <span className="mb-1.5 block text-xs font-semibold text-muted-foreground">{t("generate.mtls_key")}</span>
        <Textarea
          aria-label={t("generate.mtls_key")}
          value={value.key}
          onChange={(e) => onChange({ ...value, key: e.target.value })}
          placeholder="-----BEGIN EC PRIVATE KEY-----"
          className="min-h-20 font-mono text-xs"
        />
      </div>
      <div>
        <span className="mb-1.5 block text-xs font-semibold text-muted-foreground">{t("generate.mtls_ca")}</span>
        <Textarea
          aria-label={t("generate.mtls_ca")}
          value={value.ca}
          onChange={(e) => onChange({ ...value, ca: e.target.value })}
          placeholder="-----BEGIN CERTIFICATE-----"
          className="min-h-16 font-mono text-xs"
        />
      </div>
      {pairIncomplete ? (
        <p className="text-(--fs-micro-sm) text-warning">{t("generate.mtls_pair_required")}</p>
      ) : null}
      <p className="text-(--fs-micro-sm) text-muted-foreground">{t("generate.mtls_local_only")}</p>
    </div>
  );
}
