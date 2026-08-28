package cmdb

import (
	"context"
	"crypto/hmac"
	"crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gemini-fly/oms-platform/internal/platform"
)

const aliyunECSEndpoint = "https://ecs.aliyuncs.com/"

var aliyunECSListInstances = listAliyunECSInstances
var aliyunECSListRegions = listAliyunECSRegions

type aliyunECSInstance struct {
	InstanceID      string `json:"InstanceId"`
	InstanceName    string `json:"InstanceName"`
	InstanceType    string `json:"InstanceType"`
	RegionID        string `json:"RegionId"`
	ZoneID          string `json:"ZoneId"`
	Status          string `json:"Status"`
	OSType          string `json:"OSType"`
	OSName          string `json:"OSName"`
	ImageID         string `json:"ImageId"`
	CreationTime    string `json:"CreationTime"`
	VpcAttributes   aliyunECSVPCAttributes
	PublicIPAddress aliyunECSIPAddress  `json:"PublicIpAddress"`
	EIPAddress      aliyunECSEIPAddress `json:"EipAddress"`
	Tags            aliyunECSTags       `json:"Tags"`
}

type aliyunECSVPCAttributes struct {
	VpcID            string             `json:"VpcId"`
	VSwitchID        string             `json:"VSwitchId"`
	PrivateIPAddress aliyunECSIPAddress `json:"PrivateIpAddress"`
}

type aliyunECSIPAddress struct {
	IPAddress []string `json:"IpAddress"`
}

type aliyunECSEIPAddress struct {
	IPAddress string `json:"IpAddress"`
}

type aliyunECSTags struct {
	Tag []aliyunECSTag `json:"Tag"`
}

type aliyunECSTag struct {
	TagKey   string `json:"TagKey"`
	TagValue string `json:"TagValue"`
}

type aliyunECSDescribeInstancesResponse struct {
	RequestID  string `json:"RequestId"`
	Code       string `json:"Code"`
	Message    string `json:"Message"`
	PageNumber int    `json:"PageNumber"`
	PageSize   int    `json:"PageSize"`
	TotalCount int    `json:"TotalCount"`
	Instances  struct {
		Instance []aliyunECSInstance `json:"Instance"`
	} `json:"Instances"`
}

type aliyunECSRegion struct {
	RegionID  string `json:"RegionId"`
	LocalName string `json:"LocalName"`
}

type aliyunECSDescribeRegionsResponse struct {
	RequestID string `json:"RequestId"`
	Code      string `json:"Code"`
	Message   string `json:"Message"`
	Regions   struct {
		Region []aliyunECSRegion `json:"Region"`
	} `json:"Regions"`
}

type aliyunRPCErrorResponse struct {
	RequestID string `json:"RequestId"`
	Code      string `json:"Code"`
	Message   string `json:"Message"`
}

func listAliyunECSRegions(account platform.CloudAccount) ([]string, error) {
	client := &http.Client{Timeout: 20 * time.Second}
	body, err := callAliyunECSRPC(client, account, aliyunECSEndpoint, map[string]string{
		"Action":  "DescribeRegions",
		"Version": "2014-05-26",
	})
	if err != nil {
		return nil, err
	}
	var decoded aliyunECSDescribeRegionsResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return nil, fmt.Errorf("Aliyun ECS DescribeRegions returned invalid JSON: %w", err)
	}
	regions := make([]string, 0, len(decoded.Regions.Region))
	for _, region := range decoded.Regions.Region {
		if strings.TrimSpace(region.RegionID) != "" {
			regions = append(regions, strings.TrimSpace(region.RegionID))
		}
	}
	sort.Strings(regions)
	return regions, nil
}

func listAliyunECSInstances(account platform.CloudAccount, region string) ([]aliyunECSInstance, error) {
	const pageSize = 100
	client := &http.Client{Timeout: 20 * time.Second}
	instances := []aliyunECSInstance{}
	for pageNumber := 1; ; pageNumber++ {
		response, err := describeAliyunECSInstancesPage(client, account, region, pageNumber, pageSize)
		if err != nil {
			return nil, err
		}
		instances = append(instances, response.Instances.Instance...)
		if len(response.Instances.Instance) < pageSize {
			break
		}
		if response.TotalCount > 0 && pageNumber*pageSize >= response.TotalCount {
			break
		}
	}
	return instances, nil
}

