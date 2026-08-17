"use client";
import { PageContainer } from "@/components/ui/page-container";

import { useState } from "react";
import { api } from "@/lib/api";
import { downloadBase64 } from "@/lib/download";
import { Spinner } from "@/components/ui/spinner";
import { useI18n } from "@/lib/i18n";
import { useApiResource } from "@/lib/hooks/useApiResource";
import { Banner } from "@/components/ui/banner";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { IconBadge } from "@/components/ui/icon-badge";
import { Input } from "@/components/ui/input";
import { Label } from "@/components/ui/label";
import { Select, SelectContent, SelectItem, SelectTrigger, SelectValue } from "@/components/ui/select";
import { Tabs, TabsList, TabsTrigger, TabsContent } from "@/components/ui/tabs";
import { Box, CheckCircle, Download, Hammer, Wrench, X } from "lucide-react";

interface ArtifactTemplate {
  name: string;
  description: string;
  output_type: string;
  pe_sections: { text: string; data: string; rdata: string; reloc: string };
  entry_point_technique: string;
  timestamp: string;
  cert_option: string;
  import_manipulation: boolean;
  benign_imports: string[];
  shellcode_encode: string;
}

interface PackerInfo {
  encode_types: string[];
  entry_points: string[];
  timestamps: string[];
  cert_options: string[];
  output_types: string[];
}

