module github.com/xraph/dtl/cmd/dtl-lsp

go 1.26.0

require (
	github.com/xraph/dtl v1.5.5
	github.com/xraph/langserver v1.0.0
)

require github.com/google/uuid v1.6.0 // indirect

replace github.com/xraph/dtl => ../..