func describeAliyunECSInstancesPage(client *http.Client, account platform.CloudAccount, region string, pageNumber, pageSize int) (aliyunECSDescribeInstancesResponse, error) {
	region = strings.TrimSpace(region)
	body, err := callAliyunECSRPC(client, account, aliyunECSEndpointForRegion(region), map[string]string{
		"Action":     "DescribeInstances",
		"Version":    "2014-05-26",
		"RegionId":   region,
		"PageNumber": strconv.Itoa(pageNumber),
		"PageSize":   strconv.Itoa(pageSize),
	})
	if err != nil {
		return aliyunECSDescribeInstancesResponse{}, err
	}
	var decoded aliyunECSDescribeInstancesResponse
	if err := json.Unmarshal(body, &decoded); err != nil {
		return decoded, fmt.Errorf("Aliyun ECS DescribeInstances returned invalid JSON: %w", err)
	}
	return decoded, nil
}

func callAliyunECSRPC(client *http.Client, account platform.CloudAccount, endpoint string, actionParams map[string]string) ([]byte, error) {
	params := map[string]string{
		"Format":           "JSON",
		"AccessKeyId":      strings.TrimSpace(account.AccessKeyID),
		"SignatureMethod":  "HMAC-SHA1",
		"SignatureVersion": "1.0",
		"SignatureNonce":   fmt.Sprintf("%s-%d", actionParams["Action"], time.Now().UnixNano()),
		"Timestamp":        time.Now().UTC().Format("2006-01-02T15:04:05Z"),
	}
	for key, value := range actionParams {
		params[key] = value
	}
	signature := signAliyunRPC(params, strings.TrimSpace(account.AccessKeySecret))
	query := url.Values{}
	for key, value := range params {
		query.Set(key, value)
	}
	query.Set("Signature", signature)

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()
	requestURL := endpoint + "?" + query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, requestURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("Aliyun ECS %s failed: %w", actionParams["Action"], err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4*1024*1024))
	if err != nil {
		return nil, err
	}
	var rpcError aliyunRPCErrorResponse
	if err := json.Unmarshal(body, &rpcError); err != nil {
		return nil, fmt.Errorf("Aliyun ECS %s returned invalid JSON: %w", actionParams["Action"], err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || rpcError.Code != "" {
		code := fallback(rpcError.Code, resp.Status)
		message := strings.TrimSpace(rpcError.Message)
		if message == "" {
			message = "request failed"
		}
		return nil, fmt.Errorf("Aliyun ECS %s failed: %s: %s", actionParams["Action"], code, message)
	}
	return body, nil
}

func aliyunECSEndpointForRegion(region string) string {
	region = strings.TrimSpace(region)
	if region == "" {
		return aliyunECSEndpoint
	}
	return "https://ecs." + region + ".aliyuncs.com/"
}

func signAliyunRPC(params map[string]string, accessKeySecret string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	encodedParts := make([]string, 0, len(keys))
	for _, key := range keys {
		encodedParts = append(encodedParts, aliyunPercentEncode(key)+"="+aliyunPercentEncode(params[key]))
	}
	canonicalizedQuery := strings.Join(encodedParts, "&")
	stringToSign := "GET&%2F&" + aliyunPercentEncode(canonicalizedQuery)
	mac := hmac.New(sha1.New, []byte(accessKeySecret+"&"))
	_, _ = mac.Write([]byte(stringToSign))
	return base64.StdEncoding.EncodeToString(mac.Sum(nil))
}

func aliyunPercentEncode(value string) string {
	encoded := url.QueryEscape(value)
	encoded = strings.ReplaceAll(encoded, "+", "%20")
	encoded = strings.ReplaceAll(encoded, "*", "%2A")
	encoded = strings.ReplaceAll(encoded, "%7E", "~")
	return encoded
}
