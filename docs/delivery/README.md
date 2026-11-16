# 交付说明

## SaaS

- 从 `main` 持续集成。
- 构建 `api`、`worker`、`web` 镜像，使用 `sha-<commit>` 标签。
- 部署到 staging，完成冒烟/E2E 后人工审批晋级 production。
- 数据库迁移采用 expand/contract。

## 私有化

- 每季度从主线切 `release/YYYY.QN`。
- 使用 `v<version>-onprem` tag 触发离线包构建。
- 离线包包含前端、Go 二进制、迁移脚本、部署清单和文档。
- 正式交付前完成签名、SBOM、漏洞扫描和干净断网环境演练。

## 分支

- `main`：主干开发。
- `release/YYYY.QN`：私有化 LTS。
- `hotfix/<version>`：发布分支热修复。
