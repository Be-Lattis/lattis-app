# Choix de l'autorité de signature de code (Windows)

Étude d'octobre 2026. Depuis le 1er juin 2023, la clé privée d'un certificat de signature de code OV ou EV doit vivre sur un matériel certifié (jeton USB ou HSM) : plus de `.pfx` exportable. Pour signer depuis GitHub Actions, il faut donc soit le service de signature cloud d'une autorité, soit Azure Artifact Signing (ex-Trusted Signing). Depuis 2024, un certificat EV ne lève plus l'avertissement SmartScreen d'emblée : OV et EV construisent la réputation de la même façon, l'EV ne se justifie plus pour Lattis ([Microsoft Learn](https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/code-signing-options)).

Ce que le workflow doit signer à chaque release : `lattis.exe`, le désinstalleur NSIS, l'installeur NSIS et le paquet MSIX, soit **4 signatures par run**. Le MSIX impose que son Publisher soit le sujet exact du certificat, ce que gère déjà `build/windows/msix/build-msix.ps1`.

## Comparatif

Prix hors taxes, relevés en octobre 2026 ; les prix des revendeurs sont indiqués comme tels.

| Autorité / service | Prix annuel (OV, 1 an) | Signature depuis GitHub Actions | MSIX et `.exe` | Validation pour une petite structure |
|---|---|---|---|---|
| **SSL.com eSigner** | Certificat OV ou IV **129 $** + forfait eSigner Tier 1 **180 $/an** (20 signatures/mois, report des signatures non utilisées) : **≈ 309 $/an**. 30 jours d'essai gratuits, signatures illimitées. | Officiel : action `SSLcom/esigner-codesign` (CodeSignTool) et **eSigner CKA**, fournisseur de clés Windows qui rend le certificat cloud visible par signtool, avec un guide GitHub Actions. | Oui avec eSigner CKA : tout ce que signtool signe, donc `.exe`, désinstalleur NSIS et MSIX. (L'action CodeSignTool ne cite pas le MSIX.) | OV : entité inscrite dans un registre public (SIREN/Kbis) ou Dun & Bradstreet ; pièce d'identité du demandeur si l'entreprise a moins de 3 ans. IV pour une personne physique : pièce d'identité recto verso et photo. 3 à 5 jours ouvrés. |
| Azure Artifact Signing (ex-Trusted Signing) | ≈ **120 $/an** (9,99 $/mois, 5 000 signatures/mois) | Officiel : action `azure/trusted-signing-action` (déjà dans le workflow). | Oui (`.exe`, MSIX) ; pas le désinstalleur NSIS embarqué. | Organisations des États-Unis, du Canada, de l'UE et du Royaume-Uni ; particuliers seulement aux États-Unis et au Canada. Microsoft refuse les organisations de moins de 3 ans d'historique vérifiable (règle appliquée depuis 2025). |
| Certum SimplySign | Standard cloud **209 €** TTC (Open Source cloud 49 €, réservé aux projets open source) | Pas d'outil officiel pour la CI : SimplySign Desktop demande un code à usage unique de l'application mobile ; seuls des contournements communautaires (extraction de la graine TOTP) existent. | Oui en local (signtool via la carte virtuelle). | Entreprise ou particulier ; validation par documents, prix bas. |
| DigiCert KeyLocker | ≈ **400 $/an** chez les revendeurs, KeyLocker compris, **1 000 signatures par an** (plafond dur, packs de 1 000 en sus) | Client `smctl` et fournisseur de clés pour signtool, intégrables dans un workflow. | Oui (signtool). | OV classique ; tarif le plus élevé. |
| Sectigo | À partir de **≈ 429 $/an** (via revendeurs) | Pas de service cloud propre accessible à une petite structure : jeton USB, ou HSM cloud que l'on héberge soi-même. | Oui en local avec jeton. | OV classique, 3 à 5 jours. |
| GlobalSign | ≈ **434 $/an** + Azure Key Vault Premium (≈ 5 $/mois) | Via AzureSignTool et une clé dans Azure Key Vault (guide officiel pour Jenkins seulement). | `.exe` oui ; MSIX non documenté avec AzureSignTool. | OV classique ; mise en place lourde. |

## Décision (octobre 2026)

Les certificats payants sont jugés trop chers pour Lattis, qui passe sous licence libre (MPL-2.0). Retenu, et inscrit à la feuille de route :

- **SignPath Foundation** : signature gratuite des projets open source admis, depuis GitHub Actions, avec un certificat de confiance (`.exe` et MSIX).
- **Microsoft Store** pour le MSIX : compte développeur gratuit, et Microsoft re-signe lui-même le paquet après certification, sans avertissement SmartScreen.

En attendant, le workflow garde les voies `.pfx` et Azure Trusted Signing ([signature.md](signature.md)) ; sans secret, les paquets Windows sortent non signés. Une intégration SSL.com eSigner (eSigner CKA + signtool) a été écrite puis retirée ; elle reste dans l'historique git si l'on y revient. Plan B si SignPath refuse le projet : Certum Open Source (49 €/an).

## Sources

- Microsoft, [Code signing options for Windows app developers](https://learn.microsoft.com/en-us/windows/apps/package-and-deploy/code-signing-options) (août 2026) : prix et éligibilité d'Artifact Signing, fin de l'avantage SmartScreen de l'EV.
- Microsoft Learn, [renouvellement de la validation d'identité Trusted Signing](https://learn.microsoft.com/en-us/azure/trusted-signing/how-to-renew-identity-validation) et [Q&A sur le refus d'une organisation de moins de 3 ans](https://learn.microsoft.com/answers/a/1843216).
- SSL.com, [OV Code Signing, achat](https://www.ssl.com/certificates/code-signing/buy/) (129 $/an), [IV Code Signing](https://ssl.com/products/software-integrity/code-signing/iv/), [tarifs eSigner](https://ssl.com/guide/esigner-pricing-for-code-signing/), [exigences de validation OV/IV](https://ssl.com/faqs/ssl-ov-validation-requirements/).
- SSL.com, [intégration d'eSigner CKA dans la CI](https://ssl.com/how-to/how-to-integrate-esigner-cka-with-ci-cd-tools-for-automated-code-signing/), [action GitHub CodeSignTool](https://ssl.com/how-to/cloud-code-signing-integration-with-github-actions), [versions d'eSigner CKA](https://github.com/SSLcom/eSignerCKA/releases) ; exemple réel : [appveyor.yml de GDevelop](https://github.com/4ian/GDevelop/blob/master/appveyor.yml).
- Certum, [boutique Code Signing](https://shop.certum.eu/code-signing.html) (prix, validité limitée à 459 jours depuis février 2026).
- DigiCert, [licences KeyLocker](https://docs.digicert.com/en/digicert-keylocker/overview/licensing.html) ; prix revendeur : [SSL Dragon](https://www.ssldragon.com/ssl-certificates/code-signing/).
- Sectigo : [Advanced Installer, où acheter un certificat](https://www.advancedinstaller.com/where-can-i-buy-code-signing-certificate.html) (prix à partir de 429 $).
- GlobalSign, [signature automatisée avec Azure Key Vault et AzureSignTool](https://support.globalsign.com/code-signing/automate-code-signing).
- Comparatif des coûts en CI par volume : [codenote.net, juillet 2026](https://codenote.net/en/posts/windows-code-signing-ci-cost-per-signing-volume-2026/).
