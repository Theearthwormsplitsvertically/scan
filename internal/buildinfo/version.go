// buildinfo 包暴露可由链接器参数注入的构建元数据。
package buildinfo

// 当未提供链接器参数时，这些默认值保证开发构建仍可被识别。
var (
	Version   = "dev"
	Commit    = "unknown"
	BuildTime = "unknown"
)