export default function PackerPageContent({ embedded = false }: { embedded?: boolean }) {
  const { t } = useI18n();
  const [selectedTemplate, setSelectedTemplate] = useState("");
  const [outputType, setOutputType] = useState("exe");
  const [shellcodeB64, setShellcodeB64] = useState("");
  const [encodeType, setEncodeType] = useState("none");
  const [entryPoint, setEntryPoint] = useState("direct");
  const [timestamp, setTimestamp] = useState("random");
  const [timestampDate, setTimestampDate] = useState("");
  const [certOption, setCertOption] = useState("self_signed");
  const [importDLLs, setImportDLLs] = useState("");
  const [peText, setPeText] = useState(".text");
  const [peData, setPeData] = useState(".data");
  const [peRdata, setPeRdata] = useState(".rdata");
  const [peReloc, setPeReloc] = useState(".reloc");

  const [exeB64, setExeB64] = useState("");
  const [iconB64, setIconB64] = useState("");
  const [companyName, setCompanyName] = useState("");
  const [fileVersion, setFileVersion] = useState("");
  const [productName, setProductName] = useState("");
  const [fileDescription, setFileDescription] = useState("");
  const [originalFilename, setOriginalFilename] = useState("");
  const [bundleTimestamp, setBundleTimestamp] = useState("");
  const [bundleTimestampDate, setBundleTimestampDate] = useState("");
  const [bundleEntryPoint, setBundleEntryPoint] = useState("");

  const [resultB64, setResultB64] = useState("");
  const [resultSize, setResultSize] = useState(0);
  const [resultFilename, setResultFilename] = useState("");
  const [loading, setLoading] = useState(false);
  const [message, setMessage] = useState("");

  const { data } = useApiResource<{ templates: ArtifactTemplate[]; info: PackerInfo }>({
    fetcher: async () => {
      const [templates, info] = await Promise.all([
        api.get<{ templates: ArtifactTemplate[] }>("/packer/templates"),
        api.get<PackerInfo>("/packer/info"),
      ]);
      return { templates: templates.templates || [], info };
    },
    toastThrottleMs: 10_000,
    errorMessage: t("packer.toast.load_failed"),
  });
  const templates = data?.templates ?? [];
  const packerInfo = data?.info ?? null;

  function handleTemplateSelect(name: string) {
    setSelectedTemplate(name);
    const t = templates.find(t => t.name === name);
    if (t) {
      setOutputType(t.output_type);
      setEntryPoint(t.entry_point_technique);
      setTimestamp(t.timestamp);
      setCertOption(t.cert_option);
      setEncodeType(t.shellcode_encode);
      setPeText(t.pe_sections.text);
      setPeData(t.pe_sections.data);
      setPeRdata(t.pe_sections.rdata);
      setPeReloc(t.pe_sections.reloc);
      setImportDLLs(t.benign_imports.join(", "));
    }
  }

  function handleShellcodeFile(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
      const b64 = (reader.result as string).split(",")[1];
      setShellcodeB64(b64);
    };
    reader.readAsDataURL(file);
  }

  async function handleBuildArtifact() {
    setLoading(true); setMessage("");
    try {
      const res = await api.postJson<{ data: string; filename: string; size: number }>("/packer/artifact", {
        template_name: selectedTemplate,
        output_type: outputType,
        shellcode_b64: shellcodeB64,
        encode_type: encodeType,
        pe_section_text: peText,
        pe_section_data: peData,
        pe_section_rdata: peRdata,
        pe_section_reloc: peReloc,
        entry_point: entryPoint,
        timestamp: timestamp,
        timestamp_date: timestampDate,
        cert_option: certOption,
        import_dlls: importDLLs,
      });
      setResultB64(res.data);
      setResultSize(res.size);
      setResultFilename(res.filename);
      setMessage("Artifact built successfully!");
    } catch (e: unknown) {
      setMessage("Failed: " + (e instanceof Error ? e.message : "Unknown error"));
    } finally {
      setLoading(false);
    }
  }

  function handleFileSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
      const b64 = (reader.result as string).split(",")[1];
      setExeB64(b64);
    };
    reader.readAsDataURL(file);
  }

  function handleIconSelect(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    if (!file) return;
    const reader = new FileReader();
    reader.onload = () => {
      const b64 = (reader.result as string).split(",")[1];
      setIconB64(b64);
    };
    reader.readAsDataURL(file);
  }

  async function handleBundle() {
    if (!exeB64) { setMessage("Select an EXE file first"); return; }
    setLoading(true); setMessage("");
    try {
      const res = await api.postJson<{ data: string; size: number }>("/payload/bundle", {
        agent_exe: exeB64,
        icon_data: iconB64,
        version_info: {
          company_name: companyName,
          file_version: fileVersion,
          product_name: productName,
          file_description: fileDescription,
          original_filename: originalFilename,
        },
        pe_sections: {
          text: peText,
          data: peData,
          rdata: peRdata,
          reloc: peReloc,
        },
        entry_point: bundleEntryPoint || undefined,
        timestamp: bundleTimestamp || undefined,
        timestamp_date: bundleTimestampDate || undefined,
        cert_option: certOption,
        import_dlls: importDLLs,
      });
      setResultB64(res.data);
      setResultSize(res.size);
      setResultFilename(originalFilename || "bundled.exe");
      setMessage("Bundle created successfully!");
    } catch (e: unknown) {
      setMessage("Failed: " + (e instanceof Error ? e.message : "Unknown error"));
    } finally {
      setLoading(false);
    }
  }

  function handleDownload() {
    if (!resultB64) return;
    downloadBase64(resultB64, resultFilename || "artifact.bin");
  }

  return (
      <PageContainer embedded={embedded} title={!embedded ? t("packer.title") : undefined} subtitle={!embedded ? t("packer.subtitle") : undefined}>

      <Tabs defaultValue="artifact">
        <TabsList className="mb-6">
          <TabsTrigger value="artifact" className="gap-2">
            <Wrench className="w-4 h-4" />{t("packer.tab_artifact")}
          </TabsTrigger>
          <TabsTrigger value="bundle" className="gap-2">
            <Box className="w-4 h-4" />{t("packer.tab_bundle")}
          </TabsTrigger>
        </TabsList>

      {message && (
        <Banner tone={message.startsWith("Failed") ? "destructive" : "success"} className="mb-4" action={<Button variant="ghost" size="icon-sm" onClick={() => setMessage("")} className="opacity-60 hover:opacity-100" aria-label={t("common.dismiss")}><X className="w-4 h-4" /></Button>}>
          {message}
        </Banner>
      )}

      <TabsContent value="artifact">
        <Card className="p-4 sm:p-5 space-y-5">

          {templates.length > 0 && (
            <div>
              <Label className="text-xs mb-1.5">{t("packer.template")}</Label>
              <Select value={selectedTemplate} onValueChange={(v) => handleTemplateSelect(v ?? "")}>
                <SelectTrigger className="w-full">
                  <SelectValue placeholder={t("packer.custom")} />
                </SelectTrigger>
                <SelectContent>
                  <SelectItem value="">{t("packer.custom")}</SelectItem>
                  {templates.map(t => (
                    <SelectItem key={t.name} value={t.name}>{t.name} — {t.description}</SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>
          )}

          <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
            <div>
              <Label className="text-xs">{t("packer.output_type")}</Label>
              <Select value={outputType} onValueChange={(v) => setOutputType(v ?? "exe")}>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {packerInfo?.output_types?.map(t => <SelectItem key={t} value={t}>{t}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label className="text-xs">{t("packer.entry_point")}</Label>
              <Select value={entryPoint} onValueChange={(v) => setEntryPoint(v ?? "direct")}>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {packerInfo?.entry_points?.map(ep => <SelectItem key={ep} value={ep}>{ep}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
            <div>
              <Label className="text-xs">{t("packer.timestamp")}</Label>
              <Select value={timestamp} onValueChange={(v) => setTimestamp(v ?? "random")}>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {packerInfo?.timestamps?.map(ts => <SelectItem key={ts} value={ts}>{ts}</SelectItem>)}
                </SelectContent>
              </Select>
              {timestamp === "custom" && (
                <Input type="date" value={timestampDate} onChange={e => setTimestampDate(e.target.value)}
                  className="mt-1" />
              )}
            </div>
            <div>
              <Label className="text-xs">{t("packer.certificate")}</Label>
              <Select value={certOption} onValueChange={(v) => setCertOption(v ?? "self_signed")}>
                <SelectTrigger className="w-full">
                  <SelectValue />
                </SelectTrigger>
                <SelectContent>
                  {packerInfo?.cert_options?.map(co => <SelectItem key={co} value={co}>{co}</SelectItem>)}
                </SelectContent>
              </Select>
            </div>
          </div>

          <div className="border-t border-border pt-4">
            <h3 className="text-sm font-semibold text-foreground mb-3">{t("packer.pe_sections")}</h3>
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
              <div>
                <Label className="text-xs">.text</Label>
                <Input value={peText} onChange={e => setPeText(e.target.value)} />
              </div>
              <div>
                <Label className="text-xs">.data</Label>
                <Input value={peData} onChange={e => setPeData(e.target.value)} />
              </div>
              <div>
                <Label className="text-xs">.rdata</Label>
                <Input value={peRdata} onChange={e => setPeRdata(e.target.value)} />
              </div>
              <div>
                <Label className="text-xs">.reloc</Label>
                <Input value={peReloc} onChange={e => setPeReloc(e.target.value)} />
              </div>
            </div>
          </div>

          <div className="border-t border-border pt-4">
            <h3 className="text-sm font-semibold text-foreground mb-3">{t("packer.shellcode_encoding")}</h3>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <Label className="text-xs">{t("packer.encode_type")}</Label>
                <Select value={encodeType} onValueChange={(v) => setEncodeType(v ?? "none")}>
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    {packerInfo?.encode_types?.map(et => <SelectItem key={et} value={et}>{et}</SelectItem>)}
                  </SelectContent>
                </Select>
              </div>
              <div>
                <Label className="text-xs">{t("packer.shellcode_file")}</Label>
                <Input type="file" accept=".bin" onChange={handleShellcodeFile} aria-label={t("packer.upload_bof")} />
                {shellcodeB64 && <span className="text-xs text-success mt-1 flex items-center gap-1"><CheckCircle className="w-4 h-4" />{t("packer.shellcode_loaded")}</span>}
              </div>
            </div>
          </div>

          <div className="border-t border-border pt-4">
            <h3 className="text-sm font-semibold text-foreground mb-3">{t("packer.import_manipulation")}</h3>
            <div>
              <Label className="text-xs">{t("packer.benign_dlls")}</Label>
              <Input value={importDLLs} onChange={e => setImportDLLs(e.target.value)}
                placeholder="kernel32.dll, advapi32.dll, ws2_32.dll" />
            </div>
          </div>

          <Button onClick={handleBuildArtifact} disabled={loading}
            className="w-full">
            {loading ? <><Spinner size="xs" /> {t("packer.building")}</> : <><Hammer className="w-4 h-4" /> {t("packer.build_artifact")}</>}
          </Button>
        </Card>
      </TabsContent>

      <TabsContent value="bundle">
        <Card className="p-4 sm:p-5 space-y-5">
          <div>
            <Label className="text-xs mb-1.5">{t("packer.payload_exe")}</Label>
            <Input type="file" accept=".exe,.dll" onChange={handleFileSelect} aria-label={t("packer.upload_dll")} />
            {exeB64 && <span className="text-xs text-success mt-1 flex items-center gap-1"><CheckCircle className="w-4 h-4" />{t("packer.exe_loaded", { size: String(Math.round(exeB64.length * 0.75 / 1024)) })}</span>}
          </div>

          <div className="border-t border-border pt-4">
            <h3 className="text-sm font-semibold text-foreground mb-3">{t("packer.icon")} <span className="text-muted-foreground font-normal">{t("packer.optional")}</span></h3>
            <Input type="file" accept=".ico" onChange={handleIconSelect} aria-label={t("packer.upload_icon")} />
            {iconB64 && <span className="text-xs text-success mt-1 flex items-center gap-1"><CheckCircle className="w-4 h-4" />{t("packer.icon_loaded")}</span>}
          </div>

          <div className="border-t border-border pt-4">
            <h3 className="text-sm font-semibold text-foreground mb-3">{t("packer.pe_transformations")}</h3>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
              <Label className="text-xs">{t("packer.timestamp")}</Label>
                <Select value={bundleTimestamp} onValueChange={(v) => setBundleTimestamp(v ?? "")}>
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="">{t("packer.keep_original")}</SelectItem>
                    <SelectItem value="random">{t("packer.random")}</SelectItem>
                    <SelectItem value="custom">{t("packer.custom_val")}</SelectItem>
                  </SelectContent>
                </Select>
                {bundleTimestamp === "custom" && (
                  <Input type="date" value={bundleTimestampDate} onChange={e => setBundleTimestampDate(e.target.value)} className="mt-1" />
                )}
              </div>
              <div>
                <Label className="text-xs">{t("packer.entry_point")}</Label>
                <Select value={bundleEntryPoint} onValueChange={(v) => setBundleEntryPoint(v ?? "")}>
                  <SelectTrigger className="w-full">
                    <SelectValue />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="">{t("packer.keep_original")}</SelectItem>
                    <SelectItem value="direct">{t("packer.direct")}</SelectItem>
                    <SelectItem value="call">{t("packer.call")}</SelectItem>
                    <SelectItem value="callback">{t("packer.callback")}</SelectItem>
                    <SelectItem value="tls">{t("packer.tls")}</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
            <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 mt-3">
              <div>
                <Label className="text-xs">.text</Label>
                <Input value={peText} onChange={e => setPeText(e.target.value)} />
              </div>
              <div>
                <Label className="text-xs">.data</Label>
                <Input value={peData} onChange={e => setPeData(e.target.value)} />
              </div>
              <div>
                <Label className="text-xs">.rdata</Label>
                <Input value={peRdata} onChange={e => setPeRdata(e.target.value)} />
              </div>
              <div>
                <Label className="text-xs">.reloc</Label>
                <Input value={peReloc} onChange={e => setPeReloc(e.target.value)} />
              </div>
            </div>
          </div>

          <div className="border-t border-border pt-4">
            <h3 className="text-sm font-semibold text-foreground mb-3">{t("packer.version_info")} <span className="text-muted-foreground font-normal">{t("packer.optional")}</span></h3>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <div>
                <Label className="text-xs">{t("packer.company_name")}</Label>
                <Input value={companyName} onChange={e => setCompanyName(e.target.value)} />
              </div>
              <div>
                <Label className="text-xs">{t("packer.file_version")}</Label>
                <Input value={fileVersion} onChange={e => setFileVersion(e.target.value)} placeholder="1.0.0.0" />
              </div>
              <div>
                <Label className="text-xs">{t("packer.product_name")}</Label>
                <Input value={productName} onChange={e => setProductName(e.target.value)} />
              </div>
              <div>
                <Label className="text-xs">{t("packer.file_description")}</Label>
                <Input value={fileDescription} onChange={e => setFileDescription(e.target.value)} />
              </div>
              <div className="sm:col-span-2">
                <Label className="text-xs">{t("packer.original_filename")}</Label>
                <Input value={originalFilename} onChange={e => setOriginalFilename(e.target.value)} placeholder="bundled.exe" />
              </div>
            </div>
          </div>

          <Button onClick={handleBundle} disabled={loading}
            className="w-full">
            {loading ? <><Spinner size="xs" /> {t("packer.bundling")}</> : <><Box className="w-4 h-4" /> {t("packer.bundle_payload")}</>}
          </Button>
        </Card>
      </TabsContent>
      </Tabs>

      {resultB64 && (
        <Card className="mt-4 p-4 border-success/30 bg-success/5">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <IconBadge icon={CheckCircle} color="success" size="xl" />
              <div>
                <p className="text-sm font-medium text-success">{t("packer.ready")}</p>
                <p className="text-xs text-muted-foreground">{(resultSize / 1024).toFixed(1)} KB — {resultFilename}</p>
              </div>
            </div>
            <Button onClick={handleDownload}>
              <Download className="w-4 h-4" /> {t("packer.download")}
            </Button>
          </div>
        </Card>
      )}
    </PageContainer>
  );
}
