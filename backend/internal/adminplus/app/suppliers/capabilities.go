package suppliers

import (
	adminplusdomain "github.com/Wei-Shaw/sub2api/internal/adminplus/domain"
)

func buildSupplierCapabilities(in *adminplusdomain.Supplier) []adminplusdomain.SupplierCapability {
	if in == nil {
		return nil
	}
	switch in.Type {
	case adminplusdomain.SupplierTypeSub2API:
		return []adminplusdomain.SupplierCapability{
			sessionCapability("profile_balance", "Profile/余额", "provider_router", "Sub2API 用户 Profile 与余额读取已接入 Provider Router，需要有效供应商会话。"),
			sessionCapability("groups", "分组", "provider_router", "Sub2API 可读取供应商分组，用于后续 Key 和本地账号绑定。"),
			sessionCapability("rates", "费率", "provider_router", "Sub2API 可读取渠道费率，用于发现低倍率分组。"),
			sessionCapability("announcements", "公告", "provider_router", "Sub2API 可读取充值与站内公告，用于运营提醒。"),
			sessionCapability("usage_costs", "用量成本", "provider_router", "Sub2API 可读取供应商用量成本，用于本地成本核对。"),
			sessionCapability("funding", "充值流水", "provider_router", "Sub2API 可读取充值流水，用于成本台账。"),
			sessionCapability("entitlements", "兑换权益", "provider_router", "Sub2API 可读取兑换或权益记录，用于补齐成本来源。"),
			sessionCapability("keys", "Key 管理", "provider_router", "Sub2API 可创建和重命名第三方 Key，但执行前仍需会话确认。"),
			sessionCapability("channel_monitors", "渠道监控", "provider_router", "Sub2API 可读取供应商侧渠道监控，用于运营观测。"),
			readonlyCapability(in, "local_runtime_observation", "本地运行态", "sub2api_readonly", "通过本地 Sub2API 只读 PG/Redis 观测账号运行态，不接管调度。"),
		}
	case adminplusdomain.SupplierTypeNewAPI:
		return []adminplusdomain.SupplierCapability{
			sessionCapability("profile_balance", "Profile/余额", "provider_router", "New API 用户 Profile 与余额读取已接入 Provider Router，需要有效供应商会话。"),
			sessionCapability("groups", "分组", "provider_router", "New API 可读取分组，用于后续 Key 和本地账号绑定。"),
			plannedCapability("rates", "费率", "provider_router", "New API 费率读取边界已在路由层显式保留，当前实现尚未接入。"),
			plannedCapability("announcements", "公告", "provider_router", "New API 公告读取边界已在路由层显式保留，当前实现尚未接入。"),
			sessionCapability("usage_costs", "用量成本", "provider_router", "New API 可读取用量成本，用于本地成本核对。"),
			sessionCapability("funding", "充值流水", "provider_router", "New API 可读取充值流水，用于成本台账。"),
			sessionCapability("entitlements", "兑换权益", "provider_router", "New API 可读取兑换或权益记录，用于补齐成本来源。"),
			sessionCapability("keys", "Key 管理", "provider_router", "New API 可创建和重命名第三方 Key，但执行前仍需会话确认。"),
			sessionCapability("channel_monitors", "渠道监控", "provider_router", "New API 可读取 Pulse 渠道监控，用于运营观测。"),
			readonlyCapability(in, "local_runtime_observation", "本地运行态", "sub2api_readonly", "通过本地 Sub2API 只读 PG/Redis 观测账号运行态，不接管调度。"),
		}
	case adminplusdomain.SupplierTypeOpenAI, adminplusdomain.SupplierTypeAnthropic, adminplusdomain.SupplierTypeGemini:
		return []adminplusdomain.SupplierCapability{
			availableCapability("health_probe", "健康探测", "local_sub2api_account", "通过本地 Sub2API 账号做源站健康探测，不接管源站凭据。"),
			readonlyCapability(in, "local_runtime_observation", "本地运行态", "sub2api_readonly", "通过本地 Sub2API 只读 PG/Redis 观测账号运行态，不接管调度。"),
			unsupportedCapability("profile_balance", "Profile/余额", "source_provider", "源站账号能力不由 SuperLLM 直接读取，保持由 Sub2API 与本地账号绑定承载。"),
			unsupportedCapability("keys", "Key 管理", "source_provider", "源站 Key 创建不在 SuperLLM 供应商运营边界内。"),
		}
	case adminplusdomain.SupplierTypeBrowserOnly:
		return []adminplusdomain.SupplierCapability{
			sessionCapability("browser_session", "浏览器会话", "browser", "浏览器型供应商用于采集或导入会话，后续能力取决于识别出的供应商类型。"),
			plannedCapability("profile_balance", "Profile/余额", "provider_router", "需要先识别具体供应商类型后再读取 Profile 与余额。"),
			plannedCapability("groups", "分组", "provider_router", "需要先识别具体供应商类型后再读取分组。"),
		}
	default:
		return []adminplusdomain.SupplierCapability{
			sessionCapability("browser_session", "浏览器会话", "browser", "自定义供应商可先通过浏览器会话采集事实，再决定是否接入 Provider Adapter。"),
			readonlyCapability(in, "local_runtime_observation", "本地运行态", "sub2api_readonly", "通过本地 Sub2API 只读 PG/Redis 观测账号运行态，不接管调度。"),
			plannedCapability("profile_balance", "Profile/余额", "provider_router", "自定义供应商尚未接入标准 Profile 与余额读取。"),
			plannedCapability("groups", "分组", "provider_router", "自定义供应商尚未接入标准分组读取。"),
			plannedCapability("keys", "Key 管理", "provider_router", "自定义供应商尚未接入标准 Key 管理。"),
		}
	}
}

func sessionCapability(key string, label string, source string, description string) adminplusdomain.SupplierCapability {
	return supplierCapability(key, label, adminplusdomain.SupplierCapabilityStatusNeedsSession, source, description)
}

func readonlyCapability(in *adminplusdomain.Supplier, key string, label string, source string, description string) adminplusdomain.SupplierCapability {
	status := adminplusdomain.SupplierCapabilityStatusNeedsReadonlyDB
	if in != nil && (in.Credential.PostgresConfigured || in.Credential.RedisConfigured) {
		status = adminplusdomain.SupplierCapabilityStatusAvailable
	}
	return supplierCapability(key, label, status, source, description)
}

func availableCapability(key string, label string, source string, description string) adminplusdomain.SupplierCapability {
	return supplierCapability(key, label, adminplusdomain.SupplierCapabilityStatusAvailable, source, description)
}

func unsupportedCapability(key string, label string, source string, description string) adminplusdomain.SupplierCapability {
	return supplierCapability(key, label, adminplusdomain.SupplierCapabilityStatusUnsupported, source, description)
}

func plannedCapability(key string, label string, source string, description string) adminplusdomain.SupplierCapability {
	return supplierCapability(key, label, adminplusdomain.SupplierCapabilityStatusPlanned, source, description)
}

func supplierCapability(key string, label string, status adminplusdomain.SupplierCapabilityStatus, source string, description string) adminplusdomain.SupplierCapability {
	return adminplusdomain.SupplierCapability{
		Key:         key,
		Label:       label,
		Status:      status,
		Source:      source,
		Description: description,
	}
}
