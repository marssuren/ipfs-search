package config

/*
Workers 结构体包含了工作池的配置。

它完全包含在这里，以避免循环导入，因为 worker 包使用了中心的 Config 结构体。
*/
type Workers struct {
	HashWorkers       int `yaml:"hash_workers" env:"HASH_WORKERS"`                           // 哈希计算工人的数量。
	FileWorkers       int `yaml:"file_workers" env:"FILE_WORKERS"`                           // 文件处理工人的数量。
	DirectoryWorkers  int `yaml:"directory_workers" env:"DIRECTORY_WORKERS"`                 // 目录处理工人的数量。
	MaxIPFSConns      int `yaml:"ipfs_max_connections" env:"IPFS_MAX_CONNECTIONS"`           // 最大 IPFS 连接数。
	MaxExtractorConns int `yaml:"extractor_max_connections" env:"EXTRACTOR_MAX_CONNECTIONS"` // 最大提取器连接数。
}

// WorkersDefaults 函数返回工作池的默认配置。
func WorkersDefaults() Workers {
	return Workers{
		HashWorkers:       70,   // 哈希计算工人的默认数量。
		FileWorkers:       120,  // 文件处理工人的默认数量。
		DirectoryWorkers:  70,   // 目录处理工人的默认数量。
		MaxIPFSConns:      1000, // 最大 IPFS 连接数的默认值。
		MaxExtractorConns: 100,  // 最大提取器连接数的默认值。
	}
}
