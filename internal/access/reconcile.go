package access

import (
	"fmt"
	"reflect"
	"sort"
	"strings"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	sdkaccess "github.com/router-for-me/CLIProxyAPI/v6/sdk/access"
	sdkConfig "github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
	log "github.com/sirupsen/logrus"
)

// ReconcileProviders builds the desired provider list by reusing existing providers when possible
// and creating or removing providers only when their configuration changed. It returns the final
// ordered provider slice along with the identifiers of providers that were added, updated, or
// removed compared to the previous configuration.
func ReconcileProviders(oldCfg, newCfg *config.Config, existing []sdkaccess.Provider) (result []sdkaccess.Provider, added, updated, removed []string, err error) {
	if newCfg == nil {
		return nil, nil, nil, nil, nil
	}

	existingMap := make(map[string]sdkaccess.Provider, len(existing))
	for _, provider := range existing {
		if provider == nil {
			continue
		}
		existingMap[provider.Identifier()] = provider
	}

	oldCfgMap := accessProviderMap(oldCfg)
	newEntries := collectProviderEntries(newCfg)

	result = make([]sdkaccess.Provider, 0, len(newEntries))
	finalIDs := make(map[string]struct{}, len(newEntries))

	isInlineProvider := func(id string) bool {
		return strings.EqualFold(id, sdkConfig.DefaultAccessProviderName)
	}
	appendChange := func(list *[]string, id string) {
		if isInlineProvider(id) {
			return
		}
		*list = append(*list, id)
	}

	for _, providerCfg := range newEntries {
		key := providerIdentifier(providerCfg)
		if key == "" {
			continue
		}

		forceRebuild := strings.EqualFold(strings.TrimSpace(providerCfg.Type), sdkConfig.AccessProviderTypeConfigAPIKey)
		if oldCfgProvider, ok := oldCfgMap[key]; ok {
			isAliased := oldCfgProvider == providerCfg
			if !forceRebuild && !isAliased && providerConfigEqual(oldCfgProvider, providerCfg) {
				if existingProvider, okExisting := existingMap[key]; okExisting {
					result = append(result, existingProvider)
					finalIDs[key] = struct{}{}
					continue
				}
			}
		}

		provider, buildErr := sdkaccess.BuildProvider(providerCfg, &newCfg.SDKConfig)
		if buildErr != nil {
			return nil, nil, nil, nil, buildErr
		}
		if _, ok := oldCfgMap[key]; ok {
			if _, existed := existingMap[key]; existed {
				appendChange(&updated, key)
			} else {
				appendChange(&added, key)
			}
		} else {
			appendChange(&added, key)
		}
		result = append(result, provider)
		finalIDs[key] = struct{}{}
	}

	if len(result) == 0 {
		if inline := sdkConfig.MakeInlineAPIKeyProvider(newCfg.APIKeys); inline != nil {
			key := providerIdentifier(inline)
			if key != "" {
				if oldCfgProvider, ok := oldCfgMap[key]; ok {
					if providerConfigEqual(oldCfgProvider, inline) {
						if existingProvider, okExisting := existingMap[key]; okExisting {
							result = append(result, existingProvider)
							finalIDs[key] = struct{}{}
							goto inlineDone
						}
					}
				}
				provider, buildErr := sdkaccess.BuildProvider(inline, &newCfg.SDKConfig)
				if buildErr != nil {
					return nil, nil, nil, nil, buildErr
				}
				if _, existed := existingMap[key]; existed {
					appendChange(&updated, key)
				} else if _, hadOld := oldCfgMap[key]; hadOld {
					appendChange(&updated, key)
				} else {
					appendChange(&added, key)
				}
				result = append(result, provider)
				finalIDs[key] = struct{}{}
			}
		}
	inlineDone:
	}

	removedSet := make(map[string]struct{})
	for id := range existingMap {
		if _, ok := finalIDs[id]; !ok {
			if isInlineProvider(id) {
				continue
			}
			removedSet[id] = struct{}{}
		}
	}

	removed = make([]string, 0, len(removedSet))
	for id := range removedSet {
		removed = append(removed, id)
	}

	sort.Strings(added)
	sort.Strings(updated)
	sort.Strings(removed)

	return result, added, updated, removed, nil
}

