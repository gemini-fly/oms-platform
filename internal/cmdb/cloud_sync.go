package cmdb

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/gemini-fly/oms-platform/internal/platform"
)

func normalizeCloudAccount(item platform.CloudAccount) (platform.CloudAccount, error) {
	item.Provider = platform.NormalizeCloudProvider(item.Provider)
	if !platform.SupportedCloudProvider(item.Provider) {
		return item, fmt.Errorf("unsupported cloud provider: %s", item.Provider)
	}
	item.Name = strings.TrimSpace(item.Name)
	item.AccountRef = strings.TrimSpace(item.AccountRef)
	item.CredentialRef = strings.TrimSpace(item.CredentialRef)
	item.AccessKeyID = strings.TrimSpace(item.AccessKeyID)
	item.AccessKeySecret = strings.TrimSpace(item.AccessKeySecret)
	if item.Name == "" {
		item.Name = platform.CloudProviderDisplayName(item.Provider) + "账号"
	}
	if item.AccountRef == "" {
		item.AccountRef = strings.ToLower(strings.ReplaceAll(item.Name, " ", "-"))
	}
	if item.CredentialRef == "" && (item.AccessKeyID != "" || item.AccessKeySecret != "") {
		item.CredentialRef = fmt.Sprintf("local://cloud-credentials/%s/%s", item.Provider, item.AccountRef)
	}
	item.Regions = normalizeCloudRegions(item.Provider, item.Regions)
	item.ResourceTypes = normalizeResourceTypes(item.Provider, item.ResourceTypes)
	item.Status = strings.ToLower(strings.TrimSpace(item.Status))
	if item.Status == "" {
		item.Status = "enabled"
	}
	return item, nil
}

func normalizeSyncJob(item platform.SyncJob) platform.SyncJob {
	item.Name = strings.TrimSpace(item.Name)
	item.Provider = platform.NormalizeCloudProvider(item.Provider)
	item.ResourceTypes = normalizeStringList(item.ResourceTypes, nil)
	for index := range item.ResourceTypes {
		item.ResourceTypes[index] = strings.ToLower(item.ResourceTypes[index])
	}
	item.Regions = normalizeCloudRegions(item.Provider, item.Regions)
	item.Status = strings.ToLower(strings.TrimSpace(item.Status))
	if item.Status == "" {
		item.Status = "created"
	}
	return item
}

func normalizeStringList(values, defaults []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(values)+len(defaults))
	source := defaults
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			source = values
			break
		}
	}
	for _, value := range source {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}

func normalizeResourceTypes(provider string, values []string) []string {
	out := normalizeStringList(values, nil)
	if len(out) > 0 {
		for index := range out {
			out[index] = strings.ToLower(out[index])
		}
		return out
	}
	return platform.DefaultCloudResourceTypes(provider)
}

func normalizeCloudRegions(provider string, values []string) []string {
	out := normalizeStringList(values, nil)
	for _, value := range out {
		switch strings.ToLower(strings.TrimSpace(value)) {
		case "*", "all", "all_regions", "all-regions", "全部", "全部地域":
			return nil
		}
	}
	if cloudRegionsEqual(out, platform.DefaultCloudRegions(provider)) {
		return nil
	}
	return out
}

func cloudRegionsEqual(left, right []string) bool {
	if len(left) == 0 || len(left) != len(right) {
		return false
	}
	seen := map[string]bool{}
	for _, value := range left {
		seen[strings.ToLower(strings.TrimSpace(value))] = true
	}
	for _, value := range right {
		if !seen[strings.ToLower(strings.TrimSpace(value))] {
			return false
		}
	}
	return true
}

