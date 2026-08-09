<p align="center">
  <img src="docs/assets/machine-genome-mark.svg" width="88" height="88" alt="Machine Genome marca">
</p>

# Machine Genome

[![Conformance](https://github.com/paxlabs-inc/machine-genome/actions/workflows/ci.yml/badge.svg)](https://github.com/paxlabs-inc/machine-genome/actions/workflows/ci.yml)
[![CodeQL](https://github.com/paxlabs-inc/machine-genome/actions/workflows/codeql.yml/badge.svg)](https://github.com/paxlabs-inc/machine-genome/actions/workflows/codeql.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/paxlabs-inc/machine-genome.svg)](https://pkg.go.dev/github.com/paxlabs-inc/machine-genome)
[![License: Apache-2.0](https://img.shields.io/badge/code-Apache--2.0-blue.svg)](LICENSE)
[![Protocol: experimental](https://img.shields.io/badge/protocol-experimental-8b5cf6.svg)](docs/versioning.md)

Machine Genome es un protocolo abierto de identidad y procedencia para modelos,
agentes, harnesses, conjuntos de datos y los artefactos que los conectan. Cada
registro de génesis inmutable y firmado recibe un **Gen** con direccionamiento
por contenido. Los genes forman un grafo de linaje verificable; las enmiendas
posteriores y las atestaciones de terceros agregan evidencia sin reescribir la
historia.

**Registro en vivo:** [machinegenome.org](https://machinegenome.org) ·
**Protocolo:** [Perfil de implementación MGS 0.2.0](spec/MGS-0.2.0-implementation-profile.md) ·
**API:** [OpenAPI 3.1](api/openapi.json)

> [!IMPORTANT]
> MGS es un perfil experimental de interoperabilidad y este repositorio es una
> de las implementaciones de referencia. Un registro prueba quién firmó una
> declaración y qué bytes se comprometieron; no prueba capacidad heredada,
> conciencia, propiedad, respaldo del proveedor ni estatus legal.

## Por qué esto existe

Los agentes modernos se ensamblan a partir de pesos de modelos, prompts,
scaffolds, herramientas, conjuntos de datos y políticas de runtime. Los nombres
solos no pueden responder qué componentes exactos se usaron, quién afirmó la
relación o si la evidencia cambió. Machine Genome proporciona:

- **identidad estable** — un Gen es un multihash SHA-256 de un registro de
  génesis canónico protegido;
- **identidad de agente** — cada agente recibe un Agent ID completo respaldado
  por Gen, además de un nombre declarado y un apellido único derivado del genoma;
- **descubrimiento de agentes** — las declaraciones firmadas de capacidades y
  datos de entrenamiento permiten a los agentes anunciar fortalezas mientras
  preservan la evidencia y los límites de confianza;
- **linaje explícito** — las aristas padre tipificadas distinguen relaciones
  autorizadas, observadas por el operador y no resueltas;
- **responsabilidad criptográfica** — las pruebas W3C `eddsa-jcs-2022` vinculan
  registros a DIDs de controladores;
- **evolución solo por adición** — las enmiendas ordenadas y las atestaciones
  independientes agregan historia sin mutar el génesis;
- **transparencia pública** — los registros proporcionan bytes canónicos,
  pruebas de inclusión y consistencia, puntos de verificación firmados,
  evidencia de conflicto y lápidas.

La implementación no depende de blockchain, un proveedor de modelos específico,
Prometheus, Matrix o un resolvedor DID propietario. El perfil actual resuelve
identidades Ed25519 `did:key` sin conexión.

## Qué está incluido

| Área | Entregable |
| --- | --- |
| Protocolo | Perfil MGS `0.2.0` exacto a nivel de byte con compatibilidad de verificación `0.1.1` |
| Biblioteca Go | JCS estricto, Genes multihash, integridad de datos, linaje, enmiendas, atestaciones |
| Registro | Almacenamiento ACID, admisión verificada, control de namespace DNS, log Merkle, cadena de auditoría |
| Explorador | Vistas responsivas y en lenguaje claro de identidad, linaje, artefactos y pruebas |
| CLI | Generación de claves, creación, firma, verificación, digest, respaldo y herramientas de integridad |
| Contratos | JSON Schemas, documento OpenAPI 3.1 y vectores de conformidad legibles por máquina |
| Operaciones | Assets de systemd endurecido, nginx/Cloudflare, contenedor, respaldo, restauración y monitoreo |

Consulte [estado del proyecto](docs/status.md) para los límites precisos
implementados, validados y diferidos.

## Inicio rápido

Requisitos: Go 1.24 o más reciente, GNU Make y `jq`.

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

Los archivos de clave privada se crean con modo `0600`. Nunca haga commit de
claves, tokens de registro o evidencia privada.

### Ejecutar un registro local

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

Abra <http://127.0.0.1:8080>. Los namespaces DNS deben completar el desafío
`dns-01` del controlador del registro antes de que su primer génesis sea
admitido. Los namespaces DID se aceptan cuando el namespace es igual al DID
del controlador firmante. `--allow-unverified-namespaces` es solo para
desarrollo aislado.

Para un despliegue en producción, siga el
[runbook de operaciones](docs/registry-operations.md) y la
[guía de assets de despliegue](deploy/README.md).

## Arquitectura de un vistazo

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

El límite de admisión compartido es `registry.Store.Submit`; el servicio HTTP
no puede eludir la verificación de registros. Lea la
[arquitectura](docs/architecture.md) completa y el [modelo de amenazas](docs/threat-model.md)
antes de extender la confianza o el comportamiento del resolvedor.

## Documentación

- [Índice de documentación](docs/README.md)
- [Conceptos y semántica de confianza](docs/concepts.md)
- [Perfil de compatibilidad MGS 0.1.1](spec/MGS-0.1.1-implementation-profile.md)
- [Perfil actual de implementación del protocolo](spec/MGS-0.2.0-implementation-profile.md)
- [Auditoría del protocolo](docs/protocol-audit.md)
- [HTTP API del registro](docs/registry-api.md) y [OpenAPI](api/openapi.json)
- [Referencia CLI](docs/cli.md)
- [Arquitectura](docs/architecture.md)
- [Modelo de amenazas](docs/threat-model.md)
- [Conformidad y vectores de prueba](docs/conformance.md)
- [Operaciones de producción](docs/registry-operations.md)
- [Política del registro público](docs/registry-policy.md)
- [Versionado y compatibilidad](docs/versioning.md)
- [Gobernanza](GOVERNANCE.md), [seguridad](SECURITY.md) y
  [contribución](CONTRIBUTING.md)
- [Hoja de ruta](docs/roadmap.md)

## Desarrollo

```sh
make help
make check       # pruebas, vet, schemas, contrato API, docs, registros
make race        # detector de carreras
make coverage    # informe de cobertura HTML y textual
make build       # binario local estático
make dist        # archivos de release y checksums
```

CI repite verificaciones de conformidad, carreras, build, registros, repositorio
y vulnerabilidades en cada cambio. Los releases etiquetados se construyen desde
un árbol limpio con checksums y una lista de materiales de software.

## Seguridad y confianza

No reporte vulnerabilidades de seguridad en un issue público. Siga
[SECURITY.md](SECURITY.md) para usar el reporte privado de vulnerabilidades
de GitHub. La revisión criptográfica, la revisión de privacidad, los testigos
independientes, el chisme de puntos de verificación, la federación y la
gobernanza neutra de estándares permanecen como trabajo explícito de la hoja
de ruta.

## Licencia

La implementación Go y las herramientas del repositorio están licenciadas bajo
[Apache-2.0](LICENSE). La documentación del protocolo y explicativa está
disponible bajo [CC BY 4.0](LICENSES/CC-BY-4.0.txt). Los registros firmados
del registro no otorgan derechos sobre los modelos externos, repositorios
fuente, conjuntos de datos o artefactos que referencian. Consulte [NOTICE](NOTICE)
para procedencia y atribución.