// ApplyAccessProviders reconciles the configured access providers against the
// currently registered providers and updates the manager. It logs a concise
// summary of the detected changes and returns whether any provider changed.
func ApplyAccessProviders(manager *sdkaccess.Manager, oldCfg, newCfg *config.Config) (bool, error) {
	if manager == nil || newCfg == nil {
		return false, nil
	}

	existing := manager.Providers()
	providers, added, updated, removed, err := ReconcileProviders(oldCfg, newCfg, existing)
	if err != nil {
		log.Errorf("failed to reconcile request auth providers: %v", err)
		return false, fmt.Errorf("reconciling access providers: %w", err)
	}

	manager.SetProviders(providers)

	if len(added)+len(updated)+len(removed) > 0 {
		log.Debugf("auth providers reconciled (added=%d updated=%d removed=%d)", len(added), len(updated), len(removed))
		log.Debugf("auth providers changes details - added=%v updated=%v removed=%v", added, updated, removed)
		return true, nil
	}

	log.Debug("auth providers unchanged after config update")
	return false, nil
}

func accessProviderMap(cfg *config.Config) map[string]*sdkConfig.AccessProvider {
	result := make(map[string]*sdkConfig.AccessProvider)
	if cfg == nil {
		return result
	}
	for i := range cfg.Access.Providers {
		providerCfg := &cfg.Access.Providers[i]
		if providerCfg.Type == "" {
			continue
		}
		key := providerIdentifier(providerCfg)
		if key == "" {
			continue
		}
		result[key] = providerCfg
	}
	if len(result) == 0 && len(cfg.APIKeys) > 0 {
		if provider := sdkConfig.MakeInlineAPIKeyProvider(cfg.APIKeys); provider != nil {
			if key := providerIdentifier(provider); key != "" {
				result[key] = provider
			}
		}
	}
	return result
}

func collectProviderEntries(cfg *config.Config) []*sdkConfig.AccessProvider {
	entries := make([]*sdkConfig.AccessProvider, 0, len(cfg.Access.Providers))
	for i := range cfg.Access.Providers {
		providerCfg := &cfg.Access.Providers[i]
		if providerCfg.Type == "" {
			continue
		}
		if key := providerIdentifier(providerCfg); key != "" {
			entries = append(entries, providerCfg)
		}
	}
	if len(entries) == 0 && len(cfg.APIKeys) > 0 {
		if inline := sdkConfig.MakeInlineAPIKeyProvider(cfg.APIKeys); inline != nil {
			entries = append(entries, inline)
		}
	}
	return entries
}

func providerIdentifier(provider *sdkConfig.AccessProvider) string {
	if provider == nil {
		return ""
	}
	if name := strings.TrimSpace(provider.Name); name != "" {
		return name
	}
	typ := strings.TrimSpace(provider.Type)
	if typ == "" {
		return ""
	}
	if strings.EqualFold(typ, sdkConfig.AccessProviderTypeConfigAPIKey) {
		return sdkConfig.DefaultAccessProviderName
	}
	return typ
}

func providerConfigEqual(a, b *sdkConfig.AccessProvider) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	if !strings.EqualFold(strings.TrimSpace(a.Type), strings.TrimSpace(b.Type)) {
		return false
	}
	if strings.TrimSpace(a.SDK) != strings.TrimSpace(b.SDK) {
		return false
	}
	if !stringSetEqual(a.APIKeys, b.APIKeys) {
		return false
	}
	if len(a.Config) != len(b.Config) {
		return false
	}
	if len(a.Config) > 0 && !reflect.DeepEqual(a.Config, b.Config) {
		return false
	}
	return true
}

func stringSetEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return true
	}
	seen := make(map[string]int, len(a))
	for _, val := range a {
		seen[val]++
	}
	for _, val := range b {
		count := seen[val]
		if count == 0 {
			return false
		}
		if count == 1 {
			delete(seen, val)
		} else {
			seen[val] = count - 1
		}
	}
	return len(seen) == 0
}
