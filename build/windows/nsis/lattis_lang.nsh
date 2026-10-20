; Textes de l'installeur Lattis (page « Type d'installation », avertissements),
; dans les langues de l'installeur (project.nsi, MUI_LANGUAGE) : celle du
; système si elle y est, l'anglais sinon.
; Fichier en UTF-8 avec BOM, inclus par !include /CHARSET=UTF8.

LangString LATTIS_SCOPE_TITLE ${LANG_ENGLISH} "Installation type"
LangString LATTIS_SCOPE_TITLE ${LANG_FRENCH} "Type d'installation"
LangString LATTIS_SCOPE_TITLE ${LANG_GERMAN} "Installationsart"
LangString LATTIS_SCOPE_TITLE ${LANG_SPANISH} "Tipo de instalación"
LangString LATTIS_SCOPE_TITLE ${LANG_SIMPCHINESE} "安装类型"

LangString LATTIS_SCOPE_SUBTITLE ${LANG_ENGLISH} "Choose who ${INFO_PRODUCTNAME} is installed for."
LangString LATTIS_SCOPE_SUBTITLE ${LANG_FRENCH} "Choisissez pour qui installer ${INFO_PRODUCTNAME}."
LangString LATTIS_SCOPE_SUBTITLE ${LANG_GERMAN} "Wählen Sie, für wen ${INFO_PRODUCTNAME} installiert wird."
LangString LATTIS_SCOPE_SUBTITLE ${LANG_SPANISH} "Elija para quién se instala ${INFO_PRODUCTNAME}."
LangString LATTIS_SCOPE_SUBTITLE ${LANG_SIMPCHINESE} "选择为谁安装 ${INFO_PRODUCTNAME}。"

LangString LATTIS_SCOPE_USER ${LANG_ENGLISH} "Install for me only"
LangString LATTIS_SCOPE_USER ${LANG_FRENCH} "Installer pour moi seulement"
LangString LATTIS_SCOPE_USER ${LANG_GERMAN} "Nur für mich installieren"
LangString LATTIS_SCOPE_USER ${LANG_SPANISH} "Instalar solo para mí"
LangString LATTIS_SCOPE_USER ${LANG_SIMPCHINESE} "仅为我安装"

LangString LATTIS_SCOPE_USER_DESC ${LANG_ENGLISH} "In your profile (AppData\Local\Programs). No administrator rights are needed, and updates install without asking."
LangString LATTIS_SCOPE_USER_DESC ${LANG_FRENCH} "Dans votre profil (AppData\Local\Programs). Aucun droit d'administration n'est demandé, et les mises à jour s'installent sans rien demander."
LangString LATTIS_SCOPE_USER_DESC ${LANG_GERMAN} "In Ihrem Benutzerprofil (AppData\Local\Programs). Es werden keine Administratorrechte benötigt, und Updates werden ohne Rückfrage installiert."
LangString LATTIS_SCOPE_USER_DESC ${LANG_SPANISH} "En su perfil (AppData\Local\Programs). No se necesitan derechos de administrador y las actualizaciones se instalan sin preguntar."
LangString LATTIS_SCOPE_USER_DESC ${LANG_SIMPCHINESE} "安装到您的用户配置文件 (AppData\Local\Programs)。无需管理员权限，更新会自动安装，无需确认。"

LangString LATTIS_SCOPE_MACHINE ${LANG_ENGLISH} "Install for all users of this computer"
LangString LATTIS_SCOPE_MACHINE ${LANG_FRENCH} "Installer pour tous les utilisateurs de ce poste"
LangString LATTIS_SCOPE_MACHINE ${LANG_GERMAN} "Für alle Benutzer dieses Computers installieren"
LangString LATTIS_SCOPE_MACHINE ${LANG_SPANISH} "Instalar para todos los usuarios de este equipo"
LangString LATTIS_SCOPE_MACHINE ${LANG_SIMPCHINESE} "为此计算机的所有用户安装"

