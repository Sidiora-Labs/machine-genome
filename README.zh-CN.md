<p align="center">
  <img src="docs/assets/machine-genome-mark.svg" width="88" height="88" alt="Machine Genome 标志">
</p>

# Machine Genome

[![Conformance](https://github.com/Sidiora-Labs/machine-genome/actions/workflows/ci.yml/badge.svg)](https://github.com/Sidiora-Labs/machine-genome/actions/workflows/ci.yml)
[![CodeQL](https://github.com/Sidiora-Labs/machine-genome/actions/workflows/codeql.yml/badge.svg)](https://github.com/Sidiora-Labs/machine-genome/actions/workflows/codeql.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/Sidiora-Labs/machine-genome.svg)](https://pkg.go.dev/github.com/Sidiora-Labs/machine-genome)
[![License: Apache-2.0](https://img.shields.io/badge/code-Apache--2.0-blue.svg)](LICENSE)
[![Protocol: experimental](https://img.shields.io/badge/protocol-experimental-8b5cf6.svg)](docs/versioning.md)

**其他语言：** [English](README.md) · [Русский](README.ru.md) · [Português](README.pt.md) · [日本語](README.ja.md) · [हिन्दी](README.hi.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md)

---

Machine Genome 是一个面向模型、智能体、测试框架、数据集及其连接工件的开放身份与溯源协议。每条不可变的、已签名的创世记录都会获得一个内容寻址的**基因（Gene）**。基因构成可验证的谱系图；后续修订和第三方证明可在不改写历史的情况下添加证据。

**在线注册表：** [machinegenome.org](https://machinegenome.org) ·
**协议：** [MGS 0.2.0 实现规范](spec/MGS-0.2.0-implementation-profile.md) ·
**API：** [OpenAPI 3.1](api/openapi.json)

> [!IMPORTANT]
> MGS 是一个实验性的互操作性规范，本仓库是其中一个参考实现。记录证明了谁签署了声明以及提交了哪些字节；它不能证明继承的能力、意识、所有权、供应商认可或法律地位。

## 为什么需要这个

现代智能体由模型权重、提示词、脚手架、工具、数据集和运行时策略组装而成。仅凭名称无法回答使用了哪些确切组件、谁主张了这种关系，或者证据是否发生了变化。Machine Genome 提供：

- **稳定身份** — 基因是经过安全保护的规范创世记录的 SHA-256 多重哈希；
- **智能体身份** — 每个智能体都获得一个完整的基于基因的 Agent ID，以及一个声明的名和一个由基因组派生的唯一姓氏；
- **智能体发现** — 已签名的能力和训练数据声明让智能体能够宣传自身优势，同时保留证据和信任边界；
- **明确的谱系** — 类型化的父级边区分了授权的、运营者观察到的和未解决的关系；
- **密码学问责** — W3C `eddsa-jcs-2022` 证明将记录绑定到控制器 DID；
- **仅追加演进** — 有序的修订和独立的证明在不改变创世记录的情况下添加历史；
- **公共透明** — 注册表提供规范字节、包含性和一致性证明、已签名检查点、冲突证据和墓碑标记。

该实现不依赖区块链、特定模型供应商、Prometheus、Matrix 或专有 DID 解析器。当前规范离线解析 Ed25519 `did:key` 身份。

## 包含内容

| 领域 | 交付物 |
| --- | --- |
| 协议 | 字节精确的 MGS `0.2.0` 规范，兼容 `0.1.1` 验证 |
| Go 库 | 严格 JCS、多重哈希基因、数据完整性、谱系、修订、证明 |
| 注册表 | ACID 存储、验证准入、DNS 命名空间控制、Merkle 日志、审计链 |
| 浏览器 | 响应式的、通俗易懂的身份、谱系、工件和证明视图 |
| CLI | 密钥生成、编写、签名、验证、摘要、备份和完整性工具 |
| 契约 | JSON Schema、OpenAPI 3.1 文档和机器可读的一致性测试向量 |
| 运维 | 加固的 systemd、nginx/Cloudflare、容器、备份、恢复和监控资源 |

请参阅[项目状态](docs/status.md)了解精确的已实现、已验证和已延期的边界。

## 快速开始

要求：Go 1.24 或更新版本、GNU Make 和 `jq`。

```sh
git clone https://github.com/Sidiora-Labs/machine-genome.git
cd machine-genome
make check build

bin/mgs keygen --out controller.key.json
bin/mgs init-genesis \
  --key controller.key.json \
  --name cody \
  --namespace example.org \
  --subject-type agent \
  --given-name Cody \
  --version 1.0.0 \
  --out genesis.unsigned.json
bin/mgs sign \
  --in genesis.unsigned.json \
  --key controller.key.json \
  --out genesis.json
bin/mgs verify --in genesis.json
bin/mgs gene --in genesis.json
```

私钥文件以 `0600` 权限创建。切勿提交密钥、注册表令牌或私有证据。

### 运行本地注册表

```sh
bin/mgs keygen --out registry-checkpoint.key.json
umask 077
openssl rand -hex 32 > admin.token

bin/mgs registry-serve \
  --data-dir ./registry-data \
  --checkpoint-key registry-checkpoint.key.json \
  --admin-token-file admin.token \
  --listen 127.0.0.1:8080 \
  --public-base-url http://127.0.0.1:8080
```

打开 <http://127.0.0.1:8080>。DNS 命名空间必须在首个创世记录被准入之前完成注册表的 `dns-01` 控制器挑战。当命名空间等于签名控制器 DID 时，DID 命名空间会被接受。`--allow-unverified-namespaces` 仅用于隔离开发环境。

生产部署请遵循[运维手册](docs/registry-operations.md)和[部署资源指南](deploy/README.md)。

## 架构概览

```text
authoring CLI / library
        │ secured MGS records
        ▼
  verification boundary ── strict JSON + JCS + DID + signature + lineage
        │ admitted canonical bytes
        ▼
 ACID object store ── indexes ── append-only Merkle log ── signed checkpoint
        │                         │
        ├── HTTP API              └── inclusion / consistency proofs
        └── provenance explorer
```

共享准入边界是 `registry.Store.Submit`；HTTP 服务无法绕过记录验证。在扩展信任或解析器行为之前，请阅读完整的[架构](docs/architecture.md)和[威胁模型](docs/threat-model.md)。

## 文档

- [文档索引](docs/README.md)
- [概念与信任语义](docs/concepts.md)
- [MGS 0.1.1 兼容规范](spec/MGS-0.1.1-implementation-profile.md)
- [当前协议实现规范](spec/MGS-0.2.0-implementation-profile.md)
- [协议审计](docs/protocol-audit.md)
- [注册表 HTTP API](docs/registry-api.md) 和 [OpenAPI](api/openapi.json)
- [CLI 参考](docs/cli.md)
- [架构](docs/architecture.md)
- [威胁模型](docs/threat-model.md)
- [一致性与测试向量](docs/conformance.md)
- [生产运维](docs/registry-operations.md)
- [公共注册表策略](docs/registry-policy.md)
- [版本控制与兼容性](docs/versioning.md)
- [治理](GOVERNANCE.md)、[安全](SECURITY.md)和[贡献](CONTRIBUTING.md)
- [路线图](docs/roadmap.md)

## 开发

```sh
make help
make check       # 测试、vet、schema、API 契约、文档、记录
make race        # 竞态检测
make coverage    # HTML 和文本覆盖率报告
make build       # 静态本地二进制文件
make dist        # 发布归档和校验和
```

CI 在每次变更时重复执行一致性、竞态、构建、记录、仓库和漏洞扫描。带标签的发布从干净的代码树构建，附带校验和和软件物料清单。

## 安全与信任

请勿在公开 issue 中报告安全漏洞。请遵循 [SECURITY.md](SECURITY.md) 使用 GitHub 的私密漏洞报告。密码学审查、隐私审查、独立见证者、检查点 gossip、联邦和中立标准治理仍是路线图中的明确工作项。

## 许可证

Go 实现和仓库工具基于 [Apache-2.0](LICENSE) 许可。协议和说明性文档基于 [CC BY 4.0](LICENSES/CC-BY-4.0.txt) 提供。已签名的注册表记录不授予对其引用的外部模型、源代码仓库、数据集或工件的任何权利。请参阅 [NOTICE](NOTICE) 了解溯源和归属信息。
