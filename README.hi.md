<p align="center">
  <img src="docs/assets/machine-genome-mark.svg" width="88" height="88" alt="Machine Genome चिह्न">
</p>

# Machine Genome

[![Conformance](https://github.com/paxlabs-inc/machine-genome/actions/workflows/ci.yml/badge.svg)](https://github.com/paxlabs-inc/machine-genome/actions/workflows/ci.yml)
[![CodeQL](https://github.com/paxlabs-inc/machine-genome/actions/workflows/codeql.yml/badge.svg)](https://github.com/paxlabs-inc/machine-genome/actions/workflows/codeql.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/paxlabs-inc/machine-genome.svg)](https://pkg.go.dev/github.com/paxlabs-inc/machine-genome)
[![License: Apache-2.0](https://img.shields.io/badge/code-Apache--2.0-blue.svg)](LICENSE)
[![Protocol: experimental](https://img.shields.io/badge/protocol-experimental-8b5cf6.svg)](docs/versioning.md)

**अन्य भाषाएँ:** [English](README.md) · [Русский](README.ru.md) · [Português](README.pt.md) · [简体中文](README.zh-CN.md) · [日本語](README.ja.md) · [Deutsch](README.de.md) · [Français](README.fr.md) · [Español](README.es.md)

---

Machine Genome मॉडल, एजेंट, हार्नेस, डेटासेट और उन्हें जोड़ने वाली कलाकृतियों के लिए एक खुला पहचान और उत्पत्ति प्रोटोकॉल है। प्रत्येक अपरिवर्तनीय, हस्ताक्षरित जेनेसिस रिकॉर्ड को एक सामग्री-संबोधित **जीन (Gene)** प्राप्त होता है। जीन एक सत्यापन योग्य वंशावली ग्राफ बनाते हैं; बाद के संशोधन और तृतीय-पक्ष प्रमाणन इतिहास को फिर से लिखे बिना साक्ष्य जोड़ते हैं।

**लाइव रजिस्ट्री:** [machinegenome.org](https://machinegenome.org) ·
**प्रोटोकॉल:** [MGS 0.2.0 कार्यान्वयन प्रोफ़ाइल](spec/MGS-0.2.0-implementation-profile.md) ·
**API:** [OpenAPI 3.1](api/openapi.json)

> [!IMPORTANT]
> MGS एक प्रायोगिक अंतरसंचालनीयता प्रोफ़ाइल है और यह रिपॉज़िटरी एक संदर्भ कार्यान्वयन है। एक रिकॉर्ड यह साबित करता है कि किसी दावे पर किसने हस्ताक्षर किए और कौन से बाइट्स प्रतिबद्ध किए गए; यह विरासत में मिली क्षमता, चेतना, स्वामित्व, विक्रेता समर्थन या कानूनी स्थिति को साबित नहीं करता।

## यह क्यों मौजूद है

आधुनिक एजेंट मॉडल वेट, प्रॉम्प्ट, स्कैफोल्ड, टूल्स, डेटासेट और रनटाइम नीति से एकत्रित होते हैं। केवल नाम यह बता नहीं सकते कि कौन से सटीक घटक उपयोग किए गए, किसने संबंध का दावा किया, या साक्ष्य बदला या नहीं। Machine Genome प्रदान करता है:

- **स्थिर पहचान** — एक जीन एक सुरक्षित कैनोनिकल जेनेसिस रिकॉर्ड का SHA-256 मल्टीहैश है;
- **एजेंट पहचान** — प्रत्येक एजेंट को एक पूर्ण जीन-समर्थित Agent ID और एक घोषित नाम और जीनोम-व्युत्पन्न अद्वितीय उपनाम प्राप्त होता है;
- **एजेंट खोज** — हस्ताक्षरित क्षमता और प्रशिक्षण डेटा घोषणाएं एजेंटों को साक्ष्य और विश्वास सीमाओं को संरक्षित करते हुए अपनी ताकतों का प्रचार करने की अनुमति देती हैं;
- **स्पष्ट वंशावली** — टाइप किए गए पैरेंट एज अधिकृत, ऑपरेटर-अवलोकित और अनसुलझे संबंधों को अलग करते हैं;
- **क्रिप्टोग्राफिक जवाबदेही** — W3C `eddsa-jcs-2022` प्रमाण रिकॉर्ड को कंट्रोलर DID से जोड़ते हैं;
- **केवल-जोड़ें विकास** — क्रमबद्ध संशोधन और स्वतंत्र प्रमाणन जेनेसिस को बदले बिना इतिहास जोड़ते हैं;
- **सार्वजनिक पारदर्शिता** — रजिस्ट्री कैनोनिकल बाइट्स, समावेश और संगतता प्रमाण, हस्ताक्षरित चेकपॉइंट, संघर्ष साक्ष्य और टॉम्बस्टोन प्रदान करती है।

कार्यान्वयन को ब्लॉकचेन, विशिष्ट मॉडल विक्रेता, Prometheus, Matrix, या स्वामित्व DID रिज़ॉल्वर पर निर्भरता नहीं है। वर्तमान प्रोफ़ाइल Ed25519 `did:key` पहचानों को ऑफ़लाइन हल करती है।

## क्या शामिल है

| क्षेत्र | वितरण योग्य |
| --- | --- |
| प्रोटोकॉल | `0.1.1` सत्यापन संगतता के साथ बाइट-सटीक MGS `0.2.0` प्रोफ़ाइल |
| Go लाइब्रेरी | सख्त JCS, मल्टीहैश जीन, डेटा इंटीग्रिटी, वंशावली, संशोधन, प्रमाणन |
| रजिस्ट्री | ACID स्टोरेज, सत्यापित प्रवेश, DNS नेमस्पेस नियंत्रण, Merkle लॉग, ऑडिट चेन |
| एक्सप्लोरर | उत्तरदायी, सरल भाषा पहचान, वंशावली, कलाकृति और प्रमाण दृश्य |
| CLI | कुंजी जनरेशन, लेखन, हस्ताक्षर, सत्यापन, डाइजेस्ट, बैकअप और इंटीग्रिटी टूल्स |
| अनुबंध | JSON Schema, OpenAPI 3.1 दस्तावेज़ और मशीन-पठनीय अनुरूपता वैक्टर |
| संचालन | सुदृढ़ systemd, nginx/Cloudflare, कंटेनर, बैकअप, पुनर्स्थापना और निगरानी संपत्तियां |

सटीक कार्यान्वित, सत्यापित और स्थगित सीमाओं के लिए [प्रोजेक्ट स्थिति](docs/status.md) देखें।

## त्वरित शुरुआत

आवश्यकताएं: Go 1.24 या नया, GNU Make, और `jq`।

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

निजी कुंजी फ़ाइलें मोड `0600` के साथ बनाई जाती हैं। कभी भी कुंजियों, रजिस्ट्री टोकन, या निजी साक्ष्य को कमिट न करें।

### स्थानीय रजिस्ट्री चलाएं

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

<http://127.0.0.1:8080> खोलें। DNS नेमस्पेस को अपना पहला जेनेसिस स्वीकार होने से पहले रजिस्ट्री की `dns-01` कंट्रोलर चुनौती पूरी करनी होगी। DID नेमस्पेस तब स्वीकार किए जाते हैं जब नेमस्पेस हस्ताक्षर करने वाले कंट्रोलर DID के बराबर हो। `--allow-unverified-namespaces` केवल अलग-थलग विकास के लिए है।

उत्पादन परिनियोजन के लिए, [संचालन रनबुक](docs/registry-operations.md) और [परिनियोजन संपत्ति गाइड](deploy/README.md) का पालन करें।

## एक नज़र में वास्तुकला

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

साझा प्रवेश सीमा `registry.Store.Submit` है; HTTP सेवा रिकॉर्ड सत्यापन को बाइपास नहीं कर सकती। विश्वास या रिज़ॉल्वर व्यवहार का विस्तार करने से पहले पूर्ण [वास्तुकला](docs/architecture.md) और [खतरा मॉडल](docs/threat-model.md) पढ़ें।

## दस्तावेज़ीकरण

- [दस्तावेज़ अनुक्रमणिका](docs/README.md)
- [अवधारणाएं और विश्वास शब्दार्थ](docs/concepts.md)
- [MGS 0.1.1 संगतता प्रोफ़ाइल](spec/MGS-0.1.1-implementation-profile.md)
- [वर्तमान प्रोटोकॉल कार्यान्वयन प्रोफ़ाइल](spec/MGS-0.2.0-implementation-profile.md)
- [प्रोटोकॉल ऑडिट](docs/protocol-audit.md)
- [रजिस्ट्री HTTP API](docs/registry-api.md) और [OpenAPI](api/openapi.json)
- [CLI संदर्भ](docs/cli.md)
- [वास्तुकला](docs/architecture.md)
- [खतरा मॉडल](docs/threat-model.md)
- [अनुरूपता और परीक्षण वैक्टर](docs/conformance.md)
- [उत्पादन संचालन](docs/registry-operations.md)
- [सार्वजनिक रजिस्ट्री नीति](docs/registry-policy.md)
- [संस्करण और संगतता](docs/versioning.md)
- [शासन](GOVERNANCE.md), [सुरक्षा](SECURITY.md), और [योगदान](CONTRIBUTING.md)
- [रोडमैप](docs/roadmap.md)

## विकास

```sh
make help
make check       # परीक्षण, vet, स्कीमा, API अनुबंध, दस्तावेज़, रिकॉर्ड
make race        # रेस डिटेक्टर
make coverage    # HTML और पाठ्य कवरेज रिपोर्ट
make build       # स्थैतिक स्थानीय बाइनरी
make dist        # रिलीज़ संग्रह और चेकसम
```

CI प्रत्येक परिवर्तन पर अनुरूपता, रेस, बिल्ड, रिकॉर्ड, रिपॉज़िटरी और भेद्यता स्कैन दोहराता है। टैग की गई रिलीज़ चेकसम और सॉफ़्टवेयर सामग्री सूची के साथ एक स्वच्छ ट्री से बनाई जाती हैं।

## सुरक्षा और विश्वास

सार्वजनिक issue में सुरक्षा भेद्यता की रिपोर्ट न करें। GitHub की निजी भेद्यता रिपोर्टिंग का उपयोग करने के लिए [SECURITY.md](SECURITY.md) का पालन करें। क्रिप्टोग्राफिक समीक्षा, गोपनीयता समीक्षा, स्वतंत्र गवाह, चेकपॉइंट गॉसिप, फेडरेशन, और तटस्थ मानक शासन स्पष्ट रोडमैप कार्य बने हुए हैं।

## लाइसेंस

Go कार्यान्वयन और रिपॉज़िटरी टूलिंग [Apache-2.0](LICENSE) के तहत लाइसेंस प्राप्त हैं। प्रोटोकॉल और व्याख्यात्मक दस्तावेज़ [CC BY 4.0](LICENSES/CC-BY-4.0.txt) के तहत उपलब्ध हैं। हस्ताक्षरित रजिस्ट्री रिकॉर्ड उन बाहरी मॉडलों, स्रोत रिपॉज़िटरी, डेटासेट या कलाकृतियों पर अधिकार प्रदान नहीं करते जिनका वे संदर्भ देते हैं। उत्पत्ति और श्रेय के लिए [NOTICE](NOTICE) देखें।
