<p align="center">
  <img src="docs/assets/machine-genome-mark.svg" width="88" height="88" alt="Machine Genome Zeichen">
</p>

# Machine Genome

[![Conformance](https://github.com/Sidiora-Labs/machine-genome/actions/workflows/ci.yml/badge.svg)](https://github.com/Sidiora-Labs/machine-genome/actions/workflows/ci.yml)
[![CodeQL](https://github.com/Sidiora-Labs/machine-genome/actions/workflows/codeql.yml/badge.svg)](https://github.com/Sidiora-Labs/machine-genome/actions/workflows/codeql.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/Sidiora-Labs/machine-genome.svg)](https://pkg.go.dev/github.com/Sidiora-Labs/machine-genome)
[![License: Apache-2.0](https://img.shields.io/badge/code-Apache--2.0-blue.svg)](LICENSE)
[![Protocol: experimental](https://img.shields.io/badge/protocol-experimental-8b5cf6.svg)](docs/versioning.md)

**Weitere Sprachen:** [English](README.md) · [Русский](README.ru.md) · [Português](README.pt.md) · [简体中文](README.zh-CN.md) · [日本語](README.ja.md) · [हिन्दी](README.hi.md) · [Français](README.fr.md) · [Español](README.es.md)

---

Machine Genome ist ein offenes Identitäts- und Herkunftsprotokoll für Modelle,
Agenten, Testumgebungen, Datensätze und die Artefakte, die sie verbinden. Jeder
unveränderliche, signierte Genesis-Datensatz erhält ein inhaltsadressiertes
**Gen**. Gene bilden einen überprüfbaren Abstammungsgraphen; spätere Änderungen
und Drittattestationen fügen Beweise hinzu, ohne die Historie umzuschreiben.

**Live-Register:** [machinegenome.org](https://machinegenome.org) ·
**Protokoll:** [MGS 0.2.0 Implementierungsprofil](spec/MGS-0.2.0-implementation-profile.md) ·
**API:** [OpenAPI 3.1](api/openapi.json)

> [!IMPORTANT]
> MGS ist ein experimentelles Interoperabilitätsprofil und dieses Repository ist
> eine Referenzimplementierung. Ein Datensatz beweist, wer eine Aussage signiert
> hat und welche Bytes übertragen wurden; er beweist nicht vererbte Fähigkeiten,
> Bewusstsein, Eigentumsrecht, Herstellerbestätigung oder rechtlichen Status.

## Warum das existiert

Moderne Agenten werden aus Modellgewichten, Prompts, Gerüsten, Tools, Datensätzen
und Laufzeitrichtlinien zusammengestellt. Allein Namen können nicht beantworten,
welche genauen Komponenten verwendet wurden, wer die Beziehung behauptet hat oder
ob sich die Beweise geändert haben. Machine Genome bietet:

- **stabile Identität** — ein Gen ist ein SHA-256-Multihash eines gesicherten
  kanonischen Genesis-Datensatzes;
- **Agentenidentität** — jeder Agent erhält eine vollständige Gen-gestützte
  Agent-ID sowie einen angegebenen Vornamen und einen genom-abgeleiteten
  einzigartigen Nachnamen;
- **Agentenentdeckung** — signierte Fähigkeits- und Trainingsdaten-Deklarationen
  ermöglichen es Agenten, Stärken zu bewerben und dabei Beweise und
  Vertrauensgrenzen zu bewahren;
- **explizite Abstammung** — typisierte Elternkanten unterscheiden autorisierte,
  vom Betreiber beobachtete und ungelöste Beziehungen;
- **kryptographische Rechenschaftspflicht** — W3C `eddsa-jcs-2022`-Beweise
  binden Datensätze an Controller-DIDs;
- **Nur-Anhängen-Entwicklung** — geordnete Änderungen und unabhängige
  Attestationen fügen Historie hinzu, ohne den Genesis zu verändern;
- **öffentliche Transparenz** — Register bieten kanonische Bytes, Einschluss-
  und Konsistenznachweise, signierte Prüfpunkte, Konfliktnachweise und
  Grabsteine.

Die Implementierung hat keine Abhängigkeit von einer Blockchain, einem
bestimmten Modellanbieter, Prometheus, Matrix oder einem proprietären
DID-Resolver. Das aktuelle Profil löst Ed25519 `did:key`-Identitäten offline auf.

## Was enthalten ist

| Bereich | Lieferobjekt |
| --- | --- |
| Protokoll | Byte-exaktes MGS `0.2.0`-Profil mit `0.1.1`-Verifizierungskompatibilität |
| Go-Bibliothek | Strenge JCS, Multihash-Gene, Datenintegrität, Abstammung, Änderungen, Attestationen |
| Register | ACID-Speicher, verifizierte Zulassung, DNS-Namespace-Kontrolle, Merkle-Protokoll, Auditkette |
| Explorer | Responsive, verständliche Identitäts-, Abstammungs-, Artefakt- und Nachweisansichten |
| CLI | Schlüsselerzeugung, Erstellung, Signatur, Verifizierung, Digest, Backup und Integritätstools |
| Verträge | JSON-Schemas, OpenAPI 3.1-Dokument und maschinenlesbare Konformitätsvektoren |
| Betrieb | Gehärtete systemd, nginx/Cloudflare, Container, Backup, Wiederherstellung und Überwachungsressourcen |

Siehe [Projektstatus](docs/status.md) für die genauen implementierten, validierten
und aufgeschobenen Grenzen.

## Schnellstart

Anforderungen: Go 1.24 oder neuer, GNU Make und `jq`.

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

Private Schlüsseldateien werden mit Modus `0600` erstellt. Committen Sie
niemals Schlüssel, Register-Token oder private Beweise.

### Lokales Register starten

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

Öffnen Sie <http://127.0.0.1:8080>. DNS-Namespaces müssen die `dns-01`-
Controller-Herausforderung des Registers abschließen, bevor ihr erster Genesis
zugelassen wird. DID-Namespaces werden akzeptiert, wenn der Namespace dem
signierenden Controller-DID entspricht. `--allow-unverified-namespaces` ist
nur für isolierte Entwicklung gedacht.

Für ein Produktions-Deployment folgen Sie dem
[Betriebshandbuch](docs/registry-operations.md) und dem
[Deployment-Asset-Leitfaden](deploy/README.md).

## Architektur im Überblick

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

Die gemeinsame Zulassungsgrenze ist `registry.Store.Submit`; der HTTP-Dienst
kann die Datensatzverifizierung nicht umgehen. Lesen Sie die vollständige
[Architektur](docs/architecture.md) und das [Bedrohungsmodell](docs/threat-model.md),
bevor Sie Vertrauen oder Resolver-Verhalten erweitern.

## Dokumentation

- [Dokumentationsindex](docs/README.md)
- [Konzepte und Vertrauenssemantik](docs/concepts.md)
- [MGS 0.1.1 Kompatibilitätsprofil](spec/MGS-0.1.1-implementation-profile.md)
- [Aktuelles Protokollimplementierungsprofil](spec/MGS-0.2.0-implementation-profile.md)
- [Protokollaudit](docs/protocol-audit.md)
- [Register-HTTP-API](docs/registry-api.md) und [OpenAPI](api/openapi.json)
- [CLI-Referenz](docs/cli.md)
- [Architektur](docs/architecture.md)
- [Bedrohungsmodell](docs/threat-model.md)
- [Konformität und Testvektoren](docs/conformance.md)
- [Produktionsbetrieb](docs/registry-operations.md)
- [Öffentliche Registerpolitik](docs/registry-policy.md)
- [Versionierung und Kompatibilität](docs/versioning.md)
- [Governance](GOVERNANCE.md), [Sicherheit](SECURITY.md) und
  [Mitwirkung](CONTRIBUTING.md)
- [Roadmap](docs/roadmap.md)

## Entwicklung

```sh
make help
make check       # Tests, Vet, Schemata, API-Vertrag, Dokumente, Datensätze
make race        # Race-Detektor
make coverage    # HTML- und Text-Coverage-Bericht
make build       # Statisches lokales Binärprogramm
make dist        # Release-Archive und Prüfsummen
```

CI wiederholt Konformitäts-, Race-, Build-, Datensatz-, Repository- und
Verwundbarkeitsscans bei jeder Änderung. Getaggte Releases werden aus einem
sauberen Tree mit Prüfsummen und einer Software-Stückliste erstellt.

## Sicherheit und Vertrauen

Melden Sie keine Sicherheitslücken in einem öffentlichen Issue. Folgen Sie
[SECURITY.md](SECURITY.md), um GitHub's private Verwundbarkeitsmeldung zu
verwenden. Kryptographische Überprüfung, Datenschutzüberprüfung, unabhängige
Zeugen, Prüfpunkt-Gossip, Föderation und neutrale Standards-Governance bleiben
explizite Roadmap-Arbeiten.

## Lizenz

Die Go-Implementierung und die Repository-Werkzeuge sind unter
[Apache-2.0](LICENSE) lizenziert. Protokoll- und erläuternde Dokumentation
stehen unter [CC BY 4.0](LICENSES/CC-BY-4.0.txt) zur Verfügung. Signierte
Registerdatensätze gewähren keine Rechte an den externen Modellen,
Quellrepositories, Datensätzen oder Artefakten, auf die sie verweisen. Siehe
[NOTICE](NOTICE) für Herkunft und Zuschreibung.
