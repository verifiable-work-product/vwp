module github.com/verifiable-work-product/vwp/backend

go 1.23

toolchain go1.24.0

require github.com/verifiable-work-product/vwp/sdk/go v0.0.0

require (
	github.com/santhosh-tekuri/jsonschema/v6 v6.0.2 // indirect
	golang.org/x/text v0.14.0 // indirect
)

replace github.com/verifiable-work-product/vwp/sdk/go => ../sdk/go
