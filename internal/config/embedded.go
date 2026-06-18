package config

import _ "embed"

// defaultConfigYAML 是嵌入二进制的默认配置文件（default.yaml）。
// 启动时若本地 config.yaml 不存在，则释放此内容作为默认配置。
//
//go:embed default.yaml
var defaultConfigYAML []byte
