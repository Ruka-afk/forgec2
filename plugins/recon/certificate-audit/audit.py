#!/usr/bin/env python3
import sys, os, re
from datetime import datetime, timedelta
sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result

# ---------------------------------------------------------------------------
# Patterns — certutil (Windows)
# ---------------------------------------------------------------------------

CERTUTIL_HEADER_RE = re.compile(
    r"^\s*=\=\=\=\s+Certificate\s+\d+\s+\s*=\=\=\=", re.IGNORECASE,
)
CERTUTIL_SUBJECT_RE = re.compile(
    r"^\s*Subject:\s+(.+)", re.IGNORECASE,
)
CERTUTIL_ISSUER_RE = re.compile(
    r"^\s*Issuer:\s+(.+)", re.IGNORECASE,
)
CERTUTIL_ALGO_RE = re.compile(
    r"^\s*(?:Signature Algorithm|Algorithm):\s+(\S+)", re.IGNORECASE,
)
CERTUTIL_NOT_AFTER_RE = re.compile(
    r"^\s*Not (?:after|valid after):\s+(.+)", re.IGNORECASE,
)
CERTUTIL_SELF_SIGNED_RE = re.compile(
    r"^\s*(?:Self signed|Self-signed|Signature matches public key)", re.IGNORECASE,
)

# ---------------------------------------------------------------------------
# Patterns — openssl x509
# ---------------------------------------------------------------------------

OPENSSL_SUBJECT_RE = re.compile(
    r"^\s*Subject:\s*[=:]\s*(.+)", re.IGNORECASE,
)
OPENSSL_ISSUER_RE = re.compile(
    r"^\s*Issuer:\s*[=:]\s*(.+)", re.IGNORECASE,
)
OPENSSL_ALGO_RE = re.compile(
    r"^\s*Signature Algorithm:\s+(\S+)", re.IGNORECASE,
)
OPENSSL_NOT_AFTER_RE = re.compile(
    r"^\s*Not After\s*:\s+(.+)", re.IGNORECASE,
)
OPENSSL_NOT_BEFORE_RE = re.compile(
    r"^\s*Not Before\s*:\s+(.+)", re.IGNORECASE,
)

# ---------------------------------------------------------------------------
# Patterns — keytool (Java)
# ---------------------------------------------------------------------------

KEYTOOL_ENTRY_RE = re.compile(
    r"^\s*(\S+),\s*\w{3}\s+\d{1,2},\s+\d{4}.*?privatekey.*$", re.IGNORECASE,
)
KEYTOOL_CERT_ENTRY_RE = re.compile(
    r"^\s*(\S+),\s*\w{3}\s+\d{1,2},\s+\d{4}.*?trustedCert.*$", re.IGNORECASE,
)
KEYTOOL_OWNER_RE = re.compile(
    r"^\s*Owner:\s+(.+)", re.IGNORECASE,
)
KEYTOOL_ISSUER_RE = re.compile(
    r"^\s*Issuer:\s+(.+)", re.IGNORECASE,
)

# ---------------------------------------------------------------------------
# Patterns — generic cert lines
# ---------------------------------------------------------------------------

GENERIC_SUBJECT_RE = re.compile(
    r"(?:subject|Subject|SUBJECT)\s*[=|:]\s*(.+)", re.IGNORECASE,
)
GENERIC_ISSUER_RE = re.compile(
    r"(?:issuer|Issuer|ISSUER)\s*[=|:]\s*(.+)", re.IGNORECASE,
)
GENERIC_ALGO_RE = re.compile(
    r"(?:algorithm|algo|key\s*type|Key\s*Algorithm|signatureAlgorithm)\s*[=|:]\s*(RSA|DSA|ECDSA|Ed25519|SHA\d+with\s*RSA|sha\d+WithRSAEncryption|md5WithRSAEncryption|SHA1withRSA|SHA-?1|MD5)",
    re.IGNORECASE,
)
GENERIC_EXPIRY_RE = re.compile(
    r"(?:expires|expiry|notAfter|not.after|valid.until|end.date)\s*[=|:]\s*(\d{4}[-/]\d{2}[-/]\d{2}[\sT]\d{2}:\d{2})", re.IGNORECASE,
)
GENERIC_SELF_SIGNED_RE = re.compile(
    r"(?:self.signed|selfSigned|SELF_SIGNED|isCA|basicConstraints.*CA.*TRUE)", re.IGNORECASE,
)

