Unicode true

####
## Please note: Template replacements don't work in this file. They are provided with default defines like
## mentioned underneath.
## If the keyword is not defined, "wails_tools.nsh" will populate them.
## If they are defined here, "wails_tools.nsh" will not touch them. This allows you to use this project.nsi manually
## from outside of Wails for debugging and development of the installer.
## 
## For development first make a wails nsis build to populate the "wails_tools.nsh":
## > wails build --target windows/amd64 --nsis
## Then you can call makensis on this file with specifying the path to your binary:
## For a AMD64 only installer:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app.exe
## For a ARM64 only installer:
## > makensis -DARG_WAILS_ARM64_BINARY=..\..\bin\app.exe
## For a installer with both architectures:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app-amd64.exe -DARG_WAILS_ARM64_BINARY=..\..\bin\app-arm64.exe
####
## The following information is taken from the wails_tools.nsh file, but they can be overwritten here.
####
## !define INFO_PROJECTNAME    "my-project" # Default "lattis"
## !define INFO_COMPANYNAME    "My Company" # Default "Lattis"
## !define INFO_PRODUCTNAME    "My Product Name" # Default "Lattis"
## !define INFO_PRODUCTVERSION "1.0.0"     # Default "0.1.0"
## !define INFO_COPYRIGHT      "(c) Now, My Company" # Default "© 2026, My Company"
###
## !define PRODUCT_EXECUTABLE  "Application.exe"      # Default "${INFO_PROJECTNAME}.exe"
## !define UNINST_KEY_NAME     "UninstKeyInRegistry"  # Default "${INFO_COMPANYNAME}${INFO_PRODUCTNAME}"
####
## Lattis : RequestExecutionLevel user impose (wails_tools.nsh), portee choisie a l'execution (lattis_scope.nsh).
####
## Include the wails tools
####
!include "wails_tools.nsh"

# The version information for this two must consist of 4 parts
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

# Enable HiDPI support. https://nsis.sourceforge.io/Reference/ManifestDPIAware
ManifestDPIAware true

!include "MUI2.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
# !define MUI_WELCOMEFINISHPAGE_BITMAP "resources\leftimage.bmp" #Include this to add a bitmap on the left side of the Welcome Page. Must be a size of 164x314
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.

!define MUI_PAGE_CUSTOMFUNCTION_PRE lattis.welcomePre
!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
## Lattis : page "Type d'installation", pour moi seulement / pour tous les utilisateurs (lattis_scope.nsh).
Page custom lattis.scopePage lattis.scopeLeave
!define MUI_PAGE_CUSTOMFUNCTION_PRE lattis.directoryPre
!define MUI_PAGE_CUSTOMFUNCTION_SHOW lattis.directoryShow
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_INSTFILES # Uninstalling page

## Lattis : la langue du systeme si elle est ici, l'anglais (la premiere) sinon.
!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "French"
!insertmacro MUI_LANGUAGE "German"
!insertmacro MUI_LANGUAGE "Spanish"
!insertmacro MUI_LANGUAGE "SimpChinese"
!include /CHARSET=UTF8 "lattis_lang.nsh"

## The following two statements can be used to sign the installer and the uninstaller. The path to the binaries are provided in %1
#!uninstfinalize 'signtool --file "%1"'
#!finalize 'signtool --file "%1"'
## Lattis : makensis -DSIGN_SCRIPT=chemin\build\windows\sign.ps1 signe l'installeur et le
## desinstalleur (certificat lu dans WINDOWS_CERT_PATH / WINDOWS_CERT_PASSWORD ou
## WINDOWS_CERT_THUMBPRINT, voir docs/release/signature.md).
!ifdef SIGN_SCRIPT
    !uninstfinalize 'pwsh -NoProfile -ExecutionPolicy Bypass -File "${SIGN_SCRIPT}" -Require -Files "%1"' = 0
    !finalize 'pwsh -NoProfile -ExecutionPolicy Bypass -File "${SIGN_SCRIPT}" -Require -Files "%1"' = 0
!endif

Name "${INFO_PRODUCTNAME}"
## Lattis : un seul installeur, publie sous Lattis-windows-x64-setup.exe
## (docs/release/publier.md). Pas d'InstallDir : le dossier depend de la portee
## (lattis.applyScope), $INSTDIR n'est rempli avant .onInit que par /D=.
OutFile "..\..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe"
ShowInstDetails show # This will always show the installation details.

