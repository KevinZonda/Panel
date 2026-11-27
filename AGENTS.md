开发规范 @spec/go.md.

## 验证命令

- 格式化：`gofmt -w cmd models shared lib domain`
- 测试：`go test ./...`
- 静态检查：`go vet ./...`
- 构建：`go build -o bin/panel ./cmd/panel`

## 初始化配置

- 主机安装运行 `panel init`，默认写入 `/etc/kvzd-panel/config.json`。也可用 `panel init --config ./config.json`，并在向导内指定可写的数据目录
- 按向导输出的命令启动 Panel。配置和数据目录需要归 Panel 的运行账号所有，或由部署时设置相应权限

## 开发环境（Docker）

- 新数据卷先运行 `docker compose run --build --rm panel init`，按提示创建 `/var/lib/kvzd-panel/config.json`，配置保存在 `panel-data` 卷中
- `docker compose up --build` 启动完整开发环境：单容器内含 panel + nginx
- panel UI/API 在 `127.0.0.1:8080`，nginx demo 站点在 `127.0.0.1:8081`
- 自动化启动可以通过 `PANEL_BOOTSTRAP_ADMIN_PASSWORD_HASH` 提供初始管理员的 bcrypt 哈希，可用 `htpasswd -nBC 10 admin` 生成
- 旧数据卷继续使用数据库里的管理员账号。曾使用默认密码的账号应在面板里修改密码
- 容器内无 systemd，nginx 使用 `control_mode: binary`（`nginx -s` 信号控制）
- 本机 macOS 拉取镜像如遇到 `auth.docker.io` 网络问题，可用镜像源拉取后 `docker tag` 成标准名称

## nginx 控制模式

- `nginx.control_mode`：`systemd`（默认，生产用 `systemctl`）或 `binary`（`nginx`/`nginx -s quit/reload`，无需 init 系统）
- binary 模式 restart = quit + 等进程退出 + start