# ---------------------------------------------------------------------------
# Internal CA indicators
# ---------------------------------------------------------------------------

INTERNAL_CA_NAMES = re.compile(
    r"(?:Microsoft|Active Directory|ADCS|AD CS|Enterprise|SubCA|Root CA|"
    r"Issuing CA|Offline|Policy CA|Issuance CA|Code Signing|Domain Controller|"
    r"Web Server|Computer|User|CA[\s:]*\w+|Internal|Corp|PrivPKI|Private|"
    r"Windows Server.*Auth|Kerberos|Smartcard)", re.IGNORECASE,
)

# ---------------------------------------------------------------------------
# File paths
# ---------------------------------------------------------------------------

CERT_FILE_RE = re.compile(
    r"(?:^|[/\\])(?:[\w.-]*\.pem|[\w.-]*\.crt|[\w.-]*\.cer|[\w.-]*\.der|"
    r"[\w.-]*\.p7b|[\w.-]*\.p12|[\w.-]*\.pfx|[\w.-]*\.jks|[\w.-]*\.keystore|"
    r"ca-bundle\.txt|cert(?:s|ificates)?\.txt)$",
    re.IGNORECASE,
)

# ---------------------------------------------------------------------------
# Date parsing
# ---------------------------------------------------------------------------

DATE_FORMATS = [
    "%Y-%m-%dT%H:%M:%S",
    "%Y-%m-%dT%H:%M:%SZ",
    "%Y-%m-%d %H:%M:%S",
    "%Y-%m-%d %H:%M:%S %Z",
    "%b %d %H:%M:%S %Y %Z",
    "%b  %d %H:%M:%S %Y %Z",
    "%d %b %Y %H:%M:%S %Z",
    "%d %b %Y %H:%M:%S",
    "%m/%d/%Y %H:%M:%S",
    "%m/%d/%Y",
]


def _parse_date(text: str):
    text = text.strip().replace("GMT", "UTC")
    for fmt in DATE_FORMATS:
        try:
            return datetime.strptime(text, fmt)
        except ValueError:
            continue
    return None


def _is_internal_ca(subject: str, issuer: str) -> bool:
    combined = subject + " " + issuer
    if INTERNAL_CA_NAMES.search(combined):
        return True
    if re.search(r"\.corp\b|\.internal\b|\.ad\b|\.local\b", combined, re.IGNORECASE):
        return True
    if subject.strip() == issuer.strip():
        return True
    return False


def _is_weak_algorithm(algo: str) -> bool:
    algo_lower = algo.lower()
    if "sha-1" in algo_lower or "sha1" in algo_lower:
        return True
    if "md5" in algo_lower:
        return True
    if re.search(r"rsa(\s*<\s*)?(\d+)", algo_lower):
        bits = re.search(r"rsa(\s*<\s*)?(\d+)", algo_lower)
        if bits and int(bits.group(2)) < 2048:
            return True
    return False


def _classify_cert_type(subject: str, issuer: str, store: str = "") -> str:
    subject_lower = subject.lower()
    store_lower = store.lower()
    if "root" in store_lower or "trusted" in store_lower:
        return "root_ca"
    if "ca" in subject_lower or "certificate authority" in subject_lower:
        return "ca"
    if subject.strip() == issuer.strip():
        return "self_signed"
    if "personal" in store_lower or "my" in store_lower or "user" in store_lower:
        return "personal"
    return "intermediate"


# ---------------------------------------------------------------------------
# Parse certutil output
# ---------------------------------------------------------------------------

def _parse_certutil_store(text: str) -> list[dict]:
    certs = []
    current = {}
    for line in text.splitlines():
        line_s = line.strip()
        if CERTUTIL_HEADER_RE.match(line_s):
            if current.get("subject"):
                certs.append(current)
            current = {
                "subject": "", "issuer": "", "algorithm": "",
                "expiry": "", "self_signed": False, "store": "certutil",
            }
            continue
        m = CERTUTIL_SUBJECT_RE.match(line_s)
        if m:
            current["subject"] = m.group(1).strip()
            continue
        m = CERTUTIL_ISSUER_RE.match(line_s)
        if m:
            current["issuer"] = m.group(1).strip()
            continue
        m = CERTUTIL_ALGO_RE.match(line_s)
        if m:
            current["algorithm"] = m.group(1).strip()
            continue
        m = CERTUTIL_NOT_AFTER_RE.match(line_s)
        if m:
            current["expiry"] = m.group(1).strip()
            continue
        if CERTUTIL_SELF_SIGNED_RE.match(line_s):
            current["self_signed"] = True
    if current.get("subject"):
        certs.append(current)
    return certs


