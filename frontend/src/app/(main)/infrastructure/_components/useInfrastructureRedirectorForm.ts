import { useState, useCallback } from "react";

export function useInfrastructureRedirectorForm() {
  const [rdName, setRdName] = useState("");
  const [rdHost, setRdHost] = useState("");
  const [rdType, setRdType] = useState("nginx");
  const [rdSSHUser, setRdSSHUser] = useState("root");
  const [rdSSHPort, setRdSSHPort] = useState(22);
  const [rdSSHKey, setRdSSHKey] = useState("");
  const [rdSSHPassword, setRdSSHPassword] = useState("");
  const [rdConfig, setRdConfig] = useState("");
  const [rdGenerateHost, setRdGenerateHost] = useState("");
  const [rdGenerateDomain, setRdGenerateDomain] = useState("");
  const [rdGeneratePort, setRdGeneratePort] = useState(443);
  const [rdGenerateTLS, setRdGenerateTLS] = useState(true);
  const [rdGenerateWS, setRdGenerateWS] = useState(true);
  const [deploying, setDeploying] = useState(false);
  const [saving, setSaving] = useState(false);
  const [deleting, setDeleting] = useState<number | null>(null);
  const [testingSSH, setTestingSSH] = useState(false);
  const [generatingModal, setGeneratingModal] = useState(false);
  const [deployResult, setDeployResult] = useState<{ success: boolean; message?: string; stdout?: string; stderr?: string } | null>(null);
  const [backendURL, setBackendURL] = useState("");

  const resetForm = useCallback(() => {
    setRdName("");
    setRdHost("");
    setRdType("nginx");
    setRdSSHUser("root");
    setRdSSHPort(22);
    setRdSSHKey("");
    setRdSSHPassword("");
    setRdConfig("");
  }, []);

  const populateForEdit = useCallback((rd: {
    id: number; name: string; host: string; type: string;
    ssh_user: string; ssh_port: number; config: string;
  }) => {
    setRdName(rd.name);
    setRdHost(rd.host);
    setRdType(rd.type);
    setRdSSHUser(rd.ssh_user || "root");
    setRdSSHPort(rd.ssh_port || 22);
    setRdConfig(rd.config || "");
  }, []);

  return {
    rdName, setRdName,
    rdHost, setRdHost,
    rdType, setRdType,
    rdSSHUser, setRdSSHUser,
    rdSSHPort, setRdSSHPort,
    rdSSHKey, setRdSSHKey,
    rdSSHPassword, setRdSSHPassword,
    rdConfig, setRdConfig,
    rdGenerateHost, setRdGenerateHost,
    rdGenerateDomain, setRdGenerateDomain,
    rdGeneratePort, setRdGeneratePort,
    rdGenerateTLS, setRdGenerateTLS,
    rdGenerateWS, setRdGenerateWS,
    deploying, setDeploying,
    saving, setSaving,
    deleting, setDeleting,
    testingSSH, setTestingSSH,
    generatingModal, setGeneratingModal,
    deployResult, setDeployResult,
    backendURL, setBackendURL,
    resetForm, populateForEdit,
  };
}
