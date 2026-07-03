# 五端本地闭环交付报告

生成时间：2026-07-03 09:30（Asia/Shanghai）

## 本轮实现

- 统一端侧请求头：三个 Web 管理台、C 端 iOS、法师 iOS 均补 `X-Client-Type` / `X-Client-Version`。
- Token refresh：三个 Web 管理台和两个 iOS 均接入 `/api/v1/auth/refresh`，刷新失败走登出/未授权处理。
- APNs mock 闭环：message-service 新增 `device_token` 表、注册/解绑接口、管理台 `push-logs` 查询接口；两个 iOS 登录后自动注册本地 mock device token。
- 聊天本地闭环：message-service 新增 `/api/v1/messages/send`，C 端发送咨询消息后落站内消息，法师端 `/api/v1/admin/messages/master` 可读取 `bizType=consult` 消息；同时写入 `push_log`。
- DIY 材料表对齐：`material` 表补 `category/five_elements/image/stock/status`，修复商城管理台创建材料 500。
- 数据初始化对齐：`db/init.sql` 补材料字段、seed 数据、`device_token` 表。

## 构建结果

- 后端：`make build` 通过。
- 后端：`make vet` 通过。
- Web 寺院管理台：`npm run build` 通过。
- Web 商城管理台：`npm run build` 通过。
- Web 平台管理台：`npm run build` 通过。
- iOS C 端：`xcodebuild -project DongFangApp.xcodeproj -scheme DongFangApp -destination 'platform=iOS Simulator,name=iPhone 17 Pro' -derivedDataPath ./build build` 通过。
- iOS 法师端：`xcodebuild -project MasterApp.xcodeproj -scheme MasterApp -destination 'platform=iOS Simulator,name=iPhone 17 Pro' -derivedDataPath ./build build` 通过。

## 闭环脚本结果

- 预约祈福：`scripts/test-mvp1-closed-loop.sh`，10/10 通过。
- DIY 手串：`scripts/test-mvp2-diy-closed-loop.sh`，修复材料表后 16/16 通过。
- 普通商城：`scripts/test-mvp2-trade-closed-loop.sh`，通过；脚本统计显示 11/10，是脚本多计了物流验证 PASS。
- 评价/举报：`scripts/test-mvp3-review-closed-loop.sh`，9/9 通过。
- 营销：`scripts/test-mvp3-marketing-closed-loop.sh`，11/11 通过。
- 财务：`scripts/test-mvp3-finance-closed-loop.sh`，10/10 通过。
- 审核：`scripts/test-mvp3-audit-closed-loop.sh`，10/10 通过。

## 手动接口验证

- `POST /api/v1/messages/device-token`：返回 `{"id":1,"status":"active"}`。
- `DELETE /api/v1/messages/device-token`：返回 `{"id":1,"status":"inactive"}`。
- `GET /api/v1/admin/messages/push-logs?page=1&size=5`：可查询到预约/咨询等推送日志。
- `POST /api/v1/messages/send`：返回消息 id。
- `GET /api/v1/admin/messages/master`：可查询到 C 端发送的 `bizType=consult` 咨询消息。

## 当前限制与风险

- OpenIM：当前 `docker-compose.yml` 只包含 MongoDB/Kafka/Zookeeper 等依赖，没有 OpenIM 本体容器和配置包；登录接口对 OpenIM 仍是 best-effort，未部署真实 OpenIM 时 `imToken` 可能为空。本轮聊天闭环通过 message-service 站内消息实现，不等同于真实 OpenIM SDK 长连接。
- APNs：本轮为 mock device token + `push_log`，没有 Apple Developer 凭证，不触发真实 APNs。
- iOS：构建通过，但仍有 Swift 6 预警（MainActor `AuthStore.shared` 非隔离上下文访问）和部分 iOS 17 `onChange` deprecated 预警；当前 Swift 5 可构建。
- Web/iOS 视觉冒烟：本轮完成构建和接口闭环验证，未产出 Playwright/模拟器截图集。
- `make db-init` 对已有数据库不是完全幂等，重复初始化会遇到 admin 账号唯一键冲突；本轮已对本地库做了在线 ALTER 以完成验收。
