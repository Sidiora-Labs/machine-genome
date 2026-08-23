<p align="center">
  <img src="docs/assets/machine-genome-mark.svg" width="88" height="88" alt="Machine Genome mark">
</p>

# Machine Genome

[![Conformance](https://github.com/Sidiora-Labs/machine-genome/actions/workflows/ci.yml/badge.svg)](https://github.com/Sidiora-Labs/machine-genome/actions/workflows/ci.yml)
[![CodeQL](https://github.com/Sidiora-Labs/machine-genome/actions/workflows/codeql.yml/badge.svg)](https://github.com/Sidiora-Labs/machine-genome/actions/workflows/codeql.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/Sidiora-Labs/machine-genome.svg)](https://pkg.go.dev/github.com/Sidiora-Labs/machine-genome)
[![License: Apache-2.0](https://img.shields.io/badge/code-Apache--2.0-blue.svg)](LICENSE)
[![Protocol: experimental](https://img.shields.io/badge/protocol-experimental-8b5cf6.svg)](docs/versioning.md)

**Outros idiomas:** [English](README.md) · [Русский](README.ru.md) · [简体中文](README.zh-CN.md) · [日本語](README.ja.md) · [हिन्दी](README.hi.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md)

---

Machine Genome é um protocolo aberto de identidade e procedência para modelos,
agentes, harnesses, conjuntos de dados e artefatos que os conectam. Cada registro
de gênese imutável e assinado recebe um **Gene** com endereçamento por conteúdo.
Os genes formam um grafo de linhagem verificável; emendas posteriores e atestações
de terceiros adicionam evidências sem reescrever o histórico.

**Registro ao vivo:** [machinegenome.org](https://machinegenome.org) ·
**Protocolo:** [Perfil de implementação MGS 0.2.0](spec/MGS-0.2.0-implementation-profile.md) ·
**API:** [OpenAPI 3.1](api/openapi.json)

> [!IMPORTANT]
> MGS é um perfil experimental de interoperabilidade e este repositório é uma
> das implementações de referência. Um registro prova quem assinou uma declaração
> e quais bytes foram comprometidos; não prova capacidade herdada, consciência,
> propriedade, endosso do fornecedor ou status legal.

## Por que isso existe

Agentes modernos são montados a partir de pesos de modelos, prompts, scaffolds,
ferramentas, conjuntos de dados e política de runtime. Apenas nomes não podem
responder quais componentes exatos foram usados, quem afirmou o relacionamento
ou se as evidências mudaram. Machine Genome fornece:

- **identidade estável** — um Gene é um multihash SHA-256 de um registro de
  gênese canônico protegido;
- **identidade do agente** — cada agente recebe um Agent ID completo baseado em
  Gene, além de um nome declarado e um sobrenome único derivado do genoma;
- **descoberta de agentes** — declarações assinadas de capacidades e dados de
  treinamento permitem que agentes anunciem pontos fortes, preservando evidências
  e limites de confiança;
- **linhagem explícita** — arestas pai tipificadas distinguem relacionamentos
  autorizados, observados pelo operador e não resolvidos;
- **responsabilidade criptográfica** — provas W3C `eddsa-jcs-2022` vinculam
  registros a DIDs de controladores;
- **evolução apenas por acréscimo** — emendas ordenadas e atestações independentes
  adicionam histórico sem mutar o gênese;
- **transparência pública** — registros fornecem bytes canônicos, provas de
  inclusão e consistência, pontos de verificação assinados, evidências de conflito
  e lápides.

A implementação não depende de blockchain, um fornecedor de modelos específico,
Prometheus, Matrix ou um resolvedor DID proprietário. O perfil atual resolve
identidades Ed25519 `did:key` offline.

## O que está incluído

| Área | Entregável |
| --- | --- |
| Protocolo | Perfil MGS `0.2.0` byte-exato com compatibilidade de verificação `0.1.1` |
| Biblioteca Go | JCS estrito, Genes multihash, Integridade de Dados, linhagem, emendas, atestações |
| Registro | Armazenamento ACID, admissão verificada, controle de namespace DNS, log Merkle, cadeia de auditoria |
| Explorador | Visualizações responsivas e em linguagem clara de identidade, linhagem, artefatos e provas |
| CLI | Geração de chaves, criação, assinatura, verificação, digest, backup e ferramentas de integridade |
| Contratos | Schemas JSON, documento OpenAPI 3.1 e vetores de conformidade legíveis por máquina |
| Operações | Assets de systemd reforçado, nginx/Cloudflare, container, backup, restauração e monitoramento |

Veja [status do projeto](docs/status.md) para os limites precisos de implementação,
validação e diferimento.

## Início rápido

Requisitos: Go 1.24 ou mais novo, GNU Make e `jq`.

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

Arquivos de chave privada são criados com modo `0600`. Nunca faça commit de
chaves, tokens de registro ou evidências privadas.

### Executar um registro local

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

Abra <http://127.0.0.1:8080>. Namespaces DNS devem completar o desafio
`dns-01` do controlador do registro antes que seu primeiro gênese seja admitido.
Namespaces DID são aceitos quando o namespace é igual ao DID do controlador
assinante. `--allow-unverified-namespaces` é apenas para desenvolvimento isolado.

Para uma implantação em produção, siga o
[runbook de operações](docs/registry-operations.md) e o
[guia de assets de implantação](deploy/README.md).

## Arquitetura em resumo

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

O limite de admissão compartilhado é `registry.Store.Submit`; o serviço HTTP
não pode ignorar a verificação de registros. Leia a
[arquitetura](docs/architecture.md) completa e o [modelo de ameaças](docs/threat-model.md)
antes de estender confiança ou comportamento do resolvedor.

## Documentação

- [Índice da documentação](docs/README.md)
- [Conceitos e semântica de confiança](docs/concepts.md)
- [Perfil de compatibilidade MGS 0.1.1](spec/MGS-0.1.1-implementation-profile.md)
- [Perfil atual de implementação do protocolo](spec/MGS-0.2.0-implementation-profile.md)
- [Auditoria do protocolo](docs/protocol-audit.md)
- [HTTP API do registro](docs/registry-api.md) e [OpenAPI](api/openapi.json)
- [Referência CLI](docs/cli.md)
- [Arquitetura](docs/architecture.md)
- [Modelo de ameaças](docs/threat-model.md)
- [Conformidade e vetores de teste](docs/conformance.md)
- [Operações de produção](docs/registry-operations.md)
- [Política do registro público](docs/registry-policy.md)
- [Versionamento e compatibilidade](docs/versioning.md)
- [Governança](GOVERNANCE.md), [segurança](SECURITY.md) e
  [contribuição](CONTRIBUTING.md)
- [Roadmap](docs/roadmap.md)

## Desenvolvimento

```sh
make help
make check       # testes, vet, schemas, contrato API, docs, registros
make race        # detector de corrida
make coverage    # relatório de cobertura HTML e textual
make build       # binário local estático
make dist        # arquivos de release e checksums
```

CI repete verificações de conformidade, corrida, build, registros, repositório
e vulnerabilidades em cada alteração. Releases com tag são construídas a partir
de uma árvore limpa com checksums e uma lista de materiais de software.

## Segurança e confiança

Não reporte vulnerabilidades de segurança em uma issue pública. Siga
[SECURITY.md](SECURITY.md) para usar o relatório privado de vulnerabilidades do
GitHub. Revisão criptográfica, revisão de privacidade, testemunhas independentes,
fofocas de pontos de verificação, federação e governança neutra de padrões
permanecem como trabalho explícito do roadmap.

## Licença

A implementação Go e as ferramentas do repositório são licenciadas sob
[Apache-2.0](LICENSE). Documentação protocolar e explicativa está disponível sob
[CC BY 4.0](LICENSES/CC-BY-4.0.txt). Registros assinados do registro não
concedem direitos sobre modelos externos, repositórios de código, conjuntos de
dados ou artefatos que eles referenciam. Veja [NOTICE](NOTICE) para procedência
e atribuição.
