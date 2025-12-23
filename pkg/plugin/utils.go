package plugin

func GetStringConfig(config map[string]interface{}, key string) string {
	if val, ok := config[key].(string); ok {
		return val
	}
	return ""
}
