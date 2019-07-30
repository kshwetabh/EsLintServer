1. To compile proto buffer for Go (agents client) use the below command
   ```shell
   C:\Dev\GoProgramming\src\github.com\ksfnu\eslint_server>protoc -I proto/ proto/eslintmessage.proto --go_out=plugins=grpc:agents/agent proto/eslintmessage.proto
   ```
2. 