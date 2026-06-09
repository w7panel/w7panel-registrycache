package controller

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"gitee.com/we7coreteam/w7-registry-cache/app/application/logic"
	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
)

var manifestRegex = regexp.MustCompile(`^/v2/(?P<image_name>.+?)/manifests/(?P<image_reference>[^/]+)$`)
var blobRegex = regexp.MustCompile(`^/v2/(?P<image_name>.+?)/blobs/(?P<image_reference>[^/]+)$`)
var blobUploadProxyRegex = regexp.MustCompile(`^/v2/(?P<image_name>.+?)(?P<suffix>/blobs/uploads(?:/.*)?)$`)
var manifestPushProxyRegex = regexp.MustCompile(`^/v2/(?P<image_name>.+?)(?P<suffix>/manifests/[^/]+)$`)
var authenticateRealmRegex = regexp.MustCompile(`realm="([^"]+)"`)
var repositoryScopeRegex = regexp.MustCompile(`^repository:([^:]+):(.+)$`)

type ParamsValidate struct {
	ImageName      string `mapstructure:"image_name" binding:"required"`
	ImageReference string `mapstructure:"image_reference" binding:"required"`
}

func etagMatch(r *http.Request, etag string) bool {
	for _, headerVal := range r.Header["If-None-Match"] {
		if headerVal == etag || headerVal == fmt.Sprintf(`"%s"`, etag) { // allow quoted or unquoted
			return true
		}
	}
	return false
}

func parseBlobRange(rangeHeader string, blobSize int64) (start, end int64, partial bool, err error) {
	if rangeHeader == "" {
		return 0, blobSize - 1, false, nil
	}
	if blobSize <= 0 {
		return 0, 0, false, fmt.Errorf("invalid range for empty blob")
	}

	const prefix = "bytes="
	if !strings.HasPrefix(rangeHeader, prefix) {
		return 0, 0, false, fmt.Errorf("unsupported range unit")
	}

	spec := strings.TrimSpace(strings.TrimPrefix(rangeHeader, prefix))
	if spec == "" || strings.Contains(spec, ",") {
		return 0, 0, false, fmt.Errorf("unsupported range")
	}

	parts := strings.SplitN(spec, "-", 2)
	if len(parts) != 2 {
		return 0, 0, false, fmt.Errorf("invalid range")
	}

	if parts[0] == "" {
		suffixLen, err := strconv.ParseInt(parts[1], 10, 64)
		if err != nil || suffixLen <= 0 {
			return 0, 0, false, fmt.Errorf("invalid suffix range")
		}
		if suffixLen >= blobSize {
			return 0, blobSize - 1, true, nil
		}
		return blobSize - suffixLen, blobSize - 1, true, nil
	}

	start, err = strconv.ParseInt(parts[0], 10, 64)
	if err != nil || start < 0 || start >= blobSize {
		return 0, 0, false, fmt.Errorf("invalid range start")
	}

	if parts[1] == "" {
		return start, blobSize - 1, true, nil
	}

	end, err = strconv.ParseInt(parts[1], 10, 64)
	if err != nil || end < start {
		return 0, 0, false, fmt.Errorf("invalid range end")
	}
	if end >= blobSize {
		end = blobSize - 1
	}

	return start, end, true, nil
}

func extractParams(re *regexp.Regexp, matches []string) *ParamsValidate {
	paramsMap := map[string]string{}
	for i, name := range re.SubexpNames() {
		if i > 0 && i <= len(matches) && name != "" {
			paramsMap[name] = matches[i]
		}
	}
	if paramsMap == nil {
		return nil
	}

	params := &ParamsValidate{}
	err := mapstructure.Decode(paramsMap, params)
	if err != nil {
		return nil
	}

	return params
}

func buildAuthProxyRealm(ctx *gin.Context, upstreamRealm string) string {
	values := url.Values{}
	values.Set("upstream", upstreamRealm)
	return fmt.Sprintf("%s://%s/oauth/auth-proxy?%s", requestScheme(ctx), ctx.Request.Host, values.Encode())
}

func rewriteAuthenticateRealm(ctx *gin.Context, authenticate string) string {
	if authenticate == "" {
		return authenticate
	}

	matches := authenticateRealmRegex.FindStringSubmatch(authenticate)
	if len(matches) != 2 || matches[1] == "" {
		return authenticate
	}

	return authenticateRealmRegex.ReplaceAllString(authenticate, `realm="`+buildAuthProxyRealm(ctx, matches[1])+`"`)
}

func rewriteRepositoryScope(scope string, namespacePrefix string) string {
	if scope == "" {
		return scope
	}

	matches := repositoryScopeRegex.FindStringSubmatch(strings.TrimSpace(scope))
	if len(matches) != 3 {
		return scope
	}

	targetRepository := logic.Storage{}.RebuildImageName(matches[1], namespacePrefix)
	if targetRepository == matches[1] {
		return scope
	}

	return "repository:" + targetRepository + ":" + matches[2]
}

func rewriteBlobUploadRequestURI(requestURI string, namespacePrefix string) string {
	path := requestURI
	rawQuery := ""
	if idx := strings.Index(requestURI, "?"); idx >= 0 {
		path = requestURI[:idx]
		rawQuery = requestURI[idx:]
	}

	matches := blobUploadProxyRegex.FindStringSubmatch(path)
	if matches == nil {
		return requestURI
	}

	params := extractParams(blobUploadProxyRegex, matches)
	if params == nil {
		return requestURI
	}

	suffixIndex := blobUploadProxyRegex.SubexpIndex("suffix")
	if suffixIndex <= 0 || suffixIndex >= len(matches) {
		return requestURI
	}

	targetRepository := logic.Storage{}.RebuildImageName(params.ImageName, namespacePrefix)
	if targetRepository == params.ImageName {
		return rewriteMountFromQuery(requestURI, namespacePrefix)
	}

	return rewriteMountFromQuery("/v2/"+targetRepository+matches[suffixIndex]+rawQuery, namespacePrefix)
}

