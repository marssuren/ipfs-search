package config

// Queue 结构体表示单个队列的配置。
type Queue struct {
	Name string `yaml:"name"` // 队列的名称。
}

// Queues 结构体表示我们正在使用的各种队列。
type Queues struct {
	Files       Queue `yaml:"files"`       // 已知是文件的资源队列。
	Directories Queue `yaml:"directories"` // 已知是目录的资源队列。
	Hashes      Queue `yaml:"hashes"`      // 类型未知的资源队列。
}

// QueuesDefaults 函数返回默认的队列配置。
func QueuesDefaults() Queues {
	return Queues{
		Files: Queue{
			Name: "files", // 文件队列的默认名称。
		},
		Directories: Queue{
			Name: "directories", // 目录队列的默认名称。
		},
		Hashes: Queue{
			Name: "hashes", // 类型未知资源队列的默认名称。
		},
	}
}
