# Template Repository
[![codecov](https://codecov.io/gh/everoute/template-repo/branch/main/graph/badge.svg)](https://codecov.io/gh/everoute/template-repo)
[![release](https://github.com/everoute/template-repo/actions/workflows/release.yaml/badge.svg)](https://github.com/everoute/template-repo/actions/workflows/release.yaml)

# Overview

Everoute Template Repository 是一个私有模板仓库，为减少仓库初始化的工作量，可使用此模板进行创建。

# Get started

使用此模板仓库创建仓库后，需要按照如下步骤配置仓库：

1. 将 Makefile 和 README.md 文件中的所有 `everoute/template-repo` 替换为新仓库名称。
2. 更新 README.md 内容，并为 codecov 链接增加 token 以正常显示 codecov badge.
3. 在设置中增加以下 secret:
    - `SLACK_WEBHOOK_URL`: 当 release 时需要通知到 slack 的 webhook url.
    - `CODECOV_TOKEN`: 用于上传 codecov 结果的 token.
