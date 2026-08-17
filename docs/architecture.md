# 系统架构

本项目采用分层架构，分为领域层、应用层、基础设施层和接口层。

## 领域层 (internal/domain)

定义核心业务模型、状态机、规则和错误。所有实体均包含乐观锁版本号，用于并发控制。

## 应用层 (internal/application)

协调领域对象和仓库，实现用例，如提交申请、锁定、签发、部署确认、续期和撤销。

## 基础设施层

- internal/repository：文件持久化仓库，提供原子写入和 fsync 耐久性。
- internal/journal：写前日志，记录事件，支持恢复。
- internal/recovery：快照管理和恢复。
- internal/audit：审计日志。

## 查询与调度

- internal/query：组合查询服务。
- internal/scheduler：后台任务调度器。

## 入口

cmd/certarchive：命令行入口，支持 --self-check。