# ---------------------------------------------------------------------------
# Parse openssl output
# ---------------------------------------------------------------------------

def _parse_openssl_output(text: str) -> list[dict]:
    certs = []
    current = {}
    for line in text.splitlines():
        line_s = line.strip()
        m = OPENSSL_SUBJECT_RE.match(line_s)
        if m:
            if current.get("subject"):
                certs.append(current)
            current = {
                "subject": m.group(1).strip(), "issuer": "", "algorithm": "",
                "expiry": "", "self_signed": False, "store": "openssl",
            }
            continue
        m = OPENSSL_ISSUER_RE.match(line_s)
        if m and current:
            current["issuer"] = m.group(1).strip()
            continue
        m = OPENSSL_ALGO_RE.match(line_s)
        if m and current:
            current["algorithm"] = m.group(1).strip()
            continue
        m = OPENSSL_NOT_AFTER_RE.match(line_s)
        if m and current:
            current["expiry"] = m.group(1).strip()
            continue
    if current.get("subject"):
        certs.append(current)
    return certs


# ---------------------------------------------------------------------------
# Parse keytool output
# ---------------------------------------------------------------------------

def _parse_keytool_output(text: str) -> list[dict]:
    certs = []
    current = {}
    for line in text.splitlines():
        line_s = line.strip()
        m = KEYTOOL_ENTRY_RE.match(line_s) or KEYTOOL_CERT_ENTRY_RE.match(line_s)
        if m:
            if current.get("subject"):
                certs.append(current)
            current = {
                "subject": "", "issuer": "", "algorithm": "",
                "expiry": "", "self_signed": False, "store": "keytool",
            }
            continue
        m = KEYTOOL_OWNER_RE.match(line_s)
        if m:
            current["subject"] = m.group(1).strip()
            continue
        m = KEYTOOL_ISSUER_RE.match(line_s)
        if m:
            current["issuer"] = m.group(1).strip()
    if current.get("subject"):
        certs.append(current)
    return certs


# ---------------------------------------------------------------------------
# Scan task output for certificate data
# ---------------------------------------------------------------------------

def _scan_task_output(task) -> str:
    output = task.get("output", "") or ""
    if not output:
        result_data = task.get("result", "") or ""
        if isinstance(result_data, str):
            output = result_data
    return output if isinstance(output, str) else str(output)


def _parse_generic_certs(text: str) -> list[dict]:
    certs = []
    for block in re.split(r"\n\s*\n", text):
        subj_m = GENERIC_SUBJECT_RE.search(block)
        if not subj_m:
            continue
        issuer_m = GENERIC_ISSUER_RE.search(block)
        algo_m = GENERIC_ALGO_RE.search(block)
        expiry_m = GENERIC_EXPIRY_RE.search(block)
        self_m = GENERIC_SELF_SIGNED_RE.search(block)
        subject = subj_m.group(1).strip() if subj_m else ""
        issuer = issuer_m.group(1).strip() if issuer_m else ""
        algo = algo_m.group(1).strip() if algo_m else ""
        expiry = expiry_m.group(1).strip() if expiry_m else ""
        self_signed = bool(self_m)
        if subject:
            certs.append({
                "subject": subject,
                "issuer": issuer,
                "algorithm": algo,
                "expiry": expiry,
                "self_signed": self_signed,
                "store": "generic",
            })
    return certs


