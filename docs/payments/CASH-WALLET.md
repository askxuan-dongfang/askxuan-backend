# 现金钱包与商户配置

本次实现：真实余额账户、按分记账的不可重复流水、统一收银（商城、DIY、寺院预约、大师直约、即时咨询、AI 报告）、整单退款退钱包、充值款原路退款。历史 mock 交易不会转为余额，积分与功德值不属于现金账户。充值不计收入、不发消费积分；消费才计账，退款冲销积分。

## 配置

先执行增量 `scripts/db/20260926_balance_wallet.sql`。新表初始为空，无存量资金转换。业务状态与扣款在同一 MySQL 实例内跨库事务提交，迁移只授予所需列权限。保留原来的订单与支付记录；不要执行全量初始化脚本。

支付服务配置示例（密钥文件只存服务器，权限 0600，不进入 Git、镜像或聊天）：

```yaml
AppEnv: production
Provider: live
Wallet:
  Enabled: true
  Wechat:
    Enabled: false
    AppID: ''
    MerchantID: ''
    MerchantSerial: ''
    PrivateKeyFile: /app/etc/payment-secrets/wechat-private.pem
    PublicKeyFile: /app/etc/payment-secrets/wechatpay-public.pem
    PublicKeyID: ''
    APIv3KeyFile: /app/etc/payment-secrets/wechat-apiv3-key
    NotifyURL: https://YOUR-DOMAIN/api/v1/payments/callback/wechat
  Alipay:
    Enabled: false
    AppID: ''
    MerchantID: '' # seller_id
    PrivateKeyFile: /app/etc/payment-secrets/alipay-private.pem
    PublicKeyFile: /app/etc/payment-secrets/alipay-public.pem
    NotifyURL: https://YOUR-DOMAIN/api/v1/payments/callback/alipay
    ReturnURL: https://YOUR-DOMAIN/c/wallet
    Sandbox: false
```

未开通商户时保留两个 Enabled=false。充值入口返回“暂未开通”，不能造余额。只有明确开通、部署密钥及回调域名后再启用。生产禁止 mock；模拟环境禁止启用真实商户；生产禁止支付宝沙箱。沙箱必须使用独立数据库，不能与生产余额共用。

本期接入类型是微信 H5、支付宝手机网站支付。iOS 充值通过浏览器调起该流程，回到 App 后刷新服务端状态；不是微信／支付宝原生 App SDK 接入。确认商户开通的是对应产品，微信 H5 域名按商户平台登记。入口反代必须覆盖 X-Real-IP，payment-service 不对公网直接开放。WeChat 使用配置的平台公钥与 ID 验签，换钥需保留轮换窗口并更新配置重启；不关闭签名验证。

## 资金规则

- 单笔充值 1—5000 元。客户端提供稳定 requestId，网络失败使用同一编号重试。
- 仅验签成功且商户、应用、金额、币种、交易编号匹配的通知／查单结果增加余额，支付跳转或前端提示不能记账。
- 固定支付编号+唯一流水键+账户行锁防止重复扣／入账。余额不足整笔回滚，订单状态、支付、积分、资金流水及业务 outbox 一起提交。
- 体验商城订单禁止动用真实余额。现有模拟支付独立标示，不发真钱。
- 余额订单当前只支持一次整单退款，经过原业务取消／售后入口；外部退款 API 限平台超管，MQ 走持久事件。
- 每笔充值支持一次原路退款，可退金额不超过原充值额及当前可用余额。先把金额从可用转冻结，再调用原渠道。已消费的钱先经订单退款回钱包，才能退充值款。
- 退款 HTTP 超时／验签失败／渠道 PROCESSING 保持冻结，不假报成功。后台每 30 秒扫描，以稳定退款号重试／查单，确认成功才核销冻结。异常退款需运维核对渠道记录，禁止手工把未确认退款改成成功或释放冻结。
- 不提供转账或任意提现。充值页与消费账单分开，避免重复计算支出。

## 验证与上线边界

单测覆盖 RSA2、微信响应签名／时间窗、重复参数、金额解析、禁用配置。隔离 MySQL 测试覆盖重复回调、越权、篡改金额、重复支付、余额不足、并发竞争、退款幂等、冻结核销、六类业务、积分已解锁报告不再扣款及账表一致。

```
cd services/commerce/payment-service
go test ./...
# 仅一次性、空白 MySQL 容器；测试会创建 askxuan_* 夹具库
WALLET_TEST_ISOLATED=1 WALLET_TEST_DSN='root@tcp(127.0.0.1:33362)/askxuan_payment?charset=utf8mb4' go test ./internal/cashier -run TestCashWalletMySQL -v
```

商户未开通，因此未完成真实小额充值、渠道账单核对和原路退款到账验收。启用前分别验证两渠道签名回调、主动查单、断网重试及退款到账，确认生产域名与密钥配置。这里的后台查单是补偿机制，不替代商户平台日终账单核对。回滚代码保留资金表与流水，已有现金余额时不可回滚到仅 mock 的版本继续运营。

官方接口依据：[微信 H5 下单](https://pay.wechatpay.cn/doc/v3/merchant/4012791834)、[微信支付通知](https://pay.wechatpay.cn/doc/v3/merchant/4012791861)、[支付宝手机网站支付](https://developer.alibaba.com/docs/api.htm?apiId=1056&docType=4)。
