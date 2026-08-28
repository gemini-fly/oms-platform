package cmdb

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gemini-fly/oms-platform/internal/platform"
)

type cloudAssetSyncResult struct {
	Assets               []platform.Asset
	SkippedResourceTypes []string
}

func syncCloudAccountAssets(account platform.CloudAccount, resourceTypes, regions []string, syncedAt time.Time) (cloudAssetSyncResult, error) {
	account.Provider = platform.NormalizeCloudProvider(account.Provider)
	switch account.Provider {
	case "aliyun":
		return syncAliyunCloudAccountAssets(account, resourceTypes, regions, syncedAt)
	case "tencent", "huawei":
		return cloudAssetSyncResult{SkippedResourceTypes: dedupeStrings(resourceTypes)}, fmt.Errorf("%s真实同步尚未接入具体资源 API，不会生成模拟资产", platform.CloudProviderDisplayName(account.Provider))
	default:
		return cloudAssetSyncResult{}, fmt.Errorf("unsupported cloud provider: %s", account.Provider)
	}
}

func syncAliyunCloudAccountAssets(account platform.CloudAccount, resourceTypes, regions []string, syncedAt time.Time) (cloudAssetSyncResult, error) {
	result := cloudAssetSyncResult{}
	supported := false
	for _, resourceType := range resourceTypes {
		if strings.EqualFold(strings.TrimSpace(resourceType), "ecs.instance") {
			supported = true
			break
		}
	}
	if !supported {
		result.SkippedResourceTypes = dedupeStrings(resourceTypes)
		return result, fmt.Errorf("阿里云当前仅支持真实同步 ecs.instance；未实现资源类型: %s", strings.Join(result.SkippedResourceTypes, ", "))
	}
	if strings.TrimSpace(account.AccessKeyID) == "" || strings.TrimSpace(account.AccessKeySecret) == "" {
		return result, errors.New("云账号未配置 AccessKey ID 或 AccessKey Secret，无法真实同步阿里云资源")
	}
	resolvedRegions, err := resolveAliyunRegions(account, regions)
	if err != nil {
		return result, err
	}
	if len(resolvedRegions) == 0 {
		return result, errors.New("阿里云地域列表为空，无法同步资源")
	}

	for _, resourceType := range resourceTypes {
		resourceType = strings.ToLower(strings.TrimSpace(resourceType))
		switch resourceType {
		case "ecs.instance":
			for _, region := range resolvedRegions {
				instances, err := aliyunECSListInstances(account, region)
				if err != nil {
					return result, err
				}
				for _, instance := range instances {
					asset, ok := aliyunECSInstanceAsset(account, region, instance, syncedAt)
					if ok {
						result.Assets = append(result.Assets, asset)
					}
				}
			}
		default:
			result.SkippedResourceTypes = append(result.SkippedResourceTypes, resourceType)
		}
	}
	result.SkippedResourceTypes = dedupeStrings(result.SkippedResourceTypes)
	return result, nil
}

func resolveAliyunRegions(account platform.CloudAccount, regions []string) ([]string, error) {
	regions = normalizeCloudRegions(account.Provider, regions)
	if len(regions) > 0 {
		return regions, nil
	}
	resolved, err := aliyunECSListRegions(account)
	if err != nil {
		return nil, err
	}
	resolved = normalizeStringList(resolved, nil)
	sort.Strings(resolved)
	return resolved, nil
}

func aliyunECSInstanceAsset(account platform.CloudAccount, region string, instance aliyunECSInstance, syncedAt time.Time) (platform.Asset, bool) {
	instanceID := strings.TrimSpace(instance.InstanceID)
	if instanceID == "" {
		return platform.Asset{}, false
	}
	name := strings.TrimSpace(instance.InstanceName)
	if name == "" {
		name = instanceID
	}
	privateIPs := instance.VpcAttributes.PrivateIPAddress.IPAddress
	publicIPs := append([]string(nil), instance.PublicIPAddress.IPAddress...)
	if strings.TrimSpace(instance.EIPAddress.IPAddress) != "" {
		publicIPs = append(publicIPs, strings.TrimSpace(instance.EIPAddress.IPAddress))
	}
	publicIP := firstString(publicIPs)
	status := normalizeAliyunECSStatus(instance.Status)
	tags := map[string]string{
		"account":       account.AccountRef,
		"provider":      "aliyun",
		"resourceClass": "compute",
	}
	for _, tag := range instance.Tags.Tag {
		key := strings.TrimSpace(tag.TagKey)
		if key != "" {
			tags[key] = tag.TagValue
		}
	}
	return platform.Asset{
		Provider:     "aliyun",
		AccountID:    account.ID,
		ResourceType: "ecs.instance",
		ResourceUID:  instanceID,
		Name:         name,
		Region:       fallback(strings.TrimSpace(instance.RegionID), region),
		Source:       "sync",
		Status:       status,
		Tags:         tags,
		LastSyncedAt: syncedAt,
		Raw: map[string]any{
			"providerName":  platform.CloudProviderDisplayName("aliyun"),
			"accountName":   account.Name,
			"accountRef":    account.AccountRef,
			"credentialRef": account.CredentialRef,
			"accessKeyId":   maskAccessKeyID(account.AccessKeyID),
			"syncMode":      "aliyun_api",
			"sourceApi":     "DescribeInstances",
			"instanceId":    instanceID,
			"instanceName":  name,
			"instanceType":  instance.InstanceType,
			"imageId":       instance.ImageID,
			"osName":        instance.OSName,
			"osType":        instance.OSType,
			"zoneId":        instance.ZoneID,
			"vpcId":         instance.VpcAttributes.VpcID,
			"vSwitchId":     instance.VpcAttributes.VSwitchID,
			"privateIp":     firstString(privateIPs),
			"privateIps":    privateIPs,
			"publicIp":      publicIP,
			"publicIps":     publicIPs,
			"creationTime":  instance.CreationTime,
		},
	}, true
}

func normalizeAliyunECSStatus(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "running":
		return "running"
	case "stopped":
		return "stopped"
	case "starting":
		return "starting"
	case "stopping":
		return "stopping"
	default:
		if strings.TrimSpace(value) == "" {
			return "unknown"
		}
		return strings.ToLower(strings.TrimSpace(value))
	}
}

func cloudResourceSyncMode(provider, resourceType string) string {
	provider = platform.NormalizeCloudProvider(provider)
	resourceType = strings.ToLower(strings.TrimSpace(resourceType))
	if provider == "aliyun" && resourceType == "ecs.instance" {
		return "cloud_api"
	}
	return "api_pending"
}

func dedupeStrings(values []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.ToLower(strings.TrimSpace(value))
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func firstString(values []string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
