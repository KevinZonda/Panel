开发规范 @spec/go.md.

## 验证命令

- 格式化：`gofmt -w cmd models shared lib domain`
- 测试：`go test ./...`
- 静态检查：`go vet ./...`
- 构建：`go build -o bin/panel ./cmd/panel`

## 开发环境（Docker）

- `docker compose up --build` 启动完整开发环境：单容器内含 panel + nginx
- panel UI/API 在 `localhost:8080`，nginx demo 站点在 `localhost:8081`
- 默认 dev 账号 `admin` / `admin123456`（可用 `PANEL_BOOTSTRAP_ADMIN_PASSWORD_HASH` 覆盖，仅限本地；可用 `htpasswd -nBC 10 admin` 生成 hash）
- 容器内无 systemd，nginx 使用 `control_mode: binary`（`nginx -s` 信号控制）
- 本机 macOS 拉取镜像如遇到 `auth.docker.io` 网络问题，可用镜像源拉取后 `docker tag` 成标准名称

## nginx 控制模式

- `nginx.control_mode`：`systemd`（默认，生产用 `systemctl`）或 `binary`（`nginx`/`nginx -s quit/reload`，无需 init 系统）
- binary 模式 restart = quit + 等进程退出 + start