func runCloudSyncLocked(store *platform.Store, jobID int64, actorID int64) (platform.SyncJob, error) {
	job := findSyncJob(store.SyncJobs, jobID)
	if job == nil {
		return platform.SyncJob{}, errors.New("sync job not found")
	}
	*job = normalizeSyncJob(*job)
	accounts := targetCloudAccountsLocked(store, *job)
	if len(accounts) == 0 {
		job.Status = "failed"
		job.Summary = "未找到启用的云账号"
		now := time.Now()
		job.LastRunAt = &now
		return *job, errors.New(job.Summary)
	}

	now := time.Now()
	job.Status = "running"
	job.LastRunAt = &now
	created := 0
	updated := 0
	resourceTypeSet := map[string]bool{}
	skippedTypeSet := map[string]bool{}
	failures := []string{}

	for _, account := range accounts {
		resourceTypes := job.ResourceTypes
		if len(resourceTypes) == 0 {
			resourceTypes = account.ResourceTypes
		}
		resourceTypes = normalizeResourceTypes(account.Provider, resourceTypes)

		regions := job.Regions
		if len(regions) == 0 {
			regions = account.Regions
		}
		regions = normalizeCloudRegions(account.Provider, regions)

		for _, resourceType := range resourceTypes {
			ensureCloudResourceTypeLocked(store, account.Provider, resourceType, now)
			resourceTypeSet[platform.NormalizeCloudProvider(account.Provider)+"/"+resourceType] = true
		}

		result, err := syncCloudAccountAssets(account, resourceTypes, regions, now)
		if err != nil {
			failures = append(failures, fmt.Sprintf("%s: %s", account.AccountRef, err.Error()))
			continue
		}
		for _, resourceType := range result.SkippedResourceTypes {
			skippedTypeSet[platform.NormalizeCloudProvider(account.Provider)+"/"+resourceType] = true
		}
		for _, asset := range result.Assets {
			if upsertCloudAssetLocked(store, asset) {
				created++
			} else {
				updated++
			}
		}
	}

	if len(failures) > 0 && created+updated == 0 {
		job.Status = "failed"
		job.Summary = strings.Join(failures, "；")
		return *job, errors.New(job.Summary)
	}
	job.Status = "success"
	job.Summary = fmt.Sprintf("真实同步账号:%d，资源类型:%d，新增:%d，更新:%d", len(accounts), len(resourceTypeSet), created, updated)
	if len(skippedTypeSet) > 0 {
		job.Summary += fmt.Sprintf("，跳过未实现:%d", len(skippedTypeSet))
	}
	if len(failures) > 0 {
		job.Summary += fmt.Sprintf("，部分失败:%d", len(failures))
	}
	store.Audit(actorID, "cmdb.sync_job.run", "sync_job", job.ID, "success", job.Summary)
	return *job, nil
}

func targetCloudAccountsLocked(store *platform.Store, job platform.SyncJob) []platform.CloudAccount {
	items := make([]platform.CloudAccount, 0)
	for _, account := range store.CloudAccounts {
		account.Provider = platform.NormalizeCloudProvider(account.Provider)
		if account.Status == "disabled" {
			continue
		}
		if job.AccountID != 0 {
			if account.ID == job.AccountID {
				items = append(items, account)
				break
			}
			continue
		}
		if job.Provider == "" || platform.NormalizeCloudProvider(job.Provider) == account.Provider {
			items = append(items, account)
		}
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].Provider == items[j].Provider {
			return items[i].AccountRef < items[j].AccountRef
		}
		return items[i].Provider < items[j].Provider
	})
	return items
}

