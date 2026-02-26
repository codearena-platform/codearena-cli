module github.com/codearena-platform/codearena-cli

go 1.25.5

require (
	github.com/codearena-platform/codearena-core v0.0.0
	github.com/google/uuid v1.6.0
	github.com/spf13/cobra v1.10.2
	google.golang.org/grpc v1.79.1
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/text v0.33.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260128011058-8636f8732409 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace github.com/codearena-platform/codearena-core => ../codearena-core
