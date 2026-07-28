//go:build windows

package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"
)

type CloudProviderInfo struct {
	Provider string `json:"provider"`
	Region   string `json:"region,omitempty"`
	Zone     string `json:"zone,omitempty"`
	Project  string `json:"project,omitempty"`
	Account  string `json:"account,omitempty"`
	Instance string `json:"instance,omitempty"`
}

func detectCloudProvider() *CloudProviderInfo {
	client := &http.Client{Timeout: 2 * time.Second}

	// Try AWS IMDSv2 token endpoint first
	if info := tryAWSDetect(client); info != nil {
		return info
	}

	// Try Azure IMDS
	if info := tryAzureDetect(client); info != nil {
		return info
	}

	// Try GCP metadata
	if info := tryGCPDetect(client); info != nil {
		return info
	}

	// Try Oracle Cloud (uses same 169.254.169.254 but different headers)
	if info := tryOracleDetect(client); info != nil {
		return info
	}

	// Try Alibaba Cloud
	if info := tryAlibabaDetect(client); info != nil {
		return info
	}

	return &CloudProviderInfo{Provider: "unknown"}
}

func tryAWSDetect(client *http.Client) *CloudProviderInfo {
	tokenReq, _ := http.NewRequest("PUT", "http://169.254.169.254/latest/api/token", nil)
	tokenReq.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "5")
	resp, err := client.Do(tokenReq)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}

	// Confirm by fetching instance-id
	req, _ := http.NewRequest("GET", "http://169.254.169.254/latest/meta-data/instance-id", nil)
	tokenBytes := make([]byte, 256)
	n, _ := resp.Body.Read(tokenBytes)
	token := strings.TrimSpace(string(tokenBytes[:n]))
	if token == "" {
		return nil
	}
	req.Header.Set("X-aws-ec2-metadata-token", token)

	r2, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer r2.Body.Close()
	if r2.StatusCode != 200 {
		return nil
	}

	info := &CloudProviderInfo{Provider: "AWS"}

	// Region
	regionReq, _ := http.NewRequest("GET", "http://169.254.169.254/latest/meta-data/placement/region", nil)
	regionReq.Header.Set("X-aws-ec2-metadata-token", token)
	if rr, err := client.Do(regionReq); err == nil {
		defer rr.Body.Close()
		buf := make([]byte, 64)
		n, _ := rr.Body.Read(buf)
		info.Region = strings.TrimSpace(string(buf[:n]))
	}

	// Instance ID
	buf := make([]byte, 64)
	n2, _ := r2.Body.Read(buf)
	info.Instance = strings.TrimSpace(string(buf[:n2]))

	// Account ID from iam/info
	accReq, _ := http.NewRequest("GET", "http://169.254.169.254/latest/meta-data/iam/info", nil)
	accReq.Header.Set("X-aws-ec2-metadata-token", token)
	if ar, err := client.Do(accReq); err == nil {
		defer ar.Body.Close()
		buf2 := make([]byte, 512)
		n3, _ := ar.Body.Read(buf2)
		info.Account = strings.TrimSpace(string(buf2[:n3]))
	}

	return info
}

func tryAzureDetect(client *http.Client) *CloudProviderInfo {
	req, _ := http.NewRequest("GET", "http://169.254.169.254/metadata/instance?api-version=2021-02-01", nil)
	req.Header.Set("Metadata", "true")
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}
	return &CloudProviderInfo{Provider: "Azure"}
}

func tryGCPDetect(client *http.Client) *CloudProviderInfo {
	req, _ := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/instance/id", nil)
	req.Header.Set("Metadata-Flavor", "Google")
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}

	info := &CloudProviderInfo{Provider: "GCP"}

	zoneReq, _ := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/instance/zone", nil)
	zoneReq.Header.Set("Metadata-Flavor", "Google")
	if zr, err := client.Do(zoneReq); err == nil {
		defer zr.Body.Close()
		buf := make([]byte, 256)
		n, _ := zr.Body.Read(buf)
		info.Zone = strings.TrimSpace(string(buf[:n]))
	}

	projReq, _ := http.NewRequest("GET", "http://metadata.google.internal/computeMetadata/v1/project/project-id", nil)
	projReq.Header.Set("Metadata-Flavor", "Google")
	if pr, err := client.Do(projReq); err == nil {
		defer pr.Body.Close()
		buf := make([]byte, 256)
		n, _ := pr.Body.Read(buf)
		info.Project = strings.TrimSpace(string(buf[:n]))
	}

	return info
}

func tryOracleDetect(client *http.Client) *CloudProviderInfo {
	req, _ := http.NewRequest("GET", "http://169.254.169.254/opc/v2/instance/", nil)
	req.Header.Set("Authorization", "Bearer Oracle")
	resp, err := client.Do(req)
	if err != nil {
		req2, _ := http.NewRequest("GET", "http://169.254.169.254/opc/v1/instance/", nil)
		resp2, err2 := client.Do(req2)
		if err2 != nil {
			return nil
		}
		defer resp2.Body.Close()
		if resp2.StatusCode == 200 {
			return &CloudProviderInfo{Provider: "Oracle"}
		}
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		return &CloudProviderInfo{Provider: "Oracle"}
	}
	return nil
}

func tryAlibabaDetect(client *http.Client) *CloudProviderInfo {
	resp, err := client.Get("http://100.100.100.200/latest/meta-data/instance-id")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		return &CloudProviderInfo{Provider: "Alibaba"}
	}
	return nil
}

func (c *CloudProviderInfo) String() string {
	var parts []string
	parts = append(parts, fmt.Sprintf("Provider: %s", c.Provider))
	if c.Region != "" {
		parts = append(parts, fmt.Sprintf("Region: %s", c.Region))
	}
	if c.Zone != "" {
		parts = append(parts, fmt.Sprintf("Zone: %s", c.Zone))
	}
	if c.Project != "" {
		parts = append(parts, fmt.Sprintf("Project: %s", c.Project))
	}
	if c.Account != "" {
		parts = append(parts, fmt.Sprintf("Account: %s", c.Account))
	}
	if c.Instance != "" {
		parts = append(parts, fmt.Sprintf("Instance: %s", c.Instance))
	}
	return strings.Join(parts, ", ")
}