## Ligne de commande (deploiement, mise a jour lancee par Lattis a sa fermeture) :
##   setup.exe [/S] [/CurrentUser | /AllUsers] [/UPDATE] [/RELAUNCH] [/D=<dossier>]
## /CurrentUser, /AllUsers et la portee par defaut : lattis_scope.nsh ;
## /UPDATE : ne recree pas le raccourci du Bureau (l'utilisateur a pu le retirer) ;
## /RELAUNCH : rouvre Lattis apres l'installation (sans droits d'administration) ;
## /D= : dossier d'installation, toujours en dernier, sans guillemets.
!include "lattis_scope.nsh"

; Lattis lance la mise a jour en se fermant : attendre que lattis.exe soit
; libre (30 s au plus) avant de le remplacer.
Function lattis.waitForExit
    StrCpy $R3 0
    ${Do}
        ${IfNot} ${FileExists} "$INSTDIR\${PRODUCT_EXECUTABLE}"
            ${Break}
        ${EndIf}
        ClearErrors
        FileOpen $R4 "$INSTDIR\${PRODUCT_EXECUTABLE}" a
        ${IfNot} ${Errors}
            FileClose $R4
            ${Break}
        ${EndIf}
        IntOp $R3 $R3 + 1
        ${If} $R3 >= 60
            ${Break}
        ${EndIf}
        Sleep 500
    ${Loop}
FunctionEnd

; Trace de l'installation lancee par Lattis (/UPDATE), lue au lancement
; suivant pour dire pourquoi la mise a jour n'a pas abouti (internal/update,
; installresult.go) : %LOCALAPPDATA%\Lattis\updates\install-result.txt de
; l'utilisateur, en UTF-16LE (le dossier peut avoir des accents).
; Pile : statut (ok, failed, locked, elevation) au sommet, puis le code.
Var lattisFailStatus ; statut a ecrire par .onInstFailed (failed sinon)
Function lattis.writeResult
    Exch $0 ; statut
    Exch
    Exch $1 ; code
    Push $2
    ${If} $lattisUpdate == 1
        SetShellVarContext current
        CreateDirectory "$LOCALAPPDATA\Lattis\updates"
        ClearErrors
        FileOpen $2 "$LOCALAPPDATA\Lattis\updates\install-result.txt" w
        ${IfNot} ${Errors}
            FileWriteUTF16LE $2 "version=${INFO_PRODUCTVERSION}$\r$\n"
            FileWriteUTF16LE $2 "status=$0$\r$\n"
            FileWriteUTF16LE $2 "code=$1$\r$\n"
            FileWriteUTF16LE $2 "dir=$INSTDIR$\r$\n"
            FileClose $2
        ${EndIf}
        !insertmacro wails.setShellContext
    ${EndIf}
    Pop $2
    Pop $1
    Pop $0
FunctionEnd

Function .onInstSuccess
    Push 0
    Push ok
    Call lattis.writeResult
FunctionEnd

Function .onInstFailed
    ${If} $lattisFailStatus == ""
        StrCpy $lattisFailStatus failed
    ${EndIf}
    Push 2
    Push $lattisFailStatus
    Call lattis.writeResult
FunctionEnd

Section
    !insertmacro wails.setShellContext

    !insertmacro wails.webview2runtime

    Call lattis.waitForExit

    SetOutPath $INSTDIR

    ; lattis.exe encore ouvert apres l'attente (ou fichier ignore en mode
    ; silencieux) : echec dit, plutot qu'une installation qui garde l'ancien.
    ClearErrors
    !insertmacro wails.files
    ${If} ${Errors}
        StrCpy $lattisFailStatus locked
        Abort
    ${EndIf}

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    ${If} $lattisUpdate != 1
        CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    ${EndIf}

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    !insertmacro wails.writeUninstaller

    ${If} $lattisRelaunch == 1
        ${If} $lattisIsAdmin == 1
            ; Installeur eleve : relance par l'Explorateur, donc sans les droits
            ; d'administration.
            Exec '"$WINDIR\explorer.exe" "$INSTDIR\${PRODUCT_EXECUTABLE}"'
        ${Else}
            Exec '"$INSTDIR\${PRODUCT_EXECUTABLE}"'
        ${EndIf}
    ${EndIf}
SectionEnd

Section "uninstall" 
    !insertmacro wails.setShellContext

    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}" # Remove the WebView2 DataPath

    RMDir /r $INSTDIR

    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro wails.deleteUninstaller
SectionEnd
