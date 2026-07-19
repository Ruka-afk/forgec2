#!/usr/bin/env python3
"""Defense Recon plugin — detects installed AV/EDR products on agent hosts."""

import sys
import os

sys.path.insert(0, os.path.join(os.path.dirname(__file__), "..", ".."))
from lib.db import Database, read_stdin, write_result

# Known AV/EDR product signatures
PRODUCT_SIGNATURES = {
    "Windows Defender": {
        "processes": ["MsMpEng.exe", "MpCmdRun.exe", "MsMpDlp.exe"],
        "services": ["WinDefend", "WdNisSvc", "mpssvc", "Sense"],
        "keywords": ["windows defender", "microsoft defender", "msmpeng"],
        "type": "AV",
    },
    "CrowdStrike": {
        "processes": ["csagent.exe", "CSFalconService.exe", "falconsensor.exe"],
        "services": ["CSFalconService", "csagent"],
        "keywords": ["crowdstrike", "csfalcon", "falcon sensor"],
        "type": "EDR",
    },
    "Carbon Black": {
        "processes": ["cb.exe", "cbdefense.exe", "RepWAV.exe", "RepUX.exe"],
        "services": ["cb", "CarbonBlack", "RepMgr", "RepWAV", "RepUX"],
        "keywords": ["carbon black", "carbonblack", "cb defense"],
        "type": "EDR",
    },
    "Symantec": {
        "processes": ["smc.exe", "smcgui.exe", "ccSvcHst.exe", "SepMasterService.exe"],
        "services": ["SepMasterService", "Symantec AntiVirus", "SNAC"],
        "keywords": ["symantec", "norton antivirus", "sep", "symantec endpoint"],
        "type": "AV",
    },
    "McAfee": {
        "processes": ["mcshield.exe", "mfetp.exe", "mcshield.exe", " McAfeeService.exe"],
        "services": ["McShield", "McAfeeFramework", "mfetp", "macmnsvc"],
        "keywords": ["mcafee", "mcafee endpoint", "mcafee threat"],
        "type": "AV",
    },
    "Kaspersky": {
        "processes": ["avp.exe", "avpui.exe", "klnagent.exe", "klnac.exe"],
        "services": ["AVP", "AVP1", "klnagent"],
        "keywords": ["kaspersky", "kaspersky endpoint", "klnagent"],
        "type": "AV",
    },
    "Sophos": {
        "processes": ["sophosui.exe", "sophosfs.exe", "sophosfilescanner.exe", "SophosAgent.exe"],
        "services": ["SophosAgent", "SophosClean", "SophosFileScanner", "SAVAdminService", "sophosfs"],
        "keywords": ["sophos", "sophos endpoint", "sophos protect"],
        "type": "EDR",
    },
    "SentinelOne": {
        "processes": ["SentinelServiceHost.exe", "SentinelAgent.exe", "SentinelStaticEngine.exe"],
        "services": ["SentinelAgent", "SentinelMonitor", "SentinelStaticEngine"],
        "keywords": ["sentinelone", "sentinel agent", "sentinelstaticengine"],
        "type": "EDR",
    },
    "Cylance": {
        "processes": ["cylance.exe", "cyltray.exe", "CylanceSvc.exe"],
        "services": ["CylanceSvc"],
        "keywords": ["cylance", "cylance protect"],
        "type": "EDR",
    },
    "ESET": {
        "processes": ["ekrn.exe", "egui.exe", "eset-service.exe"],
        "services": ["ekrn", "ESET Service", "ESETHTTP"],
        "keywords": ["eset", "eset nod32", "eset endpoint"],
        "type": "AV",
    },
    "Trend Micro": {
        "processes": ["ds_agent.exe", "dsa.exe", "coreServiceShell.exe", "Ntrtscan.exe"],
        "services": ["ds_agent", "ds_agent", "TMBMServer", "ntrtscan", "tmcomm"],
        "keywords": ["trend micro", "trend micro apex", "worry-free"],
        "type": "EDR",
    },
    "F-Secure": {
        "processes": ["fsav.exe", "fshoster.exe", "fsdevcon.exe"],
        "services": ["FSMA", "F-Secure Gatekeeper", "F-Secure Network Filter"],
        "keywords": ["f-secure", "fsecure", "f-secure client"],
        "type": "AV",
    },
    "Webroot": {
        "processes": ["wrsa.exe", "WRSA.exe", "webroot.exe"],
        "services": ["WRSA", "Webroot"],
        "keywords": ["webroot", "webroot secureanywhere"],
        "type": "AV",
    },
    "Malwarebytes": {
        "processes": ["MBAMService.exe", "mbamtray.exe", "Mbamscheduler.exe"],
        "services": ["MBAMService", "MBAMScheduler"],
        "keywords": ["malwarebytes", "malwarebytes endpoint"],
        "type": "AV",
    },
    "Bitdefender": {
        "processes": ["bdagent.exe", "bdredline.exe", "EPSecurityService.exe", "bdservicehost.exe"],
        "services": ["EPSecurityService", "BDAuxSrv", "BDAuxSrv", "bdredline"],
        "keywords": ["bitdefender", "bitdefender endpoint", "gravityzone"],
        "type": "AV",
    },
    "Norton": {
        "processes": ["n360.exe", "nsbu.exe", "symerr.exe", "ccSvcHst.exe"],
        "services": ["NAVEng", "ccEvtMgr", "ccSetMgr"],
        "keywords": ["norton", "norton 360", "norton security"],
        "type": "AV",
    },
    "AVG": {
        "processes": ["avgsvc.exe", "avgui.exe", "avgwd.exe"],
        "services": ["avgsvc", "avgwd", "AVG AntiVirus"],
        "keywords": ["avg", "avg antivirus"],
        "type": "AV",
    },
    "Avast": {
        "processes": ["AvastSvc.exe", "AvastUI.exe", "aswEngSrv.exe"],
        "services": ["AvastSvc", "AvastFirewall", "aswEngSrv"],
        "keywords": ["avast", "avast antivirus"],
        "type": "AV",
    },
}