def _extract_certs_from_agent(db, agent) -> list[dict]:
    agent_id = agent.get("id", "")
    tasks = db.tasks_for_agent(agent_id)
    all_certs = []

    for task in tasks:
        raw = _scan_task_output(task)
        if not raw:
            continue
        cmd_lower = (task.get("command", "") or "").lower()

        if "certutil" in cmd_lower:
            all_certs.extend(_parse_certutil_store(raw))
        elif "openssl" in cmd_lower:
            all_certs.extend(_parse_openssl_output(raw))
        elif "keytool" in cmd_lower:
            all_certs.extend(_parse_keytool_output(raw))
        else:
            all_certs.extend(_parse_generic_certs(raw))

    return all_certs


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def run():
    try:
        db = Database()
    except FileNotFoundError as e:
        write_result(False, error=str(e))
        return

    try:
        params = read_stdin()
        agent_filter = params.get("agent_id") or params.get("params", {}).get("agent_id", "")
        agents = db.all_agents()
        if agent_filter:
            agents = [a for a in agents if a["id"] == agent_filter or a["id"][:8] == agent_filter]

        if not agents:
            write_result(True, output="No agents found", data={"agents": [], "summary": {}})
            return

        result_agents = []
        total_certs = 0
        expiring_30 = 0
        expiring_60 = 0
        expiring_90 = 0
        weak_algo_count = 0
        internal_ca_count = 0
        self_signed_count = 0
        root_ca_count = 0
        personal_cert_count = 0

        now = datetime.utcnow()
        threshold_30 = now + timedelta(days=30)
        threshold_60 = now + timedelta(days=60)
        threshold_90 = now + timedelta(days=90)

        for agent in agents:
            hostname = agent.get("hostname", "unknown")
            raw_certs = _extract_certs_from_agent(db, agent)
            agent_certs = []
            agent_has_internal_ca = False

            seen = set()
            for c in raw_certs:
                dedup_key = (c.get("subject", ""), c.get("issuer", ""), c.get("expiry", ""))
                if dedup_key in seen:
                    continue
                seen.add(dedup_key)

                subject = c.get("subject", "")
                issuer = c.get("issuer", "")
                algorithm = c.get("algorithm", "")
                expiry_str = c.get("expiry", "")
                self_signed = c.get("self_signed", False)

                if self_signed and subject.strip() == issuer.strip():
                    self_signed = True

                is_internal = _is_internal_ca(subject, issuer)
                if is_internal:
                    agent_has_internal_ca = True
                    internal_ca_count += 1

                cert_type = _classify_cert_type(subject, issuer, c.get("store", ""))
                if cert_type == "root_ca":
                    root_ca_count += 1
                elif cert_type == "personal":
                    personal_cert_count += 1
                if cert_type == "self_signed":
                    self_signed_count += 1
                elif self_signed:
                    self_signed_count += 1

                weak = _is_weak_algorithm(algorithm) if algorithm else False
                if weak:
                    weak_algo_count += 1

                expiry_dt = _parse_date(expiry_str) if expiry_str else None
                expiry_status = "valid"
                if expiry_dt:
                    if expiry_dt < now:
                        expiry_status = "expired"
                    elif expiry_dt <= threshold_30:
                        expiry_status = "expiring_30d"
                        expiring_30 += 1
                    elif expiry_dt <= threshold_60:
                        expiry_status = "expiring_60d"
                        expiring_60 += 1
                    elif expiry_dt <= threshold_90:
                        expiry_status = "expiring_90d"
                        expiring_90 += 1

                cert_entry = {
                    "subject": subject,
                    "issuer": issuer,
                    "algorithm": algorithm,
                    "expiry": expiry_str,
                    "expiry_status": expiry_status,
                    "self_signed": self_signed,
                    "internal_ca": is_internal,
                    "cert_type": cert_type,
                    "weak_algorithm": weak,
                    "chain_complete": not (self_signed and subject.strip() != issuer.strip()),
                }
                agent_certs.append(cert_entry)

            total_certs += len(agent_certs)
            result_agents.append({
                "id": agent.get("id", ""),
                "hostname": hostname,
                "certificates": agent_certs,
                "internal_ca": agent_has_internal_ca,
            })

        total_agents = len(result_agents)
        expiring_soon = expiring_30 + expiring_60 + expiring_90

        summary = {
            "total_agents": total_agents,
            "total_certs": total_certs,
            "expiring_soon": expiring_soon,
            "expiring_30d": expiring_30,
            "expiring_60d": expiring_60,
            "expiring_90d": expiring_90,
            "weak_algo": weak_algo_count,
            "internal_ca_count": internal_ca_count,
            "self_signed": self_signed_count,
            "root_cas": root_ca_count,
            "personal_certs": personal_cert_count,
        }

        output = (
            f"Audited {total_agents} agents | "
            f"{total_certs} certificates | "
            f"{expiring_soon} expiring soon | "
            f"{weak_algo_count} weak algorithms"
        )

        data = {"agents": result_agents, "summary": summary}
        write_result(True, output=output, data=data)
    finally:
        db.close()


if __name__ == "__main__":
    run()
