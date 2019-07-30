1. To install custom HMS Plugins:
    ```
    npm install -S ./eslint
    ```
2. To compile proto buffer for Go (EsLintClient client) use the below command
   ```shell
   C:\Dev\GoProgramming\src\github.com\ksfnu\eslint_server>protoc -I proto/ proto/eslintmessage.proto --go_out=plugins=grpc:EsLintClient/agent proto/eslintmessage.proto
   ```