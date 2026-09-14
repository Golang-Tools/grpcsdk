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
