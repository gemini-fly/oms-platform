package platform

import (
	"strings"
	"time"
)

func NormalizeCloudProvider(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = strings.ReplaceAll(normalized, " ", "_")
	normalized = strings.ReplaceAll(normalized, "-", "_")
	switch normalized {
	case "aliyun", "ali", "alibaba", "alibaba_cloud", "阿里云":
		return "aliyun"
	case "tencent", "qcloud", "tencent_cloud", "腾讯云":
		return "tencent"
	case "huawei", "huaweicloud", "huawei_cloud", "华为云":
		return "huawei"
	default:
		return normalized
	}
}

func SupportedCloudProvider(provider string) bool {
	switch NormalizeCloudProvider(provider) {
	case "aliyun", "tencent", "huawei":
		return true
	default:
		return false
	}
}

func CloudProviderDisplayName(provider string) string {
	switch NormalizeCloudProvider(provider) {
	case "aliyun":
		return "阿里云"
	case "tencent":
		return "腾讯云"
	case "huawei":
		return "华为云"
	default:
		return provider
	}
}

func DefaultCloudRegions(provider string) []string {
	switch NormalizeCloudProvider(provider) {
	case "aliyun":
		return []string{"cn-hangzhou", "cn-shanghai"}
	case "tencent":
		return []string{"ap-shanghai", "ap-guangzhou"}
	case "huawei":
		return []string{"cn-east-3", "cn-north-4"}
	default:
		return []string{"default"}
	}
}

func DefaultCloudResourceTypes(provider string) []string {
	switch NormalizeCloudProvider(provider) {
	case "aliyun":
		return []string{"ecs.instance", "vpc.vswitch", "rds.instance", "slb.loadbalancer", "ack.cluster"}
	case "tencent":
		return []string{"cvm.instance", "vpc.vpc", "cdb.instance", "clb.loadbalancer", "tke.cluster"}
	case "huawei":
		return []string{"ecs.cloudservers", "vpc.vpcs", "rds.instances", "elb.loadbalancers", "cce.clusters"}
	default:
		return []string{"generic.resource"}
	}
}

func CloudResourceTypeMetadata(provider, resourceType string) (string, string) {
	key := NormalizeCloudProvider(provider) + "/" + strings.ToLower(strings.TrimSpace(resourceType))
	catalog := map[string][2]string{
		"aliyun/ecs.instance":      {"ECS 云服务器", "compute"},
		"aliyun/vpc.vswitch":       {"VSwitch 交换机", "network"},
		"aliyun/rds.instance":      {"RDS 数据库", "database"},
		"aliyun/slb.loadbalancer":  {"SLB 负载均衡", "load_balancer"},
		"aliyun/ack.cluster":       {"ACK 集群", "kubernetes"},
		"tencent/cvm.instance":     {"CVM 云服务器", "compute"},
		"tencent/vpc.vpc":          {"VPC 网络", "network"},
		"tencent/cdb.instance":     {"CDB 数据库", "database"},
		"tencent/clb.loadbalancer": {"CLB 负载均衡", "load_balancer"},
		"tencent/tke.cluster":      {"TKE 集群", "kubernetes"},
		"huawei/ecs.cloudservers":  {"ECS 云服务器", "compute"},
		"huawei/vpc.vpcs":          {"VPC 网络", "network"},
		"huawei/rds.instances":     {"RDS 数据库", "database"},
		"huawei/elb.loadbalancers": {"ELB 负载均衡", "load_balancer"},
		"huawei/cce.clusters":      {"CCE 集群", "kubernetes"},
		"default/generic.resource": {"通用资源", "generic"},
	}
	if item, ok := catalog[key]; ok {
		return item[0], item[1]
	}
	resourceType = strings.TrimSpace(resourceType)
	if resourceType == "" {
		resourceType = "generic.resource"
	}
	return resourceType, "generic"
}

func BuiltinCloudResourceTypes(now time.Time) []CloudResourceType {
	providers := []string{"aliyun", "tencent", "huawei"}
	items := make([]CloudResourceType, 0)
	for _, provider := range providers {
		for _, resourceType := range DefaultCloudResourceTypes(provider) {
			displayName, category := CloudResourceTypeMetadata(provider, resourceType)
			items = append(items, CloudResourceType{
				Provider:     provider,
				ResourceType: resourceType,
				DisplayName:  displayName,
				Category:     category,
				SyncMode:     builtinCloudResourceSyncMode(provider, resourceType),
				Status:       "enabled",
				UpdatedAt:    now,
			})
		}
	}
	return items
}

func builtinCloudResourceSyncMode(provider, resourceType string) string {
	if NormalizeCloudProvider(provider) == "aliyun" && strings.EqualFold(strings.TrimSpace(resourceType), "ecs.instance") {
		return "cloud_api"
	}
	return "api_pending"
}
