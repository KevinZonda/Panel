# 权限矩阵

Panel 只有两类角色：`viewer` 与 `admin`。UI 与 API 必须使用同一套后端授权语义；模板隐藏控件只用于展示，不作为权限边界。

| 能力 | viewer | admin | 说明 |
| --- | --- | --- | --- |
| 登录、退出、查看自己的会话状态 | 是 | 是 | 未认证用户只能访问登录接口/页面 |
| 修改自己的密码 | 是 | 是 | 需要旧密码；成功后撤销其他会话 |
| Dashboard 与系统状态 | 是 | 是 | 只读运行状态 |
| nginx 运行状态与配置测试 | 是 | 是 | 只读或验证操作 |
| nginx site 列表与内容读取 | 是 | 是 | site 内容可能包含运维配置，但不含证书私钥本体 |
| nginx site 创建、修改、删除 | 否 | 是 | 写操作必须 admin |
| nginx start/stop/reload/restart | 否 | 是 | 控制操作必须 admin |
| frp systemd 服务状态 | 是 | 是 | viewer 可查看服务名、状态、PID、关联配置文件名 |
| frp 配置元数据与内容读取 | 否 | 是 | TOML 可能包含 token 等秘密 |
| frp 配置创建、修改、删除 | 否 | 是 | 写操作必须 admin |
| frp 服务 start/stop/restart | 否 | 是 | 控制操作必须 admin |
| 证书列表、详情与公开证书文件 | 是 | 是 | public 的 `pub`；selfsign 的 `server.crt`、`ca.crt` |
| 证书私钥读取 | 否 | 是 | public 的 `priv`；selfsign 的 `server.key` |
| 证书目录中的未知/其他文件 | 否 | 是 | 只有明确公开的证书文件才对 viewer 开放 |
| 证书上传、生成、替换、删除 | 否 | 是 | 写操作必须 admin |
| 用户列表、创建、修改、删除、重置密码 | 否 | 是 | 用户管理必须 admin |
| Web Terminal | 否 | 是 | Terminal 等价于宿主机 shell，必须 admin |

## 强制规则

1. 所有 mutating UI/API 路由必须 admin，除非目标是用户自己的密码。
2. viewer 的读取权限只覆盖非秘密运行状态与公开证书材料。
3. cert file API/UI 使用公开文件白名单；私钥和未知文件必须在 handler 内做二次角色检查，不能只依赖路由级 admin middleware。
4. 模板可以按角色隐藏链接和表单，但后端仍必须独立拒绝未授权请求。
5. 新增能力前先更新本矩阵，再实现 UI/API；如果两端语义不同，以后端拒绝为准。
