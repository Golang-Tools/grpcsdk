# v2.2.0

## 新增功能

+ 新增任意grpc连接选项的透传口 `WithAdditionalDialOptions`:追加在SDK构造的选项之后,可以覆盖SDK的默认设置;`stats/opentelemetry.DialOption`、`credentials/local`、`WithWriteBufferSize` 等都可以从这里接入
+ 新增 `WithStatsHandlers`:接入grpc的 `stats.Handler`(可设置多个),用于观测
+ 新增 `WithPerRPCCredentials`:设置请求级凭证,grpc的 `credentials/oauth`、`credentials/jwt`、`credentials/sts` 实现可直接使用
+ 新增 `WithAuthority`:覆盖请求的authority头(如证书校验域名与连接地址不一致时)
+ 新增 `WithWaitForReady`:请求等待连接就绪后再发送,默认false保持快速失败

## 文档

+ README新增`进阶配置`章节:透传任意grpc选项(OpenTelemetry一行接入、credentials/local、缓冲区参数)、请求级凭证与其他开关

## 测试

+ 新增透传选项(自定义拦截器生效)、stats.Handler接入、请求级凭证端到端(服务端验证authorization)、authority、等待就绪与快速失败对照等测试

# v2.1.0

## 新增功能

+ 新增grpc内建重试策略配置:`RetryPolicy` 与 `WithRetryPolicy`,通过service config的默认methodConfig下发(顶层不支持retryPolicy);`MaxAttempts` 限制为[2,5],状态码列表不能为空,只有幂等的方法才应该配置重试
+ 新增连接参数配置:`WithConnectParams`(重连退避与单次建连最短超时)与 `WithIdleTimeoutMS`(空闲连接回收,0使用grpc默认的30分钟,负数表示禁用)
+ 新增负载均衡策略配置:`WithLoadBalancingPolicy`(pick_first/round_robin/least_request/weighted_round_robin等);未设置时保持原有自动选择逻辑(单地址pick_first、多地址与dns地址round_robin、xds地址由xds决定)
+ 默认对所有调用附加 `grpc.StaticMethod()`,标记调用来自编译期确定的方法(供stats/opentelemetry等观测插件把方法名作为指标属性)

## 文档

+ README新增`负载均衡与连接池`与`重试策略`说明:grpc推荐单个ClientConn配合负载均衡策略承载并发,连接池只在需要连接级隔离等特殊场景使用

## 测试

+ 新增重试策略真实生效的端到端测试(基于自定义计数服务)、参数校验、methodConfig转换、负载均衡策略、连接参数与空闲回收等测试;语句覆盖率95.1%

## 其他

+ 移除GitHub Actions CI工作流(本地校验:`gofmt -l . && go vet ./... && go test ./... -race`)
+ grpc-go当前尚未实现对冲策略(hedging,源码中仅有TODO),因此不提供相关配置

# v2.0.1

## 测试

+ 补充单元测试,语句覆盖率由 70.6% 提升到 94.7%
+ 新增多地址本地负载均衡(含健康检查服务名)与 `dns:///`、`xds:///` 地址前缀场景的测试
+ 新增 TLS(仅根证书)与 mTLS(客户端证书+私钥)场景的测试,内含自签证书生成工具
+ 新增压缩、消息大小、keepalive、滑动窗口、阻塞建连等配置项的测试
+ 新增流式请求拦截器、未初始化时 `GetClient` 的 panic 行为测试
+ 新增连接池的故障路径测试:注水中途失败后的连接清理、连接处于 `TransientFailure`/`Shutdown` 状态时的获取与释放、池满时放回的防御分支

# v2.0.0

缺陷修复 + 现代化改造版本。

## 破坏性变更