func ensureCloudResourceTypeLocked(store *platform.Store, provider, resourceType string, now time.Time) {
	provider = platform.NormalizeCloudProvider(provider)
	resourceType = strings.ToLower(strings.TrimSpace(resourceType))
	if resourceType == "" {
		resourceType = "generic.resource"
	}
	for index := range store.CloudResourceTypes {
		if platform.NormalizeCloudProvider(store.CloudResourceTypes[index].Provider) == provider &&
			strings.EqualFold(store.CloudResourceTypes[index].ResourceType, resourceType) {
			store.CloudResourceTypes[index].UpdatedAt = now
			if store.CloudResourceTypes[index].SyncMode == "" {
				store.CloudResourceTypes[index].SyncMode = cloudResourceSyncMode(provider, resourceType)
			}
			if store.CloudResourceTypes[index].SyncMode == "generic" {
				store.CloudResourceTypes[index].SyncMode = cloudResourceSyncMode(provider, resourceType)
			}
			if store.CloudResourceTypes[index].Status == "" {
				store.CloudResourceTypes[index].Status = "enabled"
			}
			return
		}
	}
	displayName, category := platform.CloudResourceTypeMetadata(provider, resourceType)
	store.CloudResourceTypes = append(store.CloudResourceTypes, platform.CloudResourceType{
		ID:           store.Next("cloud_resource_type"),
		Provider:     provider,
		ResourceType: resourceType,
		DisplayName:  displayName,
		Category:     category,
		SyncMode:     cloudResourceSyncMode(provider, resourceType),
		Status:       "discovered",
		UpdatedAt:    now,
	})
}

func upsertCloudAssetLocked(store *platform.Store, asset platform.Asset) bool {
	for index := range store.Assets {
		current := store.Assets[index]
		if platform.NormalizeCloudProvider(current.Provider) != asset.Provider ||
			current.AccountID != asset.AccountID ||
			current.Region != asset.Region ||
			current.ResourceType != asset.ResourceType ||
			current.ResourceUID != asset.ResourceUID {
			continue
		}
		asset.ID = current.ID
		asset.ScopeType = current.ScopeType
		asset.ScopeID = current.ScopeID
		store.Assets[index] = asset
		return false
	}
	store.Assets = append(store.Assets, asset)
	return true
}

func findSyncJob(items []platform.SyncJob, id int64) *platform.SyncJob {
	for index := range items {
		if items[index].ID == id {
			return &items[index]
		}
	}
	return nil
}

func cloudAccountViews(items []platform.CloudAccount) []cloudAccountView {
	views := make([]cloudAccountView, 0, len(items))
	for _, item := range items {
		views = append(views, cloudAccountViewOf(item))
	}
	return views
}

func cloudAccountViewOf(item platform.CloudAccount) cloudAccountView {
	return cloudAccountView{
		ID:               item.ID,
		Provider:         item.Provider,
		Name:             item.Name,
		AccountRef:       item.AccountRef,
		CredentialRef:    item.CredentialRef,
		AccessKeyID:      maskAccessKeyID(item.AccessKeyID),
		SecretConfigured: item.AccessKeySecret != "",
		CredentialMode:   cloudCredentialMode(item),
		Regions:          append([]string(nil), item.Regions...),
		ResourceTypes:    append([]string(nil), item.ResourceTypes...),
		Status:           item.Status,
		Raw:              sanitizeCloudRaw(item.Raw),
	}
}

func cloudCredentialMode(item platform.CloudAccount) string {
	if item.AccessKeyID != "" || item.AccessKeySecret != "" {
		return "access_key"
	}
	if item.CredentialRef != "" {
		return "credential_ref"
	}
	return "not_configured"
}

func maskAccessKeyID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	runes := []rune(value)
	if len(runes) <= 8 {
		return strings.Repeat("*", len(runes))
	}
	return string(runes[:4]) + strings.Repeat("*", len(runes)-8) + string(runes[len(runes)-4:])
}

func sanitizeCloudRaw(raw map[string]any) map[string]any {
	if raw == nil {
		return nil
	}
	out := make(map[string]any, len(raw))
	for key, value := range raw {
		normalized := strings.ToLower(key)
		if strings.Contains(normalized, "secret") ||
			strings.Contains(normalized, "accesskey") ||
			strings.Contains(normalized, "access_key") ||
			strings.Contains(normalized, "token") ||
			strings.Contains(normalized, "password") {
			out[key] = "******"
			continue
		}
		out[key] = value
	}
	return out
}
