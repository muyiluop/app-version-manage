package config

// DeployConfig 部署配置
type DeployConfig struct {
	// 服务器配置
	Server struct {
		Port int    `yaml:"port"`
		Host string `yaml:"host"`
	} `yaml:"server"`

	// 数据库配置
	Database struct {
		Type string `yaml:"type"`
		Path string `yaml:"path"`
	} `yaml:"database"`

	// JWT配置
	JWT struct {
		Secret     string `yaml:"secret"`
		ExpireTime int    `yaml:"expireTime"`
	} `yaml:"jwt"`

	// 存储配置
	Storage struct {
		Path string `yaml:"path"`
	} `yaml:"storage"`
}

// 加载部署配置
func LoadDeployConfig(configPath string) (*DeployConfig, error) {
	// TODO: 从配置文件加载
	config := &DeployConfig{}
	return config, nil
}