+ 模块路径变更为 `github.com/Golang-Tools/grpcsdk/v2`,旧路径 `github.com/Golang-Tools/grpcsdk` 停留在 v0.0.2 不再更新
+ 最低 go 版本提升到 1.25(由 grpc v1.83.2 的版本要求决定)
+ 依赖升级:`google.golang.org/grpc` v1.83.2、`github.com/Golang-Tools/optparams` v1.0.0、`github.com/Golang-Tools/loggerhelper/v4` v4.0.0;不再依赖 `github.com/json-iterator/go`(改用标准库 `encoding/json`)
+ `optparams` v1.0.0 的 `GetOption` 改为拷贝语义(不再原地修改),内置的选项解析全部改用 `optparams.Apply`
+ `SDK.Logger` 类型由 `*loggerhelper.Log` 改为 `*slog.Logger`(标准库 `log/slog`),日志字段值直接以 key-value 传入
+ `grpc.Dial` 已废弃,改用 `grpc.NewClient`;`Conn_With_Block` 的阻塞建连语义改由 `Connect` + 等待 `Ready` 实现,默认最长等待 10s,并新增 `NewClientOptions.BlockUntilReady`/`BlockWaitTime` 字段与 `WithBlockUntilReady()`/`WithBlockWaitTime()` 选项
+ `NewClient`/`NewPool` 的默认配置内置 insecure 传输凭证(grpc 新版本要求显式设置传输凭证),自行设置 `DialOpts` 时需要自行提供凭证

## bug修复

+ 修复 `GrpcConnPool.Release` 在池已关闭或发送阻塞时会永久持有锁导致 `Close` 死锁的问题
+ 修复 `GrpcConnPool.Close` 关闭 channel 后 `acquire` 可能读到空值导致 panic、以及 `p.clis` 字段并发读写的数据竞争
+ 修复 `GrpcConnPool.acquire` 遇到 `Shutdown` 状态的连接时既不放回也不关闭导致的连接泄漏与空转
+ 修复 `fillConns` 注水失败时会泄漏已创建连接的问题
+ 修复 `NewPool` 重复应用选项、以及浅拷贝 `DefaultNewClientPoolOpts` 导致 `HasClientConfig` 污染全局默认配置的问题
+ 修复并发调用 `SDK.GetClient` 时可能重复创建客户端获取器导致连接泄漏的竞态
+ 修复 `SDK.NewClientGetter` 重复调用会覆盖旧引用导致旧连接泄漏的问题,现在首次创建后重复调用返回同一对象
+ 修复 `SDK.Init` 重复调用会累积 DialOption 且不关闭旧连接的问题,现在重复调用会关闭旧的获取器并重建
+ 修复 `SDK.Close` 读取 `clientgetter` 无锁保护的数据竞争
+ 修复 `RegistInterceptor` 注册多个拦截器时只有最后一个生效的问题(改用 `WithChainUnaryInterceptor`/`WithChainStreamInterceptor`)
+ 修复创建 SDK 时会修改并清空全局 logger 扩展字段的问题,不再修改全局 logger 配置
+ 修复 `NewCtx` 在 SDK 未配置 `Query_Timeout` 时显式传入的 `WithTimeout` 不生效的问题
+ 修复 `Available`/`Limits` 在 nil 池对象上 panic 的问题

## 其他变更

+ 新增标准错误:`ErrNoAddresses`、`ErrEmptyAddr`、`ErrNilClientFactory`、`ErrCACertNotParsed`;`NewClient`/`NewPool` 会校验工厂函数与地址
+ TLS 配置增加最低版本限制(TLS 1.2),CA 证书解析失败会返回明确错误
+ 池的 `Close` 使用 `errors.Join` 聚合所有连接的关闭错误,且重复调用是安全的
+ 修正多处复制粘贴错误的函数注释与单位说明
+ 清理仓库中残留的 `docs/` 文档导出目录与 `pmfprc.json` 模板文件,不再跟踪 `.DS_Store`

## 工程化

+ 新增单元测试覆盖客户端、连接池、SDK 全流程以及并发/竞态场景
+ 新增 GitHub Actions CI(gofmt 检查、go vet、单元测试、race 检测、golangci-lint)

# v0.0.2

## bug修复

修复`SDKConfig`的jsonschema校验问题,如果要校验则不会因为array型的数据为空而报错

# v0.0.1

项目创建
