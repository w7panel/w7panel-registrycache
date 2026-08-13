package logic

import "encoding/json"

type CommonRegistrySource struct {
	ServerURL string `json:"server_url"`
}

type CommonPageSetting struct {
	Markdown      string `json:"markdown"`
	ICPNumber     string `json:"icp_number"`
	ICPLink       string `json:"icp_link"`
	PoliceNumber  string `json:"police_number"`
	PoliceLink    string `json:"police_link"`
	Copyright     string `json:"copyright"`
	CopyrightLink string `json:"copyright_link"`
}

type CommonExtra struct {
	PageSetting *CommonPageSetting `json:"page_setting,omitempty"`
}

type CommonRegistryCacheSetting struct {
	OriginRegistry *CommonRegistrySource `json:"origin_registry,omitempty"`
	Extra          *CommonExtra          `json:"extra,omitempty"`
}

func BuildCommonRegistryCacheList(list map[string]*RegistryCacheSetting) map[string]CommonRegistryCacheSetting {
	// 公开接口只返回首页所需字段，避免缓存仓库凭据、源仓库配置、代理配置、
	// 缓存规则以及后续新增的内部字段被意外暴露。
	commonList := make(map[string]CommonRegistryCacheSetting)
	for group, setting := range list {
		if group == globalSettingGroup {
			commonList[group] = CommonRegistryCacheSetting{
				Extra: &CommonExtra{
					PageSetting: commonPageSettingFromExtra(setting.Extra),
				},
			}
			continue
		}

		commonSetting := CommonRegistryCacheSetting{}
		if setting.OriginRegistry.ServerUrl != "" {
			commonSetting.OriginRegistry = &CommonRegistrySource{
				ServerURL: setting.OriginRegistry.ServerUrl,
			}
		}
		commonList[group] = commonSetting
	}
	return commonList
}

func commonPageSettingFromExtra(extra map[string]interface{}) *CommonPageSetting {
	value, exists := extra["page_setting"]
	if !exists {
		return nil
	}

	content, err := json.Marshal(value)
	if err != nil {
		return nil
	}
	pageSetting := CommonPageSetting{}
	if err = json.Unmarshal(content, &pageSetting); err != nil {
		return nil
	}
	return &pageSetting
}
