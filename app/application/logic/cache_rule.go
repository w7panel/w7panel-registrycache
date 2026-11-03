package logic

import (
	"fmt"
	"math"
	"regexp"
	"strings"
	"time"
)

type RepositoryCacheRule struct {
	CacheType        string   `json:"cache_type"`
	RepositoryName   []string `json:"repository_name"`
	CacheTtl         int64    `json:"cache_ttl"`
	Enable           bool     `json:"enable"`
	Deny             bool     `json:"deny"`
	Weight           int      `json:"weight"`
	AssignRegistry   string   `json:"assign_registry"`
	DistributedCache bool     `json:"distributed_cache"`
}

type CacheRule struct {
	logic
}

func (l CacheRule) MatchRepositoryCacheRule(repositoryName string, rules []RepositoryCacheRule) (*RepositoryCacheRule, error) {
	if rules == nil || len(rules) == 0 {
		return nil, nil
	}

	var defaultRule RepositoryCacheRule
	for _, rule := range rules {
		switch rule.CacheType {
		case "fix":
			for _, rpath := range rule.RepositoryName {
				if repositoryName == rpath {
					return &rule, nil
				}
			}
		case "prefix":
			for _, rpath := range rule.RepositoryName {
				if strings.HasPrefix(repositoryName, rpath) {
					return &rule, nil
				}
			}
		case "suffix":
			for _, rpath := range rule.RepositoryName {
				if rpath != "" && strings.HasSuffix(repositoryName, rpath) {
					return &rule, nil
				}
			}
		case "regex":
			for _, rpath := range rule.RepositoryName {
				regex, err := regexp.Compile(rpath)
				if err != nil {
					return nil, fmt.Errorf("invalid cache rule pattern %q: %w", rpath, err)
				}
				if regex.MatchString(repositoryName) {
					return &rule, nil
				}
			}
		case "all":
			defaultRule = rule // 保存匹配所有文件的规则
		default:
			fmt.Printf("Unknown cacheType: %s\n", rule.CacheType)
			return nil, fmt.Errorf("Unknown cacheType: %s", rule.CacheType)
		}
	}

	// 如果没有找到其他匹配规则，返回默认规则
	return &defaultRule, nil
}

// Deny takes precedence over Enable for both old and new configurations.
func (r RepositoryCacheRule) CacheEnabled() bool {
	return r.Enable && !r.Deny
}

func (l CacheRule) RepositoryAccessDenied(repository string, rules []RepositoryCacheRule) (bool, error) {
	rule, err := l.MatchRepositoryCacheRule(repository, rules)
	return rule != nil && rule.Deny, err
}

func cacheTTLDuration(ttl int64) (time.Duration, bool) {
	if ttl <= 0 || ttl > math.MaxInt64/int64(time.Minute) {
		return 0, false
	}
	return time.Duration(ttl) * time.Minute, true
}
