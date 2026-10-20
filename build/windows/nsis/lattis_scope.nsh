; Lattis : un seul installeur, pour l'utilisateur ou pour tous les utilisateurs
; du poste (comme celui de Python).
;
; L'installeur demarre SANS droits d'administration (RequestExecutionLevel
; user, manifeste asInvoker). Les droits ne sont demandes que si
; l'installation pour tous les utilisateurs est choisie : l'installeur se
; relance alors avec le verbe « runas » (invite UAC), attend la fin de cette
; seconde instance et rend son code de sortie.
;
;   pour l'utilisateur (scope "user")   : %LOCALAPPDATA%\Programs\Lattis, HKCU
;   pour tous (scope "machine")         : Program Files\Lattis\Lattis, HKLM
;
; Ligne de commande :
;   /CurrentUser  pour l'utilisateur, sans jamais demander de droits ;
;   /AllUsers     pour tous les utilisateurs (droits demandes si besoin) ;
;   sans l'un ni l'autre : avec /D=, la portee de l'installation de ce
;   dossier (pour tous sous Program Files) ; sans /D=, celle de
;   l'installation existante (les deux : pour tous si l'installeur a deja
;   les droits, pour l'utilisateur sinon) ; sinon, l'interface propose
;   « pour moi seulement » et une installation silencieuse (/S) se fait pour
;   tous si elle tourne avec les droits d'administration (deploiement), pour
;   l'utilisateur sinon.
;   /S, /D=, /UPDATE et /RELAUNCH : voir project.nsi.
; Parametres internes de la relance : /ELEVATED (anti-boucle ; =page : la
; page de choix a deja ete vue, la page d'accueil aussi) et /LANGID=<langue>.
;
; Les installations des installeurs de la 0.3.0-beta (par utilisateur et par
; machine) ont la meme cle de desinstallation (UNINST_KEY) et les memes
; dossiers : elles sont reconnues et mises a jour sur place.

!include "LogicLib.nsh"
!include "nsDialogs.nsh"
!include "WinMessages.nsh"
!include "FileFunc.nsh"

!ifndef SEE_MASK_NOCLOSEPROCESS
    !define SEE_MASK_NOCLOSEPROCESS 0x40
!endif
!ifndef BCM_SETSHIELD
    !define BCM_SETSHIELD 0x160C
!endif

Var lattisScope         ; "user" ou "machine"
Var lattisScopeForced   ; 1 : /AllUsers ou /CurrentUser
Var lattisIsAdmin       ; 1 : le processus a les droits d'administration
Var lattisElevated      ; 1 : instance relancee par runas
Var lattisFromPage      ; 1 : relancee depuis la page de choix
Var lattisElevateTag    ; /ELEVATED ou /ELEVATED=page
Var lattisCmdDir        ; dossier donne par /D= (vide sinon)
Var lattisUserDir       ; installation existante pour l'utilisateur (HKCU)
Var lattisMachineDir    ; installation existante pour tous (HKLM)
Var lattisUpdate        ; /UPDATE
Var lattisRelaunch      ; /RELAUNCH
Var lattisChildParams
Var lattisChildProc
Var lattisChildErr
Var lattisRadioUser
Var lattisRadioMachine

; Fonctions communes a l'installeur et au desinstalleur (prefixe un.).
!macro LATTIS_COMMON_FUNCTIONS UN

; Dossier d'une valeur UninstallString (« "C:\...\uninstall.exe" »), pour les
; installations plus anciennes, sans InstallLocation.
Function ${UN}lattis.dirOfUninstaller
    Exch $0
    Push $1
    Push $2
    StrCpy $1 $0 1
    ${If} $1 == '"'
        StrCpy $0 $0 "" 1
        StrCpy $2 0
        ${Do}
            StrCpy $1 $0 1 $2
            ${If} $1 == '"'
            ${OrIf} $1 == ""
                ${ExitDo}
            ${EndIf}
            IntOp $2 $2 + 1
        ${Loop}
        StrCpy $0 $0 $2
    ${EndIf}
    ${GetParent} $0 $0
    Pop $2
    Pop $1
    Exch $0
FunctionEnd

; Relance de cet executable avec les droits d'administration (verbe runas) et
; les parametres $lattisChildParams. $lattisChildProc : le processus lance
; (0 : echec, code d'erreur Windows dans $lattisChildErr, 1223 : refus).
Function ${UN}lattis.runElevated
    Push $0
    Push $1
    StrCpy $lattisChildProc 0
    System::Call '*(&l4, i ${SEE_MASK_NOCLOSEPROCESS}, p $HWNDPARENT, w "runas", w "$EXEPATH", w "$lattisChildParams", w "$EXEDIR", i ${SW_SHOWNORMAL}, p 0, p 0, p 0, p 0, i 0, p 0, p 0) p .r1'
    System::Call 'shell32::ShellExecuteExW(p r1) i .r0 ?e'
    Pop $lattisChildErr
    ${If} $0 <> 0
        System::Call '*$1(i, i, p, p, p, p, p, i, p, p, p, p, i, p, p .r0)'
        StrCpy $lattisChildProc $0
        StrCpy $lattisChildErr 0
    ${EndIf}
    System::Free $1
    Pop $1
    Pop $0
FunctionEnd

; Attend la fin de $lattisChildProc ; son code de sortie dans $lattisChildErr.
Function ${UN}lattis.waitChild
    Push $0
    StrCpy $0 1
    System::Call 'kernel32::WaitForSingleObject(p $lattisChildProc, i -1) i'
    System::Call 'kernel32::GetExitCodeProcess(p $lattisChildProc, *i .r0) i'
    System::Call 'kernel32::CloseHandle(p $lattisChildProc) i'
    StrCpy $lattisChildErr $0
    Pop $0
FunctionEnd

Function ${UN}lattis.detectAdmin
    Push $0
    StrCpy $lattisIsAdmin 0
    System::Call 'shell32::IsUserAnAdmin() i .r0'
    ${If} $0 <> 0
        StrCpy $lattisIsAdmin 1
    ${EndIf}
    Pop $0
FunctionEnd

; Installations existantes : InstallLocation de la cle de desinstallation,
; sinon le dossier de UninstallString.
Function ${UN}lattis.readInstalls
    SetRegView 64
    ReadRegStr $lattisUserDir HKCU "${UNINST_KEY}" "InstallLocation"
    ${If} $lattisUserDir == ""
        ReadRegStr $lattisUserDir HKCU "${UNINST_KEY}" "UninstallString"
        ${If} $lattisUserDir != ""
            Push $lattisUserDir
            Call ${UN}lattis.dirOfUninstaller
            Pop $lattisUserDir
        ${EndIf}
    ${EndIf}
    ReadRegStr $lattisMachineDir HKLM "${UNINST_KEY}" "InstallLocation"
    ${If} $lattisMachineDir == ""
        ReadRegStr $lattisMachineDir HKLM "${UNINST_KEY}" "UninstallString"
        ${If} $lattisMachineDir != ""
            Push $lattisMachineDir
            Call ${UN}lattis.dirOfUninstaller
            Pop $lattisMachineDir
        ${EndIf}
    ${EndIf}
FunctionEnd

; Le dossier sur la pile est-il sous Program Files ? Rend 1 ou 0.
Function ${UN}lattis.underProgramFiles
    Exch $0
    Push $1
    Push $2
    StrLen $1 "$PROGRAMFILES64\"
    StrCpy $2 $0 $1
    ${If} $2 == "$PROGRAMFILES64\"
        StrCpy $0 1
    ${Else}
        StrLen $1 "$PROGRAMFILES32\"
        StrCpy $2 $0 $1
        ${If} $2 == "$PROGRAMFILES32\"
            StrCpy $0 1
        ${Else}
            StrCpy $0 0
        ${EndIf}
    ${EndIf}
    Pop $2
    Pop $1
    Exch $0
FunctionEnd

!macroend

!insertmacro LATTIS_COMMON_FUNCTIONS ""
!insertmacro LATTIS_COMMON_FUNCTIONS "un."

; Portee par defaut, sans /AllUsers ni /CurrentUser (voir en tete).
Function lattis.guessScope
    Push $0
    StrCpy $lattisScope user
    ${If} $lattisCmdDir != ""
        ${If} $lattisMachineDir != ""
        ${AndIf} $lattisCmdDir == $lattisMachineDir
            StrCpy $lattisScope machine
            Goto lattis_guess_done
        ${EndIf}
        ${If} $lattisUserDir != ""
        ${AndIf} $lattisCmdDir == $lattisUserDir
            Goto lattis_guess_done
        ${EndIf}
        Push $lattisCmdDir
        Call lattis.underProgramFiles
        Pop $0
        ${If} $0 == 1
            StrCpy $lattisScope machine
        ${EndIf}
        ; Autre dossier : nouvelle installation, comme sans installation existante.
        ${If} ${Silent}
        ${AndIf} $lattisIsAdmin == 1
            StrCpy $lattisScope machine
        ${EndIf}
        Goto lattis_guess_done
    ${EndIf}
    ${If} $lattisMachineDir != ""
    ${AndIf} $lattisUserDir != ""
        ${If} $lattisIsAdmin == 1
            StrCpy $lattisScope machine
        ${EndIf}
    ${ElseIf} $lattisMachineDir != ""
        StrCpy $lattisScope machine
    ${ElseIf} $lattisUserDir != ""
        StrCpy $lattisScope user
    ${ElseIf} ${Silent}
    ${AndIf} $lattisIsAdmin == 1
        StrCpy $lattisScope machine
    ${EndIf}
    lattis_guess_done:
    Pop $0
FunctionEnd

; Contexte (dossiers et Registre « SHCTX ») et dossier d'installation de la
; portee choisie : /D=, sinon l'installation existante, sinon le dossier
; par defaut.
Function lattis.applyScope
    ${If} $lattisScope == machine
        SetShellVarContext all
    ${Else}
        SetShellVarContext current
    ${EndIf}
    ${If} $lattisCmdDir != ""
        StrCpy $INSTDIR $lattisCmdDir
    ${ElseIf} $lattisScope == machine
        ${If} $lattisMachineDir != ""
            StrCpy $INSTDIR $lattisMachineDir
        ${Else}
            StrCpy $INSTDIR "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
        ${EndIf}
    ${Else}
        ${If} $lattisUserDir != ""
            StrCpy $INSTDIR $lattisUserDir
        ${Else}
            StrCpy $INSTDIR "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}"
        ${EndIf}
    ${EndIf}
FunctionEnd

; Relance pour tous les utilisateurs, avec les droits d'administration : les
; memes parametres plus /AllUsers, /ELEVATED, la langue et /D= (en dernier).
; Attend la fin de l'instance relancee et quitte avec son code de sortie ;
; ne revient qu'en cas d'echec (refus de l'UAC : $lattisChildErr = 1223).
Function lattis.elevate
    ${GetParameters} $lattisChildParams
    StrCpy $lattisChildParams "$lattisChildParams /AllUsers $lattisElevateTag /LANGID=$LANGUAGE"
    ${If} $lattisCmdDir != ""
        StrCpy $lattisChildParams "$lattisChildParams /D=$lattisCmdDir"
    ${EndIf}
    Call lattis.runElevated
    ${If} $lattisChildProc == 0
        Return
    ${EndIf}
    HideWindow
    Call lattis.waitChild
    SetErrorLevel $lattisChildErr
    Quit
FunctionEnd

Function .onInit
    !insertmacro wails.checkArchitecture

    ; Sans InstallDir, $INSTDIR n'est rempli ici que par /D=.
    StrCpy $lattisCmdDir $INSTDIR

    ${GetParameters} $R0
    ClearErrors
    ${GetOptions} $R0 "/LANGID=" $R1
    ${IfNot} ${Errors}
    ${AndIf} $R1 != ""
        StrCpy $LANGUAGE $R1
    ${EndIf}
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
    ClearErrors
    ${GetOptions} $R0 "/ELEVATED" $R1
    ${IfNot} ${Errors}
        StrCpy $lattisElevated 1
        ${If} $R1 == "=page"
            StrCpy $lattisFromPage 1
        ${EndIf}
    ${EndIf}

    Call lattis.detectAdmin
    Call lattis.readInstalls

    StrCpy $lattisScopeForced 0
    ClearErrors
    ${GetOptions} $R0 "/AllUsers" $R1
    ${IfNot} ${Errors}
        StrCpy $lattisScope machine
        StrCpy $lattisScopeForced 1
    ${Else}
        ClearErrors
        ${GetOptions} $R0 "/CurrentUser" $R1
        ${IfNot} ${Errors}
            StrCpy $lattisScope user
            StrCpy $lattisScopeForced 1
        ${Else}
            Call lattis.guessScope
        ${EndIf}
    ${EndIf}

    ; Sans interface, ou portee imposee : droits demandes des maintenant si
    ; l'installation est pour tous. Sinon, la page de choix s'en charge.
    StrCpy $R2 0
    ${If} ${Silent}
        StrCpy $R2 1
    ${EndIf}
    ${If} $lattisScopeForced == 1
        StrCpy $R2 1
    ${EndIf}
    ${If} $R2 == 1
    ${AndIf} $lattisScope == machine
    ${AndIf} $lattisIsAdmin != 1
        ${If} $lattisElevated != 1
            StrCpy $lattisElevateTag "/ELEVATED"
            Call lattis.elevate
        ${Else}
            ; Deja relance et toujours sans droits (UAC desactive ?).
            StrCpy $lattisChildErr 740 ; ERROR_ELEVATION_REQUIRED
        ${EndIf}
        ${IfNot} ${Silent}
            MessageBox MB_OK|MB_ICONSTOP "$(LATTIS_ELEVATION_REFUSED)"
        ${EndIf}
        SetErrorLevel $lattisChildErr
        Quit
    ${EndIf}

    Call lattis.applyScope
FunctionEnd

; Bouton Suivant : bouclier UAC quand il menera a l'invite.
Function lattis.updateShield
    Push $0
    Push $1
    GetDlgItem $0 $HWNDPARENT 1
    StrCpy $1 0
    ${If} $lattisRadioMachine != ""
    ${AndIf} $lattisIsAdmin != 1
        ${NSD_GetState} $lattisRadioMachine $1
        ${If} $1 == ${BST_CHECKED}
            StrCpy $1 1
        ${Else}
            StrCpy $1 0
        ${EndIf}
    ${EndIf}
    SendMessage $0 ${BCM_SETSHIELD} 0 $1
    Pop $1
    Pop $0
FunctionEnd

Function lattis.clearShield
    Push $0
    GetDlgItem $0 $HWNDPARENT 1
    SendMessage $0 ${BCM_SETSHIELD} 0 0
    Pop $0
FunctionEnd

; Accueil : deja vu par l'instance qui s'est relancee depuis la page de choix.
Function lattis.welcomePre
    Call lattis.clearShield
    ${If} $lattisFromPage == 1
        Abort
    ${EndIf}
FunctionEnd

Function lattis.directoryPre
    Call lattis.clearShield
FunctionEnd

; Instance relancee depuis la page de choix : rien avant le dossier, donc
; pas de bouton Precedent.
Function lattis.directoryShow
    ${If} $lattisFromPage == 1
        GetDlgItem $0 $HWNDPARENT 3
        EnableWindow $0 0
    ${EndIf}
FunctionEnd

Function lattis.scopeClick
    Pop $0 ; controle clique
    Call lattis.updateShield
FunctionEnd

; Page « Type d'installation » : pour moi seulement / pour tous les
; utilisateurs du poste, la portee de l'installation existante cochee.
Function lattis.scopePage
    ${If} $lattisScopeForced == 1
        Abort
    ${EndIf}
    !insertmacro MUI_HEADER_TEXT "$(LATTIS_SCOPE_TITLE)" "$(LATTIS_SCOPE_SUBTITLE)"
    nsDialogs::Create 1018
    Pop $0
    ${If} $0 == error
        Abort
    ${EndIf}

    StrCpy $1 "$(LATTIS_SCOPE_USER)"
    ${If} $lattisUserDir != ""
        StrCpy $1 "$1$(LATTIS_SCOPE_INSTALLED)"
    ${EndIf}
    ${NSD_CreateRadioButton} 0 0 100% 12u "$1"
    Pop $lattisRadioUser
    ${NSD_AddStyle} $lattisRadioUser ${WS_GROUP}
    ${NSD_OnClick} $lattisRadioUser lattis.scopeClick
    ${NSD_CreateLabel} 12u 14u -12u 28u "$(LATTIS_SCOPE_USER_DESC)"
    Pop $0

    StrCpy $1 "$(LATTIS_SCOPE_MACHINE)"
    ${If} $lattisMachineDir != ""
        StrCpy $1 "$1$(LATTIS_SCOPE_INSTALLED)"
    ${EndIf}
    ${NSD_CreateRadioButton} 0 50u 100% 12u "$1"
    Pop $lattisRadioMachine
    ${NSD_OnClick} $lattisRadioMachine lattis.scopeClick
    ${NSD_CreateLabel} 12u 64u -12u 28u "$(LATTIS_SCOPE_MACHINE_DESC)"
    Pop $0

    ${If} $lattisScope == machine
        ${NSD_Check} $lattisRadioMachine
    ${Else}
        ${NSD_Check} $lattisRadioUser
    ${EndIf}
    Call lattis.updateShield
    nsDialogs::Show
FunctionEnd

Function lattis.scopeLeave
    ${NSD_GetState} $lattisRadioMachine $0
    ${If} $0 == ${BST_CHECKED}
        StrCpy $lattisScope machine
    ${Else}
        StrCpy $lattisScope user
    ${EndIf}

    ; Deux copies cote a cote : prevenir.
    ${If} $lattisScope == user
    ${AndIf} $lattisMachineDir != ""
    ${AndIf} $lattisUserDir == ""
        MessageBox MB_YESNO|MB_ICONEXCLAMATION "$(LATTIS_OTHER_MACHINE)" IDYES lattis_keep_user
        Abort
        lattis_keep_user:
    ${EndIf}
    ${If} $lattisScope == machine
    ${AndIf} $lattisUserDir != ""
    ${AndIf} $lattisMachineDir == ""
        MessageBox MB_YESNO|MB_ICONEXCLAMATION "$(LATTIS_OTHER_USER)" IDYES lattis_keep_machine
        Abort
        lattis_keep_machine:
    ${EndIf}

    ${If} $lattisScope == machine
    ${AndIf} $lattisIsAdmin != 1
        StrCpy $lattisElevateTag "/ELEVATED=page"
        Call lattis.elevate
        ; Refus de l'UAC : on reste sur la page.
        MessageBox MB_OK|MB_ICONEXCLAMATION "$(LATTIS_ELEVATION_REFUSED)"
        Abort
    ${EndIf}

    Call lattis.clearShield
    Call lattis.applyScope
FunctionEnd

; Desinstalleur : la portee est celle de la cle de desinstallation qui
; designe son dossier ; pour tous les utilisateurs, il se relance avec les
; droits d'administration s'il ne les a pas.
Function un.onInit
    ${GetParameters} $R0
    ClearErrors
    ${GetOptions} $R0 "/LANGID=" $R1
    ${IfNot} ${Errors}
    ${AndIf} $R1 != ""
        StrCpy $LANGUAGE $R1
    ${EndIf}
    ClearErrors
    ${GetOptions} $R0 "/ELEVATED" $R1
    ${IfNot} ${Errors}
        StrCpy $lattisElevated 1
    ${EndIf}

    Call un.lattis.detectAdmin
    Call un.lattis.readInstalls

    StrCpy $lattisScope user
    ${If} $lattisMachineDir != ""
    ${AndIf} $INSTDIR == $lattisMachineDir
        StrCpy $lattisScope machine
    ${ElseIf} $lattisUserDir != ""
    ${AndIf} $INSTDIR == $lattisUserDir
        StrCpy $lattisScope user
    ${Else}
        Push $INSTDIR
        Call un.lattis.underProgramFiles
        Pop $R1
        ${If} $R1 == 1
            StrCpy $lattisScope machine
        ${EndIf}
    ${EndIf}

    ${If} $lattisScope == machine
    ${AndIf} $lattisIsAdmin != 1
        ${If} $lattisElevated != 1
            ; _?= en dernier : le desinstalleur relance ne se recopie pas.
            StrCpy $lattisChildParams "/ELEVATED /LANGID=$LANGUAGE _?=$INSTDIR"
            ${If} ${Silent}
                StrCpy $lattisChildParams "/S $lattisChildParams"
            ${EndIf}
            Call un.lattis.runElevated
            ${If} $lattisChildProc != 0
                Call un.lattis.waitChild
                SetErrorLevel $lattisChildErr
                Quit
            ${EndIf}
        ${Else}
            StrCpy $lattisChildErr 740
        ${EndIf}
        ${IfNot} ${Silent}
            MessageBox MB_OK|MB_ICONSTOP "$(LATTIS_UNINSTALL_REFUSED)"
        ${EndIf}
        SetErrorLevel $lattisChildErr
        Quit
    ${EndIf}

    ${If} $lattisScope == machine
        SetShellVarContext all
    ${Else}
        SetShellVarContext current
    ${EndIf}
FunctionEnd
