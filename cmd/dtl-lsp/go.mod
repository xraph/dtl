module github.com/xraph/dtl/cmd/dtl-lsp

go 1.26.0

require (
	github.com/xraph/dtl v0.0.0
	github.com/xraph/langserver v0.0.0
)

require github.com/google/uuid v1.6.0 // indirect

replace github.com/xraph/dtl => ../..

replace github.com/xraph/langserver => ../../../langserver
