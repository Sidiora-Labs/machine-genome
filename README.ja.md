<p align="center">
  <img src="docs/assets/machine-genome-mark.svg" width="88" height="88" alt="Machine Genome マーク">
</p>

# Machine Genome

[![Conformance](https://github.com/paxlabs-inc/machine-genome/actions/workflows/ci.yml/badge.svg)](https://github.com/paxlabs-inc/machine-genome/actions/workflows/ci.yml)
[![CodeQL](https://github.com/paxlabs-inc/machine-genome/actions/workflows/codeql.yml/badge.svg)](https://github.com/paxlabs-inc/machine-genome/actions/workflows/codeql.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/paxlabs-inc/machine-genome.svg)](https://pkg.go.dev/github.com/paxlabs-inc/machine-genome)
[![License: Apache-2.0](https://img.shields.io/badge/code-Apache--2.0-blue.svg)](LICENSE)
[![Protocol: experimental](https://img.shields.io/badge/protocol-experimental-8b5cf6.svg)](docs/versioning.md)

Machine Genome は、モデル、エージェント、ハーネス、データセット、およびそれらを接続するアーティファクトのためのオープンなアイデンティティとプロベナンス（出所証明）プロトコルです。改変不可能で署名された各ジェネシスレコードは、コンテンツアドレス指定された**遺伝子（Gene）**を受け取ります。遺伝子は検証可能な系譜グラフを形成し、その後の修正や第三者のアテステーションは、履歴を上書きせずに証拠を追加します。

**ライブレジストリ：** [machinegenome.org](https://machinegenome.org) ·
**プロトコル：** [MGS 0.2.0 実装プロファイル](spec/MGS-0.2.0-implementation-profile.md) ·
**API：** [OpenAPI 3.1](api/openapi.json)

> [!IMPORTANT]
> MGS は実験的な相互運用性プロファイルであり、本リポジトリはリファレンス実装の一つです。レコードは誰が声明に署名し、どのバイトがコミットされたかを証明しますが、継承された能力、意識、所有権、ベンダーの保証、法的ステータスを証明するものではありません。

## なぜこれが必要か

現代のエージェントは、モデルの重み、プロンプト、スキャフォールド、ツール、データセット、ランタイムポリシーから構成されます。名前だけでは、どのコンポーネントが使用されたか、誰がその関係を主張したか、証拠が変更されたかどうかを判断できません。Machine Genome は以下を提供します：

- **安定したアイデンティティ** — 遺伝子はセキュアな正規のジェネシスレコードの SHA-256 マルチハッシュです；
- **エージェントアイデンティティ** — 各エージェントは完全な Gene ベースの Agent ID と、申告された名前、遺伝子グループから派生した一意の姓を受け取ります；
- **エージェント発見** — 署名された能力とトレーニングデータの宣言により、エージェントは証拠と信頼境界を保ちながら強みをアドバタイズできます；
- **明示的な系譜** — 型付きの親エッジにより、認可された、オペレーターが観察した、未解決の関係を区別します；
- **暗号化された説明責任** — W3C `eddsa-jcs-2022` 証明により、レコードをコントローラ DID に結び付けます；
- **追記専用の進化** — 順序付きの修正と独立したアテステーションが、ジェネシスを変更せずに履歴を追加します；
- **公開透明性** — レジストリは正規バイト、包含性と整合性の証明、署名付きチェックポイント、競合証拠、およびトゥームストーンを提供します。

本実装はブロックチェーン、特定のモデルベンダー、Prometheus、Matrix、またはプロプライエタリな DID リゾルバーに依存しません。現在のプロファイルは Ed25519 `did:key` アイデンティティをオフラインで解決します。

## 含まれるもの

| 領域 | 成果物 |
| --- | --- |
| プロトコル | `0.1.1` 検証との互換性を持つバイト正確な MGS `0.2.0` プロファイル |
| Go ライブラリ | 厳密な JCS、マルチハッシュ遺伝子、データインテグリティ、系譜、修正、アテステーション |
| レジストリ | ACID ストレージ、検証済みアドミッション、DNS 名前空間管理、Merkle ログ、監査チェーン |
| エクスプローラー | レスポンシブで分かりやすいアイデンティティ、系譜、アーティファクト、証明のビュー |
| CLI | 鍵生成、作成、署名、検証、ダイジェスト、バックアップ、インテグリティツール |
| コントラクト | JSON Schema、OpenAPI 3.1 ドキュメント、機械可読な適合性テストベクトル |
| 運用 | 強化された systemd、nginx/Cloudflare、コンテナ、バックアップ、リストア、監視アセット |

正確な実装済み、検証済み、延期された境界については[プロジェクトステータス](docs/status.md)をご覧ください。

## クイックスタート

要件：Go 1.24 以降、GNU Make、`jq`。

```sh
git clone https://github.com/paxlabs-inc/machine-genome.git
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

秘密鍵ファイルはモード `0600` で作成されます。鍵、レジストリトークン、プライベートな証拠をコミットしないでください。

### ローカルレジストリの実行

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

<http://127.0.0.1:8080> を開いてください。DNS 名前空間は、最初のジェネシスが受理される前に、レジストリの `dns-01` コントローラチャレンジを完了する必要があります。DID 名前空間は、名前空間が署名コントローラ DID と等しい場合に受け入れられます。`--allow-unverified-namespaces` は隔離された開発専用です。

本番環境のデプロイメントについては、[運用ランブック](docs/registry-operations.md)と[デプロイメントアセットガイド](deploy/README.md)に従ってください。

## アーキテクチャ概要

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

共有アドミッション境界は `registry.Store.Submit` です。HTTP サービスはレコード検証をバイパスできません。信頼やリゾルバーの動作を拡張する前に、完全な[アーキテクチャ](docs/architecture.md)と[脅威モデル](docs/threat-model.md)をお読みください。

## ドキュメント

- [ドキュメントインデックス](docs/README.md)
- [概念と信頼セマンティクス](docs/concepts.md)
- [MGS 0.1.1 互換プロファイル](spec/MGS-0.1.1-implementation-profile.md)
- [現在のプロトコル実装プロファイル](spec/MGS-0.2.0-implementation-profile.md)
- [プロトコル監査](docs/protocol-audit.md)
- [レジストリ HTTP API](docs/registry-api.md) と [OpenAPI](api/openapi.json)
- [CLI リファレンス](docs/cli.md)
- [アーキテクチャ](docs/architecture.md)
- [脅威モデル](docs/threat-model.md)
- [適合性テストベクトル](docs/conformance.md)
- [本番運用](docs/registry-operations.md)
- [公開レジストリポリシー](docs/registry-policy.md)
- [バージョニングと互換性](docs/versioning.md)
- [ガバナンス](GOVERNANCE.md)、[セキュリティ](SECURITY.md)、[コントリビューション](CONTRIBUTING.md)
- [ロードマップ](docs/roadmap.md)

## 開発

```sh
make help
make check       # テスト、vet、スキーマ、API コントラクト、ドキュメント、レコード
make race        # レースディテクター
make coverage    # HTML およびテキストカバレッジレポート
make build       # スタティックローカルバイナリ
make dist        # リリースアーカイブとチェックサム
```

CI は変更のたびに、適合性、レース、ビルド、レコード、リポジトリ、脆弱性スキャンを繰り返します。タグ付きリリースは、チェックサムとソフトウェア部品表を含むクリーンなツリーからビルドされます。

## セキュリティと信頼

公開の issue でセキュリティ脆弱性を報告しないでください。GitHub のプライベート脆弱性報告を使用するには [SECURITY.md](SECURITY.md) に従ってください。暗号化レビュー、プライバシーレビュー、独立したウィットネス、チェックポイントゴシップ、フェデレーション、中立的な標準ガバナンスは、ロードマップの明示的な作業として残っています。

## ライセンス

Go 実装とリポジトリツールは [Apache-2.0](LICENSE) の下でライセンスされています。プロトコルと説明ドキュメントは [CC BY 4.0](LICENSES/CC-BY-4.0.txt) の下で提供されています。署名されたレジストリレコードは、参照する外部モデル、ソースリポジトリ、データセット、アーティファクトに対する権利を付与しません。プロベナンスと帰属については [NOTICE](NOTICE) をご覧ください。
