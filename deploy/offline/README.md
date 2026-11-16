# 私有化离线包

离线包由 `release-onprem` 流水线生成，建议结构：

```text
mlf-erp-<version>-offline-linux-<arch>.tar.gz
├─ manifest.json          # 版本、组件、架构、校验和
├─ SHA256SUMS             # 完整性校验
├─ signature              # 发布签名（待接入）
├─ web/                   # 前端静态文件
├─ bin/                   # api / worker / migrate 二进制
├─ db/                    # 迁移脚本
├─ deploy/                # compose / helm / 安装器
├─ docs/                  # 安装、升级、运维、回滚手册
└─ scripts/               # preflight、backup、restore、diagnose
```

正式交付前必须补齐：

- 离线 License 文件与机器指纹校验。
- 干净断网环境安装/升级演练。
- 备份、恢复、巡检、诊断包。
- 发布包签名、SBOM、漏洞扫描报告。
