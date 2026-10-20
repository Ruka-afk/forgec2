package server

import (
	"encoding/json"

	"github.com/forgec2/forgec2/pkg/protocol"
)

const fileScanRedactionNote = "Sensitive values are available only in the encrypted task result. AI, callbacks and exports receive redacted findings."

// redactSensitiveResultForAuxiliary returns the task result that non-UI
// consumers may see. file_scan is the only task whose result contains raw
// operator-facing matches, so every AI/callback/export path funnels through
// this helper. Unknown or unreadable scan documents fail closed.
func redactSensitiveResultForAuxiliary(taskType, result string) string {
	if taskType != protocol.TaskTypeFileScan || result == "" {
		return result
	}
	return redactFileScanResult(result)
}

type redactedFileScanFinding struct {
	Path        string `json:"path"`
	Name        string `json:"name"`
	Rule        string `json:"rule"`
	Value       string `json:"value"`
	Line        int    `json:"line"`
	Occurrences int    `json:"occurrences"`
	Redacted    bool   `json:"redacted"`
}

type redactedFileScanResult struct {
	Root              string                    `json:"root"`
	Patterns          []string                  `json:"patterns,omitempty"`
	Rules             []string                  `json:"rules,omitempty"`
	FilesScanned      int                       `json:"files_scanned"`
	FilesSkipped      int                       `json:"files_skipped,omitempty"`
	SkippedBinary     int                       `json:"skipped_binary,omitempty"`
	SkippedLarge      int                       `json:"skipped_large,omitempty"`
	BytesScanned      int64                     `json:"bytes_scanned"`
	Findings          []redactedFileScanFinding `json:"findings"`
	Counts            map[string]int            `json:"counts,omitempty"`
	Errors            int                       `json:"errors,omitempty"`
	Truncated         bool                      `json:"truncated"`
	TruncatedReason   string                    `json:"truncated_reason,omitempty"`
	ElapsedMS         int64                     `json:"elapsed_ms,omitempty"`
	AuxiliaryRedacted bool                      `json:"auxiliary_redacted"`
	RedactionNote     string                    `json:"redaction_note"`
}

type fileScanDocument struct {
	Root          string   `json:"root"`
	Patterns      []string `json:"patterns"`
	Rules         []string `json:"rules"`
	FilesScanned  int      `json:"files_scanned"`
	FilesSkipped  int      `json:"files_skipped"`
	SkippedBinary int      `json:"skipped_binary"`
	SkippedLarge  int      `json:"skipped_large"`
	BytesScanned  int64    `json:"bytes_scanned"`
	Findings      *[]struct {
		Path        string `json:"path"`
		Name        string `json:"name"`
		Rule        string `json:"rule"`
		Value       string `json:"value"`
		Line        int    `json:"line"`
		Occurrences int    `json:"occurrences"`
	} `json:"findings"`
	Counts          map[string]int `json:"counts"`
	Errors          int            `json:"errors"`
	Truncated       bool           `json:"truncated"`
	TruncatedReason string         `json:"truncated_reason"`
	ElapsedMS       int64          `json:"elapsed_ms"`
}

func redactFileScanResult(raw string) string {
	var doc fileScanDocument
	if err := json.Unmarshal([]byte(raw), &doc); err != nil || doc.Findings == nil {
		return marshalRedactedFileScanResult(redactedFileScanResult{
			Findings:          []redactedFileScanFinding{},
			AuxiliaryRedacted: true,
			RedactionNote:     "The file_scan result could not be parsed and was withheld from this surface.",
		})
	}
	out := redactedFileScanResult{
		Root:              doc.Root,
		Patterns:          doc.Patterns,
		Rules:             doc.Rules,
		FilesScanned:      doc.FilesScanned,
		FilesSkipped:      doc.FilesSkipped,
		SkippedBinary:     doc.SkippedBinary,
		SkippedLarge:      doc.SkippedLarge,
		BytesScanned:      doc.BytesScanned,
		Findings:          make([]redactedFileScanFinding, 0, len(*doc.Findings)),
		Counts:            doc.Counts,
		Errors:            doc.Errors,
		Truncated:         doc.Truncated,
		TruncatedReason:   doc.TruncatedReason,
		ElapsedMS:         doc.ElapsedMS,
		AuxiliaryRedacted: true,
		RedactionNote:     fileScanRedactionNote,
	}
	for _, finding := range *doc.Findings {
		out.Findings = append(out.Findings, redactedFileScanFinding{
			Path:        finding.Path,
			Name:        finding.Name,
			Rule:        finding.Rule,
			Value:       "[redacted]",
			Line:        finding.Line,
			Occurrences: finding.Occurrences,
			Redacted:    true,
		})
	}
	return marshalRedactedFileScanResult(out)
}

func marshalRedactedFileScanResult(v redactedFileScanResult) string {
	b, err := json.Marshal(v)
	if err != nil {
		return `{"findings":[],"auxiliary_redacted":true,"redaction_note":"The file_scan result was withheld."}`
	}
	return string(b)
}
