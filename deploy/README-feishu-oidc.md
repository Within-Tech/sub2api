# 飞书组织登录接入

飞书授权及组织限制由独立的 `Within-Tech/feishu-auth` 服务处理。
本分支基于线上版本 `073e92d17178a1ccdb0a27017f572f10c9c7ab62`，仅补充 OIDC 不依赖邮箱的首次注册路径。

接入时使用 `openid profile` scopes，不请求 email。新账号使用 issuer + subject 派生的保留地址作为内部唯一标识；该地址不是可收信邮箱。
只有 ID Token 验证开启，且邮箱验证、强制邮箱和邀请码要求均关闭时才启用此路径。
正常保留注册开关、后端模式、用户状态、额度发放、身份绑定和后续登录检查。
填写过的飞书企业邮箱不会被当作邮箱验证证明，也不会据此绑定已有邮箱账号。

构建时先在 `frontend/` 执行 `pnpm@9 install --frozen-lockfile` 和 `pnpm@9 run build`，再编译 backend：

```sh
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags embed -trimpath -o /tmp/sub2api ./cmd/server
```

`Dockerfile.oidc-overlay` 用于覆盖相同线上版本的后端二进制，保留原镜像运行环境。
部署必须先备份 compose 配置和原镜像标识，只替换 sub2api 服务镜像；无需删除数据库或数据目录。
