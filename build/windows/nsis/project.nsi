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
## !define REQUEST_EXECUTION_LEVEL "admin"            # Default "admin"  see also https://nsis.sourceforge.io/Docs/Chapter4.html
## !define WAILS_INSTALL_SCOPE     "user"             # Default "machine" - set to "user" for per-user install ($LOCALAPPDATA) without UAC prompt
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

!include "MUI.nsh"

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
# !define MUI_WELCOMEFINISHPAGE_BITMAP "resources\leftimage.bmp" #Include this to add a bitmap on the left side of the Welcome Page. Must be a size of 164x314
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.

!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_INSTFILES # Uninstalling page

!insertmacro MUI_LANGUAGE "English" # Set the Language of the installer

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
## Lattis : deux installeurs par version (docs/release/publier.md).
##   par utilisateur (-DWAILS_INSTALL_SCOPE=user) : %LOCALAPPDATA%\Programs\Lattis, HKCU,
##     sans droits d'administration ; publie sous Lattis-windows-x64-setup.exe ;
##   par machine (defaut) : Program Files, HKLM, droits d'administration ;
##     publie sous Lattis-windows-x64-setup-machine.exe.
!if "${WAILS_INSTALL_SCOPE}" == "user"
    OutFile "..\..\..\bin\${INFO_PROJECTNAME}-${ARCH}-user-installer.exe"
    InstallDir "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}"
!else
    OutFile "..\..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe"
    InstallDir "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
!endif
ShowInstDetails show # This will always show the installation details.

## Mise a jour lancee par Lattis a sa fermeture (internal/update) :
##   setup.exe /S /UPDATE [/RELAUNCH] /D=<dossier d'installation>
## /UPDATE : ne recree pas le raccourci du Bureau (l'utilisateur a pu le retirer) ;
## /RELAUNCH : rouvre Lattis apres l'installation (sans droits d'administration).
Var lattisUpdate
Var lattisRelaunch

!if "${WAILS_INSTALL_SCOPE}" == "user"
    !define LATTIS_OTHER_SCOPE_ROOT HKLM
    !define LATTIS_OTHER_SCOPE_TEXT "Lattis is already installed for all users of this computer (Program Files).$\r$\n$\r$\nInstalling it for your account as well gives you two copies: this one will open .lattis files and lattis:// links for you, and it updates itself without administrator rights. You can uninstall the other copy from Settings > Apps.$\r$\n$\r$\nContinue?"
!else
    !define LATTIS_OTHER_SCOPE_ROOT HKCU
    !define LATTIS_OTHER_SCOPE_TEXT "Lattis is already installed for your account only (AppData\Local\Programs).$\r$\n$\r$\nInstalling it for all users as well gives you two copies, and your per-user copy keeps opening .lattis files and lattis:// links for you. You can uninstall it from Settings > Apps.$\r$\n$\r$\nContinue?"
!endif

Function .onInit
   !insertmacro wails.checkArchitecture

   ${GetParameters} $R0
   ClearErrors
   ${GetOptions} $R0 "/UPDATE" $R1
   ${IfNot} ${Errors}
       StrCpy $lattisUpdate 1
   ${EndIf}
   ClearErrors
   ${GetOptions} $R0 "/RELAUNCH" $R1
   ${IfNot} ${Errors}
       StrCpy $lattisRelaunch 1
   ${EndIf}

   ; Les deux installeurs coexistent mal : prevenir si l'autre est deja la.
   ${IfNot} ${Silent}
       SetRegView 64
       ReadRegStr $R2 ${LATTIS_OTHER_SCOPE_ROOT} "${UNINST_KEY}" "UninstallString"
       ${If} $R2 != ""
           MessageBox MB_YESNO|MB_ICONEXCLAMATION "${LATTIS_OTHER_SCOPE_TEXT}" IDYES lattis_other_ok
           Quit
           lattis_other_ok:
       ${EndIf}
   ${EndIf}
FunctionEnd

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

Section
    !insertmacro wails.setShellContext

    !insertmacro wails.webview2runtime

    Call lattis.waitForExit

    SetOutPath $INSTDIR

    !insertmacro wails.files

    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    ${If} $lattisUpdate != 1
        CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}"
    ${EndIf}

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    !insertmacro wails.writeUninstaller

    ${If} $lattisRelaunch == 1
        !if "${WAILS_INSTALL_SCOPE}" == "user"
            Exec '"$INSTDIR\${PRODUCT_EXECUTABLE}"'
        !else
            ; Installeur eleve : relance par l'Explorateur, donc sans les droits
            ; d'administration.
            Exec '"$WINDIR\explorer.exe" "$INSTDIR\${PRODUCT_EXECUTABLE}"'
        !endif
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
