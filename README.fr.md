<p align="center">
  <img src="docs/assets/machine-genome-mark.svg" width="88" height="88" alt="Machine Genome marque">
</p>

# Machine Genome

[![Conformance](https://github.com/paxlabs-inc/machine-genome/actions/workflows/ci.yml/badge.svg)](https://github.com/paxlabs-inc/machine-genome/actions/workflows/ci.yml)
[![CodeQL](https://github.com/paxlabs-inc/machine-genome/actions/workflows/codeql.yml/badge.svg)](https://github.com/paxlabs-inc/machine-genome/actions/workflows/codeql.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/paxlabs-inc/machine-genome.svg)](https://pkg.go.dev/github.com/paxlabs-inc/machine-genome)
[![License: Apache-2.0](https://img.shields.io/badge/code-Apache--2.0-blue.svg)](LICENSE)
[![Protocol: experimental](https://img.shields.io/badge/protocol-experimental-8b5cf6.svg)](docs/versioning.md)

Machine Genome est un protocole ouvert d'identité et de provenance pour les
modèles, les agents, les harnesses, les ensembles de données et les artefacts
qui les relient. Chaque enregistrement de genèse immuable et signé reçoit un
**Gène** adressé par contenu. Les gènes forment un graphe de lignée vérifiable ;
les amendements ultérieurs et les attestations tierces ajoutent des preuves sans
réécrire l'historique.

**Registre en direct :** [machinegenome.org](https://machinegenome.org) ·
**Protocole :** [Profil d'implémentation MGS 0.2.0](spec/MGS-0.2.0-implementation-profile.md) ·
**API :** [OpenAPI 3.1](api/openapi.json)

> [!IMPORTANT]
> MGS est un profil d'interopérabilité expérimental et ce dépôt est l'une des
> implémentations de référence. Un enregistrement prouve qui a signé une
> déclaration et quels octets ont été engagés ; il ne prouve pas les capacités
> héritées, la conscience, la propriété, l'approbation du fournisseur ou le
> statut juridique.

## Pourquoi cela existe

Les agents modernes sont assemblés à partir de poids de modèles, de prompts,
d'échafaudages, d'outils, d'ensembles de données et de politiques d'exécution.
Les seuls noms ne peuvent pas répondre à la question de savoir quels composants
exacts ont été utilisés, qui a affirmé la relation ou si les preuves ont changé.
Machine Genome fournit :

- **identité stable** — un Gène est un multihash SHA-256 d'un enregistrement
  de genèse canonique sécurisé ;
- **identité d'agent** — chaque agent reçoit un Agent ID complet soutenu par un
  Gène, ainsi qu'un prénom déclaré et un nom de famille unique dérivé du génome ;
- **découverte d'agents** — les déclarations signées de capacités et de données
  d'entraînement permettent aux agents de promouvoir leurs forces tout en
  préservant les preuves et les limites de confiance ;
- **lignée explicite** — les arêtes parentes typées distinguent les relations
  autorisées, observées par l'opérateur et non résolues ;
- **responsabilité cryptographique** — les preuves W3C `eddsa-jcs-2022` lient
  les enregistrements aux DID des contrôleurs ;
- **évolution en ajout seul** — les amendements ordonnés et les attestations
  indépendantes ajoutent de l'historique sans muter la genèse ;
- **transparence publique** — les registres fournissent des octets canoniques,
  des preuves d'inclusion et de cohérence, des points de contrôle signés, des
  preuves de conflit et des pierres tombales.

L'implémentation n'a aucune dépendance envers une blockchain, un fournisseur
de modèles spécifique, Prometheus, Matrix ou un résolveur DID propriétaire.
Le profil actuel résout les identités Ed25519 `did:key` hors ligne.

## Ce qui est inclus

| Domaine | Livrable |
| --- | --- |
| Protocole | Profil MGS `0.2.0` exact au niveau de l'octet avec compatibilité de vérification `0.1.1` |
| Bibliothèque Go | JCS strict, Gènes multihash, intégrité des données, lignée, amendements, attestations |
| Registre | Stockage ACID, admission vérifiée, contrôle de namespace DNS, journal Merkle, chaîne d'audit |
| Explorateur | Vues responsives et en langage clair de l'identité, de la lignée, des artefacts et des preuves |
| CLI | Génération de clés, création, signature, vérification, digest, sauvegarde et outils d'intégrité |
| Contrats | JSON Schemas, document OpenAPI 3.1 et vecteurs de conformité lisibles par machine |
| Opérations | Assets systemd renforcé, nginx/Cloudflare, conteneur, sauvegarde, restauration et surveillance |

Voir [statut du projet](docs/status.md) pour les limites précises implémentées,
validées et différées.

## Démarrage rapide

Prérequis : Go 1.24 ou plus récent, GNU Make et `jq`.

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

Les fichiers de clé privée sont créés avec le mode `0600`. Ne commitez jamais
les clés, les jetons de registre ou les preuves privées.

### Exécuter un registre local

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

Ouvrez <http://127.0.0.1:8080>. Les namespaces DNS doivent relever le défi
`dns-01` du contrôleur du registre avant que leur première genèse soit admise.
Les namespaces DID sont acceptés lorsque le namespace est égal au DID du
contrôleur signataire. `--allow-unverified-namespaces` est réservé au
développement isolé.

Pour un déploiement en production, suivez le
[runbook d'opérations](docs/registry-operations.md) et le
[guide des assets de déploiement](deploy/README.md).

## Architecture en un coup d'œil

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

La limite d'admission partagée est `registry.Store.Submit` ; le service HTTP
ne peut pas contourner la vérification des enregistrements. Lisez l'
[architecture](docs/architecture.md) complète et le [modèle de menaces](docs/threat-model.md)
avant d'étendre la confiance ou le comportement du résolveur.

## Documentation

- [Index de la documentation](docs/README.md)
- [Concepts et sémantique de confiance](docs/concepts.md)
- [Profil de compatibilité MGS 0.1.1](spec/MGS-0.1.1-implementation-profile.md)
- [Profil d'implémentation actuel du protocole](spec/MGS-0.2.0-implementation-profile.md)
- [Audit du protocole](docs/protocol-audit.md)
- [HTTP API du registre](docs/registry-api.md) et [OpenAPI](api/openapi.json)
- [Référence CLI](docs/cli.md)
- [Architecture](docs/architecture.md)
- [Modèle de menaces](docs/threat-model.md)
- [Conformité et vecteurs de test](docs/conformance.md)
- [Opérations de production](docs/registry-operations.md)
- [Politique du registre public](docs/registry-policy.md)
- [Versionnage et compatibilité](docs/versioning.md)
- [Gouvernance](GOVERNANCE.md), [sécurité](SECURITY.md) et
  [contribution](CONTRIBUTING.md)
- [Feuille de route](docs/roadmap.md)

## Développement

```sh
make help
make check       # tests, vet, schémas, contrat API, docs, enregistrements
make race        # détecteur de courses
make coverage    # rapport de couverture HTML et textuel
make build       # binaire local statique
make dist        # archives de release et sommes de contrôle
```

CI répète les vérifications de conformité, de courses, de build, d'enregistrements,
de dépôt et de vulnérabilités à chaque changement. Les releases taguées sont
construites à partir d'un arbre propre avec des sommes de contrôle et une nomenclature
des logiciels.

## Sécurité et confiance

Ne signalez pas les vulnérabilités de sécurité dans un issue public. Suivez
[SECURITY.md](SECURITY.md) pour utiliser le signalement privé de vulnérabilités
de GitHub. La revue cryptographique, la revue de confidentialité, les témoins
indépendants, le potin de points de contrôle, la fédération et la gouvernance
neutre des standards restent des travaux explicites de la feuille de route.

## Licence

L'implémentation Go et les outils du dépôt sont sous licence
[Apache-2.0](LICENSE). La documentation protocolaire et explicative est
disponible sous [CC BY 4.0](LICENSES/CC-BY-4.0.txt). Les enregistrements signés
du registre n'accordent aucun droit sur les modèles externes, les dépôts source,
les ensembles de données ou les artefats qu'ils référencent. Voir [NOTICE](NOTICE)
pour la provenance et l'attribution.
