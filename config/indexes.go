package config

// Index 结构体表示单个索引的配置。
type Index struct {
	Name   string // 索引的名称。
	Prefix string // 索引的前缀。
}

// Indexes 结构体表示我们正在使用的各种索引。
type Indexes struct {
	Files       Index `yaml:"files"`       // 文件索引的配置。
	Directories Index `yaml:"directories"` // 目录索引的配置。
	Invalids    Index `yaml:"invalids"`    // 无效条目索引的配置。
	Partials    Index `yaml:"partials"`    // 部分条目索引的配置。
}

// IndexesDefaults 函数返回默认的索引配置。
func IndexesDefaults() Indexes {
	return Indexes{
		Files: Index{
			Name:   "ipfs_files", // 文件索引的默认名称。
			Prefix: "f",          // 文件索引的默认前缀。
		},
		Directories: Index{
			Name:   "ipfs_directories", // 目录索引的默认名称。
			Prefix: "d",                // 目录索引的默认前缀。
		},
		Invalids: Index{
			Name:   "ipfs_invalids", // 无效条目索引的默认名称。
			Prefix: "i",             // 无效条目索引的默认前缀。
		},
		Partials: Index{
			Name:   "ipfs_partials", // 部分条目索引的默认名称。
			Prefix: "p",             // 部分条目索引的默认前缀。
		},
	}
}
