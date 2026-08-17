# 证书生命周期存档库

本项目是一个使用 Go 标准库实现的证书生命周期存档系统，支持证书申请、签发配置、证书版本、部署凭据、撤销记录的管理。

## 功能特性

- 证书申请状态机：草稿、锁定、签发登记、分批部署、确认生效、续期、撤销。
- 跨实体规则校验：有效期重叠、部署容量、续期截止时间、撤销不可逆。
- 本地文件持久化，带写前日志、原子快照、校验和安全恢复。
- 背景任务：续期扫描、快照轮换。
- 组合查询：有效期冲突、部署缺口、即将到期、撤销影响。
- 支持并发访问和乐观版本控制。

## 运行自检

```bash
go run ./cmd/certarchive --self-check
```

## 测试

```bash
go test ./...
go test -race ./...
```

## 项目结构

- `cmd/certarchive`：可执行入口。
- `internal/domain`：领域模型、状态机、错误。
- `internal/application`：应用服务与用例。
- `internal/repository`：仓库接口与实现。
- `internal/journal`：写前日志。
- `internal/recovery`：恢复与快照。
- `internal/scheduler`：后台任务。
- `internal/query`：查询服务。
- `internal/audit`：审计记录。

## 环境要求

- Go 1.21+
- 不依赖任何外部库或网络服务。
