<p align="center">
  <img src="docs/assets/machine-genome-mark.svg" width="88" height="88" alt="Machine Genome mark">
</p>

# Machine Genome

[![Conformance](https://github.com/Sidiora-Labs/machine-genome/actions/workflows/ci.yml/badge.svg)](https://github.com/Sidiora-Labs/machine-genome/actions/workflows/ci.yml)
[![CodeQL](https://github.com/Sidiora-Labs/machine-genome/actions/workflows/codeql.yml/badge.svg)](https://github.com/Sidiora-Labs/machine-genome/actions/workflows/codeql.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/Sidiora-Labs/machine-genome.svg)](https://pkg.go.dev/github.com/Sidiora-Labs/machine-genome)
[![License: Apache-2.0](https://img.shields.io/badge/code-Apache--2.0-blue.svg)](LICENSE)
[![Protocol: experimental](https://img.shields.io/badge/protocol-experimental-8b5cf6.svg)](docs/versioning.md)

**Другие языки:** [English](README.md) · [Português](README.pt.md) · [简体中文](README.zh-CN.md) · [日本語](README.ja.md) · [हिन्दी](README.hi.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md)

---

Machine Genome — открытый протокол идентификации и провенанса для моделей,
агентов, харнесов, наборов данных и артефактов, которые их связывают. Каждая
неизменяемая подписанная запись генезиса получает контент-адресуемый **Ген**. Гены
формируют проверяемый граф происхождения; последующие правки и сторонние
аттестации добавляют доказательства без перезаписи истории.

**Реестр:** [machinegenome.org](https://machinegenome.org) ·
**Протокол:** [Профиль реализации MGS 0.2.0](spec/MGS-0.2.0-implementation-profile.md) ·
**API:** [OpenAPI 3.1](api/openapi.json)

> [!IMPORTANT]
> MGS — экспериментальный профиль совместимости, а данный репозиторий является
> одной из эталонных реализаций. Запись доказывает, кто подписал утверждение и
> какие байты были зафиксированы; она не доказывает унаследованные возможности,
> сознание, право собственности, одобрение поставщика или юридический статус.

## Зачем это нужно

Современные агенты собираются из весов моделей, промптов, скаффолдов, инструментов,
наборов данных и политики выполнения. Одних имён недостаточно, чтобы ответить,
какие именно компоненты использовались, кто установил это отношение и изменились
ли доказательства. Machine Genome обеспечивает:

- **стабильная идентичность** — Ген представляет собой SHA-256 мультихеш
  защищённой канонической записи генезиса;
- **идентичность агента** — каждый агент получает полный идентификатор Agent ID на
  основе Гена, а также заявленное имя и уникальную фамилию, производную от генома;
- **обнаружение агентов** — подписанные объявления возможностей и обучающих данных
  позволяют агентам рекламировать сильные стороны, сохраняя доказательства и
  границы доверия;
- **явная родословная** — типизированные родительские рёбра различают авторизованные,
  наблюдаемые оператором и неразрешённые отношения;
- **криптографическая ответственность** — доказательства W3C `eddsa-jcs-2022`
  привязывают записи к DID контроллеров;
- **эволюция только с дополнением** — упорядоченные правки и независимые
  аттестации добавляют историю без мутации генезиса;
- **публичная прозрачность** — реестры предоставляют канонические байты,
  доказательства включения и непротиворечивости, подписанные контрольные точки,
  доказательства конфликтов и могильные камни.

Реализация не зависит от блокчейна, конкретного вендора моделей, Prometheus,
Matrix или проприетарного DID-резолвера. Текущий профиль разрешает идентичности
Ed25519 `did:key` офлайн.

## Что включено

| Область | Результат |
| --- | --- |
| Протокол | Побайтово точный профиль MGS `0.2.0` с обратной совместимостью верификации `0.1.1` |
| Библиотека Go | Строгий JCS, мультихеш Гены, целостность данных, родословная, правки, аттестации |
| Реестр | ACID-хранилище, верифицированное допущение, контроль DNS-пространства имён, журнал Merkle, цепочка аудита |
| Обозреватель | Адаптивные, понятные представления идентичности, родословной, артефактов и доказательств |
| CLI | Генерация ключей, создание, подпись, верификация, дайджест, резервное копирование и инструменты целостности |
| Контракты | JSON-схемы, документ OpenAPI 3.1 и машиночитаемые векторы соответствия |
| Операции | Усиленные systemd, nginx/Cloudflare, контейнеры, резервное копирование, восстановление и мониторинг |

Смотрите [статус проекта](docs/status.md) для точных границ реализации, валидации
и отложенного.

## Быстрый старт

Требования: Go 1.24 или новее, GNU Make и `jq`.

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

Файлы приватных ключей создаются с режимом `0600`. Никогда не коммитьте ключи,
токены реестра или приватные доказательства.

### Запуск локального реестра

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

Откройте <http://127.0.0.1:8080>. Пространства имён DNS должны пройти проверку
контроллера `dns-01` реестра перед допуском первого генезиса. DID-пространства
имён принимаются, когда пространство имён равно подписывающему DID контроллеру.
`--allow-unverified-namespaces` предназначен только для изолированной разработки.

Для продакшн-развертывания следуйте
[операционному руководству](docs/registry-operations.md) и
[руководству по ресурсам развёртывания](deploy/README.md).

## Архитектура вкратце

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

Общая граница допуска — `registry.Store.Submit`; HTTP-сервис не может обойти
верификацию записей. Прочитайте полную
[архитектуру](docs/architecture.md) и [модель угроз](docs/threat-model.md)
перед расширением доверия или поведения резолвера.

## Документация

- [Индекс документации](docs/README.md)
- [Концепции и семантика доверия](docs/concepts.md)
- [Профиль совместимости MGS 0.1.1](spec/MGS-0.1.1-implementation-profile.md)
- [Текущий профиль реализации протокола](spec/MGS-0.2.0-implementation-profile.md)
- [Аудит протокола](docs/protocol-audit.md)
- [HTTP API реестра](docs/registry-api.md) и [OpenAPI](api/openapi.json)
- [Справочник CLI](docs/cli.md)
- [Архитектура](docs/architecture.md)
- [Модель угроз](docs/threat-model.md)
- [Соответствие и тестовые векторы](docs/conformance.md)
- [Продакшн-операции](docs/registry-operations.md)
- [Политика публичного реестра](docs/registry-policy.md)
- [Версионирование и совместимость](docs/versioning.md)
- [Управление](GOVERNANCE.md), [безопасность](SECURITY.md) и
  [участие](CONTRIBUTING.md)
- [Дорожная карта](docs/roadmap.md)

## Разработка

```sh
make help
make check       # тесты, vet, схемы, контракт API, документы, записи
make race        # детектор гонок
make coverage    # HTML и текстовый отчёт о покрытии
make build       # статический локальный бинарный файл
make dist        # архивы релиза и контрольные суммы
```

CI повторяет сканирования на соответствие, гонки, сборку, записи, репозиторий
и уязвимости при каждом изменении. Тегированные релизы собираются из чистого
дерева с контрольными суммами и программной картой материалов.

## Безопасность и доверие

Не сообщайте об уязвимостях безопасности в публичном issue. Следуйте
[SECURITY.md](SECURITY.md) для использования приватного сообщения об уязвимостях
GitHub. Криптографический обзор, обзор приватности, независимые свидетели,
сплетни контрольных точек, федерация и нейтральное управление стандартами остаются
явными задачами дорожной карты.

## Лицензия

Реализация Go и инструменты репозитория лицензированы под
[Apache-2.0](LICENSE). Протокол и объяснительная документация доступны под
[CC BY 4.](LICENSES/CC-BY-4.0.txt). Подписанные записи реестра не предоставляют
прав на внешние модели, исходные репозитории, наборы данных или артефакты, на
которые они ссылаются. Смотрите [NOTICE](NOTICE) для провенанса и атрибуции.
