# API 契约

- OpenAPI 文件：`api/openapi.yaml`
- 当前 `packages/sdk` 为手写的最小客户端。
- 后续通过 OpenAPI 生成 TypeScript SDK，确保前后端类型一致。

约定：

- 金额、数量使用字符串传输。
- 响应统一为 `{ code, message, data, request_id }`。
- 租户通过 `X-Tenant-ID` 传递；开发环境省略时使用默认租户。
- 错误码包含 `BAD_REQUEST`、`UNAUTHORIZED`、`FORBIDDEN`、`CAPABILITY_REQUIRED`、`LIMIT_EXCEEDED`、`LICENSE_EXPIRED`。