def detect_defenses_for_agent(agent, tasks):
    """Analyze a single agent's task results for defense products."""
    detected = []
    methods = []

    # Collect text from all completed task results
    all_text = ""
    for task in tasks:
        if task.get("status") == "completed" and task.get("result"):
            all_text += " " + task["result"]
        if task.get("command"):
            all_text += " " + task["command"]

    all_text_lower = all_text.lower()

    for product_name, sigs in PRODUCT_SIGNATURES.items():
        found_via = []

        # Check process names
        for proc in sigs["processes"]:
            if proc.lower() in all_text_lower:
                found_via.append("process")
                break

        # Check service names
        for svc in sigs["services"]:
            if svc.lower() in all_text_lower:
                found_via.append("service")
                break

        # Check keyword matches
        for kw in sigs["keywords"]:
            if kw in all_text_lower:
                found_via.append("keyword")
                break

        if found_via:
            # Confidence based on how many detection methods matched
            num_methods = len(found_via)
            if num_methods >= 2:
                confidence = "high"
            elif num_methods == 1:
                confidence = "medium"
            else:
                confidence = "low"

            detected.append({
                "name": product_name,
                "type": sigs["type"],
                "confidence": confidence,
                "methods": found_via,
            })
            methods.extend(found_via)

    # Deduplicate methods
    methods = list(set(methods))
    return detected, methods


def main():
    data = read_stdin()
    params = data.get("params", {})
    agent_id = params.get("agent_id", "")

    try:
        db = Database()
    except FileNotFoundError as e:
        write_result(False, error=str(e))
        return

    try:
        agents = db.all_agents()
        all_tasks = db.all_tasks()

        # Filter to specific agent if provided
        if agent_id:
            agent = db.agent_by_id(agent_id)
            if not agent:
                write_result(False, error=f"Agent {agent_id} not found")
                return
            agents = [agent]

        # Group tasks by agent_id
        tasks_by_agent = {}
        for task in all_tasks:
            aid = task.get("agent_id", "")
            if aid not in tasks_by_agent:
                tasks_by_agent[aid] = []
            tasks_by_agent[aid].append(task)

        agent_results = []
        product_distribution = {}
        agents_with_defenses = 0

        for agent in agents:
            aid = agent.get("id", "")
            tasks = tasks_by_agent.get(aid, [])
            detected, methods = detect_defenses_for_agent(agent, tasks)

            has_defenses = len(detected) > 0
            if has_defenses:
                agents_with_defenses += 1

            for prod in detected:
                pname = prod["name"]
                product_distribution[pname] = product_distribution.get(pname, 0) + 1

            agent_results.append({
                "agent_id": aid,
                "hostname": agent.get("hostname", ""),
                "ip": agent.get("ip", ""),
                "status": agent.get("status", ""),
                "has_defenses": has_defenses,
                "detected_products": detected,
                "detection_methods": methods,
            })

        total_agents = len(agents)
        agents_without_defenses = total_agents - agents_with_defenses

        # Coverage risk assessment
        if total_agents > 0:
            pct = (agents_with_defenses / total_agents) * 100
            if pct > 80:
                coverage_risk = "high"
            elif pct >= 50:
                coverage_risk = "medium"
            else:
                coverage_risk = "low"
        else:
            coverage_risk = "low"

        output = (
            f"Scanned {total_agents} agents | "
            f"{agents_with_defenses} with defenses | "
            f"{agents_without_defenses} clean"
        )

        write_result(
            True,
            output=output,
            data={
                "total_agents": total_agents,
                "agents_with_defenses": agents_with_defenses,
                "agents_without_defenses": agents_without_defenses,
                "product_distribution": product_distribution,
                "coverage_risk": coverage_risk,
                "agents": agent_results,
            },
        )
    finally:
        db.close()


if __name__ == "__main__":
    main()
