; Textes de l'installeur Lattis (page « Type d'installation », avertissements),
; dans les langues de l'installeur (project.nsi, MUI_LANGUAGE) : celle du
; système si elle y est, l'anglais sinon.
; Fichier en UTF-8 avec BOM, inclus par !include /CHARSET=UTF8.

; Version de l'installation remplacée, précédée d'une espace (vide si
; inconnue) : déclarée ici, avant les textes qui la citent (lattis_scope.nsh,
; lattis.findOther).
Var lattisOtherVer

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

; « Pour moi seulement » alors qu'une installation pour tous existe : elle est remplacée.
LangString LATTIS_REPLACE_MACHINE ${LANG_ENGLISH} "${INFO_PRODUCTNAME}$lattisOtherVer is already installed for all users of this computer (Program Files). It will be replaced by this installation for your account only; Windows will ask for administrator rights to remove it.$\r$\n$\r$\nContinue?"
LangString LATTIS_REPLACE_MACHINE ${LANG_FRENCH} "${INFO_PRODUCTNAME}$lattisOtherVer est déjà installé pour tous les utilisateurs de ce poste (Program Files). Il sera remplacé par cette installation pour votre compte seulement ; Windows demandera les droits d'administration pour le retirer.$\r$\n$\r$\nContinuer ?"
LangString LATTIS_REPLACE_MACHINE ${LANG_GERMAN} "${INFO_PRODUCTNAME}$lattisOtherVer ist bereits für alle Benutzer dieses Computers installiert (Program Files). Es wird durch diese Installation nur für Ihr Konto ersetzt; Windows fragt zum Entfernen nach Administratorrechten.$\r$\n$\r$\nFortfahren?"
LangString LATTIS_REPLACE_MACHINE ${LANG_SPANISH} "${INFO_PRODUCTNAME}$lattisOtherVer ya está instalado para todos los usuarios de este equipo (Program Files). Se sustituirá por esta instalación solo para su cuenta; Windows pedirá derechos de administrador para quitarlo.$\r$\n$\r$\n¿Continuar?"
LangString LATTIS_REPLACE_MACHINE ${LANG_SIMPCHINESE} "${INFO_PRODUCTNAME}$lattisOtherVer 已为此计算机的所有用户安装 (Program Files)。它将被此仅为您的帐户进行的安装替换；Windows 会请求管理员权限以将其移除。$\r$\n$\r$\n是否继续？"

; « Pour tous les utilisateurs » alors qu'une installation pour le compte existe : elle est remplacée.
LangString LATTIS_REPLACE_USER ${LANG_ENGLISH} "${INFO_PRODUCTNAME}$lattisOtherVer is already installed for your account only (AppData\Local\Programs). It will be replaced by this installation for all users of this computer.$\r$\n$\r$\nContinue?"
LangString LATTIS_REPLACE_USER ${LANG_FRENCH} "${INFO_PRODUCTNAME}$lattisOtherVer est déjà installé pour votre compte seulement (AppData\Local\Programs). Il sera remplacé par cette installation pour tous les utilisateurs de ce poste.$\r$\n$\r$\nContinuer ?"
LangString LATTIS_REPLACE_USER ${LANG_GERMAN} "${INFO_PRODUCTNAME}$lattisOtherVer ist bereits nur für Ihr Konto installiert (AppData\Local\Programs). Es wird durch diese Installation für alle Benutzer dieses Computers ersetzt.$\r$\n$\r$\nFortfahren?"
LangString LATTIS_REPLACE_USER ${LANG_SPANISH} "${INFO_PRODUCTNAME}$lattisOtherVer ya está instalado solo para su cuenta (AppData\Local\Programs). Se sustituirá por esta instalación para todos los usuarios de este equipo.$\r$\n$\r$\n¿Continuar?"
LangString LATTIS_REPLACE_USER ${LANG_SIMPCHINESE} "${INFO_PRODUCTNAME}$lattisOtherVer 已仅为您的帐户安装 (AppData\Local\Programs)。它将被此为此计算机所有用户进行的安装替换。$\r$\n$\r$\n是否继续？"

