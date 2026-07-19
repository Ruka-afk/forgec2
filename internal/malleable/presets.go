package malleable

var Presets = map[string]string{
	"default": `
name: default
user_agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36"
sleep: 10
jitter: 20
http-get:
  uri: /api/v1/beacon
  verb: GET
  client:
    metadata:
      - name: base64
      - name: prepend
        value: "session="
    output:
      - name: print
  server:
    output:
      - name: print
http-post:
  uri: /api/v1/beacon
  verb: POST
  client:
    id:
      - name: base64
    output:
      - name: base64
  server:
    output:
      - name: print
post-ex:
  pipename: forgec2
  key: forgec2
`,

	"slack": `
name: slack
user_agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Slack/4.36.0"
sleep: 15
jitter: 25
http-get:
  uri: /api/rtm.connect
  verb: GET
  client:
    metadata:
      - name: base64
      - name: prepend
        value: "token=xoxb-"
    output:
      - name: print
  server:
    output:
      - name: base64
http-post:
  uri: /api/chat.postMessage
  verb: POST
  client:
    id:
      - name: base64
    output:
      - name: base64
  server:
    output:
      - name: base64
      - name: prepend
        value: "{\"ok\":true,"
      - name: append
        value: "}"
post-ex:
  pipename: slack_c2
  key: slack_secret
`,

	"microsoft": `
name: microsoft
user_agent: "Microsoft Office/16.0 (Windows NT 10.0; Microsoft Windows 10 Pro; en-US)"
sleep: 30
jitter: 15
http-get:
  uri: /common/oauth2/token
  verb: GET
  client:
    metadata:
      - name: base64
      - name: prepend
        value: "session_id="
    output:
      - name: print
  server:
    output:
      - name: base64
http-post:
  uri: /common/oauth2/token
  verb: POST
  client:
    id:
      - name: append
        value: "@microsoft.com"
    output:
      - name: base64
      - name: xor
        value: "microsoft"
  server:
    output:
      - name: base64
post-ex:
  pipename: microsoft_c2
  key: msft
`,

	"cloudflare": `
name: cloudflare
user_agent: "Mozilla/5.0 (compatible; Cloudflare-Health-Checks/1.0; +https://www.cloudflare.com/)"
sleep: 20
jitter: 10
http-get:
  uri: /cdn-cgi/trace
  verb: GET
  client:
    metadata:
      - name: base64
      - name: prepend
        value: "cf-ray="
    output:
      - name: print
  server:
    output:
      - name: base64
http-post:
  uri: /cdn-cgi/rum
  verb: POST
  client:
    id:
      - name: base64
    output:
      - name: base64
  server:
    output:
      - name: print
post-ex:
  pipename: cloudflare_c2
  key: cf
`,

	"google_analytics": `
name: google_analytics
user_agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
sleep: 5
jitter: 30
http-get:
  uri: /collect
  verb: GET
  client:
    metadata:
      - name: base64
      - name: prepend
        value: "v=1&t=pageview&tid=UA-"
    output:
      - name: print
  server:
    output:
      - name: base64
      - name: prepend
        value: ")"
http-post:
  uri: /batch
  verb: POST
  client:
    id:
      - name: base64
    output:
      - name: base64url
  server:
    output:
      - name: base64url
post-ex:
  pipename: ga_c2
  key: ga4
`,

	"github": `
name: github
user_agent: "GitHub-Hookshot/abcdef123456"
sleep: 60
jitter: 10
http-get:
  uri: /api/v3/repos/forgec2/forgec2
  verb: GET
  client:
    metadata:
      - name: base64
      - name: prepend
        value: "authorization: bearer "
    output:
      - name: print
  server:
    output:
      - name: base64
http-post:
  uri: /api/v3/repos/forgec2/forgec2/issues
  verb: POST
  client:
    id:
      - name: base64
    output:
      - name: base64
  server:
    output:
      - name: print
post-ex:
  pipename: github_c2
  key: gh
`,

	"akamai": `
name: akamai
user_agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
sleep: 20
jitter: 15
http-get:
  uri: /akamai/collect
  verb: GET
  client:
    metadata:
      - name: base64
      - name: prepend
        value: "__ak="
    output:
      - name: print
  server:
    output:
      - name: base64
http-post:
  uri: /akamai/beacon
  verb: POST
  client:
    id:
      - name: base64
    output:
      - name: base64
  server:
    output:
      - name: base64
      - name: prepend
        value: "{\"status\":\"ok\",\"data\":\""
      - name: append
        value: "\"}"
post-ex:
  pipename: akamai_c2
  key: ak
`,

	"cloudfront": `
name: cloudfront
user_agent: "Amazon CloudFront"
sleep: 30
jitter: 20
http-get:
  uri: /d/cloudfront.gif
  verb: GET
  client:
    metadata:
      - name: netbios
      - name: base64
    output:
      - name: print
  server:
    output:
      - name: base64
http-post:
  uri: /e/cloudfront-post
  verb: POST
  client:
    id:
      - name: base64
    output:
      - name: mask
        value: "cloudfront"
  server:
    output:
      - name: mask
        value: "cloudfront"
post-ex:
  pipename: cloudfront_c2
  key: cf_key
`,

	"azure": `
name: azure
user_agent: "AzureDevOps/2024.01.01 (Windows; MSIL)"
sleep: 45
jitter: 20
http-get:
  uri: /dev.azure.com/forgec2/_apis
  verb: GET
  client:
    metadata:
      - name: base64
      - name: prepend
        value: "Authorization: Basic "
    output:
      - name: print
  server:
    output:
      - name: base64
http-post:
  uri: /dev.azure.com/forgec2/_apis/build/builds
  verb: POST
  client:
    id:
      - name: append
        value: "@azure.com"
    output:
      - name: base64
  server:
    output:
      - name: base64
      - name: prepend
        value: "{\"count\":1,\"value\":["
      - name: append
        value: "]}"
post-ex:
  pipename: azure_c2
  key: azure_secret
`,

	"okta": `
name: okta
user_agent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
sleep: 25
jitter: 20
http-get:
  uri: /oauth2/default/v1/authorize
  verb: GET
  client:
    metadata:
      - name: base64
      - name: prepend
        value: "sessionToken="
    output:
      - name: print
  server:
    output:
      - name: base64
http-post:
  uri: /oauth2/default/v1/token
  verb: POST
  client:
    id:
      - name: base64
    output:
      - name: base64
      - name: xor
        value: "okta2024"
  server:
    output:
      - name: xor
        value: "okta2024"
post-ex:
  pipename: okta_c2
  key: okta
`,

	"aws": `
name: aws
user_agent: "aws-sdk-go/1.44.0 (go1.25; windows; amd64)"
sleep: 30
jitter: 20
http-get:
  uri: /sts.amazonaws.com/?Action=GetCallerIdentity
  verb: GET
  client:
    metadata:
      - name: base64
      - name: prepend
        value: "X-Amz-Credential="
    output:
      - name: print
  server:
    output:
      - name: base64
http-post:
  uri: /ec2.amazonaws.com/?Action=DescribeInstances
  verb: POST
  client:
    id:
      - name: base64url
    output:
      - name: base64
  server:
    output:
      - name: base64
      - name: prepend
        value: "<DescribeInstancesResponse><reservationSet>"
      - name: append
        value: "</reservationSet></DescribeInstancesResponse>"
post-ex:
  pipename: aws_c2
  key: aws_secret
`,
}
