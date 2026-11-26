开发规范 @spec/go.md.

## 验证命令

- 格式化：`gofmt -w cmd models shared lib domain`
- 测试：`go test ./...`
- 静态检查：`go vet ./...`
- 构建：`go build -o bin/panel ./cmd/panel`