; Droits refusés pour retirer l'installation pour tous : rien n'est installé (ce serait une seconde installation).
LangString LATTIS_REMOVE_REFUSED ${LANG_ENGLISH} "${INFO_PRODUCTNAME}$lattisOtherVer remains installed for all users of this computer: removing it requires administrator rights, which were not granted. To avoid two installations, nothing was installed. Run the installer again and choose “Install for all users of this computer” to update it."
LangString LATTIS_REMOVE_REFUSED ${LANG_FRENCH} "${INFO_PRODUCTNAME}$lattisOtherVer reste installé pour tous les utilisateurs de ce poste : le retirer demande les droits d'administration, qui n'ont pas été accordés. Pour ne pas avoir deux installations, rien n'a été installé. Relancez l'installation et choisissez « Installer pour tous les utilisateurs de ce poste » pour le mettre à jour."
LangString LATTIS_REMOVE_REFUSED ${LANG_GERMAN} "${INFO_PRODUCTNAME}$lattisOtherVer bleibt für alle Benutzer dieses Computers installiert: Zum Entfernen sind Administratorrechte nötig, die nicht erteilt wurden. Um zwei Installationen zu vermeiden, wurde nichts installiert. Starten Sie die Installation erneut und wählen Sie „Für alle Benutzer dieses Computers installieren“, um es zu aktualisieren."
LangString LATTIS_REMOVE_REFUSED ${LANG_SPANISH} "${INFO_PRODUCTNAME}$lattisOtherVer sigue instalado para todos los usuarios de este equipo: quitarlo requiere derechos de administrador, que no se concedieron. Para no tener dos instalaciones, no se ha instalado nada. Vuelva a ejecutar el instalador y elija «Instalar para todos los usuarios de este equipo» para actualizarlo."
LangString LATTIS_REMOVE_REFUSED ${LANG_SIMPCHINESE} "${INFO_PRODUCTNAME}$lattisOtherVer 仍为此计算机的所有用户安装：移除它需要管理员权限，但未获得授权。为避免出现两个安装，未安装任何内容。请重新运行安装程序并选择“为此计算机的所有用户安装”以更新它。"

; L'autre installation n'a pas pu être retirée (détails de l'installation).
LangString LATTIS_REMOVE_KEPT ${LANG_ENGLISH} "The other installation of ${INFO_PRODUCTNAME} could not be removed; ${INFO_PRODUCTNAME} will offer to remove it when it starts."
LangString LATTIS_REMOVE_KEPT ${LANG_FRENCH} "L'autre installation de ${INFO_PRODUCTNAME} n'a pas pu être retirée ; ${INFO_PRODUCTNAME} proposera de la retirer à son lancement."
LangString LATTIS_REMOVE_KEPT ${LANG_GERMAN} "Die andere Installation von ${INFO_PRODUCTNAME} konnte nicht entfernt werden; ${INFO_PRODUCTNAME} bietet beim Start an, sie zu entfernen."
LangString LATTIS_REMOVE_KEPT ${LANG_SPANISH} "No se pudo quitar la otra instalación de ${INFO_PRODUCTNAME}; ${INFO_PRODUCTNAME} ofrecerá quitarla al iniciarse."
LangString LATTIS_REMOVE_KEPT ${LANG_SIMPCHINESE} "无法移除 ${INFO_PRODUCTNAME} 的另一个安装；${INFO_PRODUCTNAME} 启动时会提示将其移除。"

; Détails de l'installation, suivis d'un dossier.
LangString LATTIS_REMOVING ${LANG_ENGLISH} "Removing the other installation: "
LangString LATTIS_REMOVING ${LANG_FRENCH} "Retrait de l'autre installation : "
LangString LATTIS_REMOVING ${LANG_GERMAN} "Andere Installation wird entfernt: "
LangString LATTIS_REMOVING ${LANG_SPANISH} "Quitando la otra instalación: "
LangString LATTIS_REMOVING ${LANG_SIMPCHINESE} "正在移除另一个安装："

LangString LATTIS_LEFTOVER ${LANG_ENGLISH} "Removing what is left of an earlier installation: "
LangString LATTIS_LEFTOVER ${LANG_FRENCH} "Retrait d'un reste d'installation : "
LangString LATTIS_LEFTOVER ${LANG_GERMAN} "Reste einer früheren Installation werden entfernt: "
LangString LATTIS_LEFTOVER ${LANG_SPANISH} "Quitando restos de una instalación anterior: "
LangString LATTIS_LEFTOVER ${LANG_SIMPCHINESE} "正在移除早期安装的残留："