LangString LATTIS_SCOPE_MACHINE_DESC ${LANG_ENGLISH} "In Program Files. Windows will ask for administrator rights, now and for each update."
LangString LATTIS_SCOPE_MACHINE_DESC ${LANG_FRENCH} "Dans Program Files. Windows demandera les droits d'administration, maintenant et à chaque mise à jour."
LangString LATTIS_SCOPE_MACHINE_DESC ${LANG_GERMAN} "Im Ordner „Programme“ (Program Files). Windows fragt jetzt und bei jedem Update nach Administratorrechten."
LangString LATTIS_SCOPE_MACHINE_DESC ${LANG_SPANISH} "En Program Files. Windows pedirá derechos de administrador, ahora y en cada actualización."
LangString LATTIS_SCOPE_MACHINE_DESC ${LANG_SIMPCHINESE} "安装到 Program Files。Windows 会请求管理员权限（现在以及每次更新时）。"

LangString LATTIS_SCOPE_INSTALLED ${LANG_ENGLISH} " (already installed)"
LangString LATTIS_SCOPE_INSTALLED ${LANG_FRENCH} " (déjà installé)"
LangString LATTIS_SCOPE_INSTALLED ${LANG_GERMAN} " (bereits installiert)"
LangString LATTIS_SCOPE_INSTALLED ${LANG_SPANISH} " (ya instalado)"
LangString LATTIS_SCOPE_INSTALLED ${LANG_SIMPCHINESE} "（已安装）"

LangString LATTIS_ELEVATION_REFUSED ${LANG_ENGLISH} "Installing for all users requires administrator rights, which were not granted."
LangString LATTIS_ELEVATION_REFUSED ${LANG_FRENCH} "L'installation pour tous les utilisateurs demande les droits d'administration, qui n'ont pas été accordés."
LangString LATTIS_ELEVATION_REFUSED ${LANG_GERMAN} "Die Installation für alle Benutzer erfordert Administratorrechte, die nicht erteilt wurden."
LangString LATTIS_ELEVATION_REFUSED ${LANG_SPANISH} "La instalación para todos los usuarios requiere derechos de administrador, que no se concedieron."
LangString LATTIS_ELEVATION_REFUSED ${LANG_SIMPCHINESE} "为所有用户安装需要管理员权限，但未获得授权。"

LangString LATTIS_UNINSTALL_REFUSED ${LANG_ENGLISH} "${INFO_PRODUCTNAME} is installed for all users: uninstalling it requires administrator rights, which were not granted."
LangString LATTIS_UNINSTALL_REFUSED ${LANG_FRENCH} "${INFO_PRODUCTNAME} est installé pour tous les utilisateurs : le désinstaller demande les droits d'administration, qui n'ont pas été accordés."
LangString LATTIS_UNINSTALL_REFUSED ${LANG_GERMAN} "${INFO_PRODUCTNAME} ist für alle Benutzer installiert: Die Deinstallation erfordert Administratorrechte, die nicht erteilt wurden."
LangString LATTIS_UNINSTALL_REFUSED ${LANG_SPANISH} "${INFO_PRODUCTNAME} está instalado para todos los usuarios: desinstalarlo requiere derechos de administrador, que no se concedieron."
LangString LATTIS_UNINSTALL_REFUSED ${LANG_SIMPCHINESE} "${INFO_PRODUCTNAME} 已为所有用户安装：卸载需要管理员权限，但未获得授权。"

