package blacklist

import (
	"fmt"
	"net/netip"
)

// Rule remains the persisted/API representation of a rule.
type Rule struct {
	Type    string `json:"type"`
	Pattern string `json:"pattern"`
	Enable  bool   `json:"enable"`
}

// Scope keeps client restrictions separate from background cache policy.
type Scope uint8

const (
	Request Scope = 1 << iota
	Cache
)

// Context is independent of HTTP and transfer implementations.
// ClientIP is the peer address; forwarded headers require trusted proxy handling.
type Context struct {
	Scope      Scope
	Repository string
	ClientIP   netip.Addr
}

// Matcher is immutable and must support concurrent calls.
type Matcher interface {
	Match(Context) bool
}

// Add each rule type with its compiler and applicable scopes. Implement the
// compiler and matcher in a dedicated file (for example ip.go).
var ruleTypes = map[string]ruleType{
	"namespace":  {Request | Cache, compileNamespace},
	"repository": {Request | Cache, compileRepository},
}

type ruleType struct {
	scopes  Scope
	compile func(string) (Matcher, error)
}

type compiledRule struct {
	rule    Rule
	index   int
	scopes  Scope
	matcher Matcher
}

// Engine is compiled once per loaded configuration and shared read-only.
type Engine struct{ rules []compiledRule }

// Decision identifies the matched rule for logging or future auditing.
type Decision struct {
	Denied    bool
	Rule      Rule
	RuleIndex int
	Err       error
}

func compileRules(rules []Rule, types map[string]ruleType) (*Engine, error) {
	engine := &Engine{}
	for i, rule := range rules {
		kind, exists := types[rule.Type]
		if !exists {
			return nil, fmt.Errorf("黑名单规则 %d: 不支持的黑名单类型: %s", i+1, rule.Type)
		}
		matcher, err := kind.compile(rule.Pattern)
		if err != nil {
			return nil, fmt.Errorf("黑名单规则 %d: %w", i+1, err)
		}
		if rule.Enable {
			engine.rules = append(engine.rules, compiledRule{rule, i, kind.scopes, matcher})
		}
	}
	return engine, nil
}

func Compile(rules []Rule) (*Engine, error) {
	return compileRules(rules, ruleTypes)
}

func Validate(rules []Rule) error {
	_, err := Compile(rules)
	return err
}

func (e *Engine) Evaluate(ctx Context) Decision {
	for _, rule := range e.rules {
		if rule.scopes&ctx.Scope != 0 && rule.matcher.Match(ctx) {
			return Decision{Denied: true, Rule: rule.rule, RuleIndex: rule.index}
		}
	}
	return Decision{RuleIndex: -1}
}
