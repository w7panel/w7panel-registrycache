package logic

import (
	"fmt"
	"regexp"
	"strings"
)

type RepositoryCacheRule struct {
	CacheType      string   `json:"cache_type"`
	RepositoryName []string `json:"repository_name"`
	CacheTtl       int64    `json:"cache_ttl"`
	Enable         bool     `json:"enable"`
	Weight         int      `json:"weight"`
	AssignRegistry string   `json:"assign_registry"`
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
		case "regex":
			for _, rpath := range rule.RepositoryName {
				regex := regexp.MustCompile(rpath)
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