; « Pour moi seulement » alors qu'une installation pour tous existe.
LangString LATTIS_OTHER_MACHINE ${LANG_ENGLISH} "${INFO_PRODUCTNAME} is already installed for all users of this computer (Program Files).$\r$\n$\r$\nInstalling it for your account as well gives you two copies: this one will open .lattis files and lattis:// links for you, and it updates itself without administrator rights. You can uninstall the other copy from Settings > Apps.$\r$\n$\r$\nContinue?"
LangString LATTIS_OTHER_MACHINE ${LANG_FRENCH} "${INFO_PRODUCTNAME} est déjà installé pour tous les utilisateurs de ce poste (Program Files).$\r$\n$\r$\nL'installer aussi pour votre compte en fait deux copies : celle-ci ouvrira pour vous les fichiers .lattis et les liens lattis://, et se mettra à jour sans droits d'administration. Vous pouvez désinstaller l'autre copie depuis Paramètres > Applications.$\r$\n$\r$\nContinuer ?"
LangString LATTIS_OTHER_MACHINE ${LANG_GERMAN} "${INFO_PRODUCTNAME} ist bereits für alle Benutzer dieses Computers installiert (Program Files).$\r$\n$\r$\nWenn Sie es zusätzlich für Ihr Konto installieren, gibt es zwei Kopien: Diese öffnet für Sie .lattis-Dateien und lattis://-Links und aktualisiert sich ohne Administratorrechte. Die andere Kopie können Sie unter Einstellungen > Apps deinstallieren.$\r$\n$\r$\nFortfahren?"
LangString LATTIS_OTHER_MACHINE ${LANG_SPANISH} "${INFO_PRODUCTNAME} ya está instalado para todos los usuarios de este equipo (Program Files).$\r$\n$\r$\nSi lo instala también para su cuenta tendrá dos copias: esta abrirá para usted los archivos .lattis y los enlaces lattis://, y se actualizará sin derechos de administrador. Puede desinstalar la otra copia desde Configuración > Aplicaciones.$\r$\n$\r$\n¿Continuar?"
LangString LATTIS_OTHER_MACHINE ${LANG_SIMPCHINESE} "${INFO_PRODUCTNAME} 已为此计算机的所有用户安装 (Program Files)。$\r$\n$\r$\n再为您的帐户安装一份将得到两个副本：此副本将为您打开 .lattis 文件和 lattis:// 链接，并且无需管理员权限即可自动更新。您可以在“设置 > 应用”中卸载另一个副本。$\r$\n$\r$\n是否继续？"

; « Pour tous les utilisateurs » alors qu'une installation pour le compte existe.
LangString LATTIS_OTHER_USER ${LANG_ENGLISH} "${INFO_PRODUCTNAME} is already installed for your account only (AppData\Local\Programs).$\r$\n$\r$\nInstalling it for all users as well gives you two copies, and your per-user copy keeps opening .lattis files and lattis:// links for you. You can uninstall it from Settings > Apps.$\r$\n$\r$\nContinue?"
LangString LATTIS_OTHER_USER ${LANG_FRENCH} "${INFO_PRODUCTNAME} est déjà installé pour votre compte seulement (AppData\Local\Programs).$\r$\n$\r$\nL'installer aussi pour tous les utilisateurs en fait deux copies, et votre copie personnelle continuera d'ouvrir pour vous les fichiers .lattis et les liens lattis://. Vous pouvez la désinstaller depuis Paramètres > Applications.$\r$\n$\r$\nContinuer ?"
LangString LATTIS_OTHER_USER ${LANG_GERMAN} "${INFO_PRODUCTNAME} ist bereits nur für Ihr Konto installiert (AppData\Local\Programs).$\r$\n$\r$\nWenn Sie es zusätzlich für alle Benutzer installieren, gibt es zwei Kopien, und Ihre persönliche Kopie öffnet für Sie weiterhin .lattis-Dateien und lattis://-Links. Sie können sie unter Einstellungen > Apps deinstallieren.$\r$\n$\r$\nFortfahren?"
LangString LATTIS_OTHER_USER ${LANG_SPANISH} "${INFO_PRODUCTNAME} ya está instalado solo para su cuenta (AppData\Local\Programs).$\r$\n$\r$\nSi lo instala también para todos los usuarios tendrá dos copias, y su copia personal seguirá abriendo para usted los archivos .lattis y los enlaces lattis://. Puede desinstalarla desde Configuración > Aplicaciones.$\r$\n$\r$\n¿Continuar?"
LangString LATTIS_OTHER_USER ${LANG_SIMPCHINESE} "${INFO_PRODUCTNAME} 已仅为您的帐户安装 (AppData\Local\Programs)。$\r$\n$\r$\n再为所有用户安装一份将得到两个副本，并且您的个人副本将继续为您打开 .lattis 文件和 lattis:// 链接。您可以在“设置 > 应用”中将其卸载。$\r$\n$\r$\n是否继续？"
