module CommandCenter

go 1.25.4

require (
	github.com/openbook/shared v0.0.0
	google.golang.org/grpc v1.64.0
	google.golang.org/protobuf v1.34.1
)

require (
	github.com/joho/godotenv v1.5.1 // indirect
	golang.org/x/net v0.25.0 // indirect
	golang.org/x/sys v0.32.0 // indirect
	golang.org/x/text v0.24.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240318140521-94a12d6c2237 // indirect
)

replace github.com/openbook/shared => ../shared