func rewriteMountFromQuery(requestURI string, namespacePrefix string) string {
	path := requestURI
	rawQuery := ""
	if idx := strings.Index(requestURI, "?"); idx >= 0 {
		path = requestURI[:idx]
		rawQuery = requestURI[idx+1:]
	}
	if rawQuery == "" {
		return requestURI
	}

	values, err := url.ParseQuery(rawQuery)
	if err != nil {
		return requestURI
	}

	from := values.Get("from")
	if from == "" {
		return requestURI
	}

	rewrittenFrom := logic.Storage{}.RebuildImageName(from, namespacePrefix)
	if rewrittenFrom == from {
		return requestURI
	}
	values.Set("from", rewrittenFrom)
	return path + "?" + values.Encode()
}

func rewriteManifestPushRequestURI(requestURI string, namespacePrefix string) string {
	path := requestURI
	rawQuery := ""
	if idx := strings.Index(requestURI, "?"); idx >= 0 {
		path = requestURI[:idx]
		rawQuery = requestURI[idx:]
	}

	matches := manifestPushProxyRegex.FindStringSubmatch(path)
	if matches == nil {
		return requestURI
	}

	params := extractParams(manifestPushProxyRegex, matches)
	if params == nil {
		return requestURI
	}

	suffixIndex := manifestPushProxyRegex.SubexpIndex("suffix")
	if suffixIndex <= 0 || suffixIndex >= len(matches) {
		return requestURI
	}

	targetRepository := logic.Storage{}.RebuildImageName(params.ImageName, namespacePrefix)
	if targetRepository == params.ImageName {
		return requestURI
	}

	return "/v2/" + targetRepository + matches[suffixIndex] + rawQuery
}

func rewritePushRequestURI(method string, requestURI string, namespacePrefix string) string {
	switch method {
	case http.MethodPost, http.MethodPatch:
		return rewriteBlobUploadRequestURI(requestURI, namespacePrefix)
	case http.MethodPut:
		rewritten := rewriteBlobUploadRequestURI(requestURI, namespacePrefix)
		if rewritten != requestURI {
			return rewritten
		}
		return rewriteManifestPushRequestURI(requestURI, namespacePrefix)
	default:
		return requestURI
	}
}

func rewritePushLocation(ctx *gin.Context, originalRequestURI string, rewrittenRequestURI string, location string) string {
	if location == "" {
		return location
	}

	locationURL, err := url.Parse(location)
	if err != nil {
		return location
	}
	locationURL.Scheme = requestScheme(ctx)
	locationURL.Host = ctx.Request.Host

	if originalRequestURI == rewrittenRequestURI {
		return locationURL.String()
	}

	originalPath := originalRequestURI
	rewrittenPath := rewrittenRequestURI
	if idx := strings.Index(originalPath, "?"); idx >= 0 {
		originalPath = originalPath[:idx]
	}
	if idx := strings.Index(rewrittenPath, "?"); idx >= 0 {
		rewrittenPath = rewrittenPath[:idx]
	}

	originalMatches := blobUploadProxyRegex.FindStringSubmatch(originalPath)
	rewrittenMatches := blobUploadProxyRegex.FindStringSubmatch(rewrittenPath)
	if originalMatches != nil && rewrittenMatches != nil {
		originalParams := extractParams(blobUploadProxyRegex, originalMatches)
		rewrittenParams := extractParams(blobUploadProxyRegex, rewrittenMatches)
		if originalParams == nil || rewrittenParams == nil {
			return locationURL.String()
		}
		return rewriteLocationRepository(locationURL, originalParams.ImageName, rewrittenParams.ImageName)
	}

	originalMatches = manifestPushProxyRegex.FindStringSubmatch(originalPath)
	rewrittenMatches = manifestPushProxyRegex.FindStringSubmatch(rewrittenPath)
	if originalMatches != nil && rewrittenMatches != nil {
		originalParams := extractParams(manifestPushProxyRegex, originalMatches)
		rewrittenParams := extractParams(manifestPushProxyRegex, rewrittenMatches)
		if originalParams == nil || rewrittenParams == nil {
			return locationURL.String()
		}
		return rewriteLocationRepository(locationURL, originalParams.ImageName, rewrittenParams.ImageName)
	}

	return locationURL.String()
}

func rewriteLocationRepository(locationURL *url.URL, originalRepository string, rewrittenRepository string) string {
	rewrittenRepository = strings.Trim(rewrittenRepository, "/")
	originalRepository = strings.Trim(originalRepository, "/")
	if rewrittenRepository == "" || originalRepository == "" || rewrittenRepository == originalRepository {
		return locationURL.String()
	}

	rewrittenPrefix := "/v2/" + rewrittenRepository + "/"
	originalPrefix := "/v2/" + originalRepository + "/"
	if !strings.HasPrefix(locationURL.Path, rewrittenPrefix) {
		return locationURL.String()
	}

	locationURL.Path = originalPrefix + strings.TrimPrefix(locationURL.Path, rewrittenPrefix)
	return locationURL.String()
}

func requestScheme(ctx *gin.Context) string {
	if proto := ctx.Request.Header.Get("X-Forwarded-Proto"); proto != "" {
		return proto
	}
	if ctx.Request.TLS != nil {
		return "https"
	}
	return "http"
}
