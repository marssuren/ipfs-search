package types

import (
	"fmt"
)

// AnnotatedResource 用于给引用的资源添加额外的信息。
type AnnotatedResource struct {
	*Resource
	Source    SourceType `json:",omitempty"`
	Reference `json:",omitempty"`
	Stat      `json:",omitempty"`
}

// String 方法返回第一个引用的名称或 URI。
func (r *AnnotatedResource) String() string {
	if r.Reference.Name != "" {
		return fmt.Sprintf("%s (%s)", r.Reference.Name, r.URI())
	}

	return r.URI()
}
