
gen:
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@protoc --go_out=./proto --proto_path=./proto --go_opt=paths=source_relative ./proto/*.proto


