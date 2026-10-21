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
;
; Une seule installation par utilisateur (jamais deux entrees « Lattis »
; dans Applications installees) : l'installation de l'autre portee est
; retiree par l'operation (lattis.findOther), apres confirmation avec
; l'interface (« Lattis X est deja installe pour ... Il sera remplace. ») :
;   - pour l'utilisateur, l'installation pour tous est retiree AVANT la copie
;     des fichiers (lattis.replaceMachine) : sans droits, par une relance
;     elevee de cet installeur avec /REMOVE=machine ; refusee, rien n'est
;     installe si ce serait une seconde installation (sinon : mise a jour
;     sur place de l'installation pour l'utilisateur, l'application
;     proposera de retirer l'autre) ;
;   - pour tous, l'installation pour l'utilisateur est retiree APRES
;     (lattis.replaceUser), dans le compte de l'utilisateur : par cette
;     instance si elle a deja les droits, sinon par l'instance non elevee
;     une fois l'instance elevee terminee (lattis.afterElevatedInstall ;
;     l'instance elevee peut tourner sous un autre compte, dont HKCU n'est
;     pas celui de l'utilisateur) ;
;   - en silencieux (/S, mise a jour par l'application), l'installation
;     existante est mise a jour sur place, jamais une seconde creee ; l'autre
;     n'est retiree que sans invite UAC (sauf /CurrentUser sans installation
;     pour l'utilisateur, qui la demande).
; Les restes sans desinstalleur ni cle (Program Files\Lattis\Lattis\lattis.exe
; d'une installation ancienne) sont retires quand l'installeur en a le droit
; (lattis.cleanLeftovers), fichiers connus seulement.
; Parametre interne : /REMOVE=machine|user (instance elevee : retire cette
; installation, silencieusement, et rend 0 si elle n'est plus inscrite).

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
Var lattisUserVer       ; version de l'installation pour l'utilisateur (DisplayVersion)
Var lattisMachineVer    ; version de l'installation pour tous
Var lattisOtherScope    ; installation de l'autre portee, a retirer ("" : aucune)
Var lattisKeepData      ; desinstalleur : garder les donnees WebView2 (/KEEPDATA)
Var lattisFailStatus    ; statut a ecrire par .onInstFailed (failed sinon, project.nsi)
; lattisOtherVer (« 0.3.3-beta » precede d'une espace) : lattis_lang.nsh.

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
    ReadRegStr $lattisUserVer HKCU "${UNINST_KEY}" "DisplayVersion"
    ReadRegStr $lattisMachineVer HKLM "${UNINST_KEY}" "DisplayVersion"
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

; Attend que le lattis.exe du dossier sur la pile soit libre (30 s au plus) :
; Lattis lance la mise a jour en se fermant.
Function lattis.waitFree
    Exch $0
    Push $1
    Push $2
    StrCpy $1 0
    ${Do}
        ${IfNot} ${FileExists} "$0\${PRODUCT_EXECUTABLE}"
            ${Break}
        ${EndIf}
        ClearErrors
        FileOpen $2 "$0\${PRODUCT_EXECUTABLE}" a
        ${IfNot} ${Errors}
            FileClose $2
            ${Break}
        ${EndIf}
        IntOp $1 $1 + 1
        ${If} $1 >= 60
            ${Break}
        ${EndIf}
        Sleep 500
    ${Loop}
    Pop $2
    Pop $1
    Pop $0
FunctionEnd

; Fichiers connus d'un dossier Lattis sans desinstalleur : l'executable et
; les restes d'une mise a jour, jamais autre chose ; puis le dossier s'il
; est vide.
Function lattis.cleanDir
    Exch $0
    Delete "$0\${PRODUCT_EXECUTABLE}"
    Delete "$0\${PRODUCT_EXECUTABLE}.old"
    Delete "$0\${PRODUCT_EXECUTABLE}.new"
    RMDir "$0"
    Pop $0
FunctionEnd

; Reste d'installation au dossier sur la pile : lattis.exe sans
; desinstalleur, dossier ni inscrit ni celui de cette installation.
Function lattis.cleanLeftoverAt
    Exch $0
    ${If} $0 != $INSTDIR
    ${AndIf} $0 != $lattisUserDir
    ${AndIf} $0 != $lattisMachineDir
    ${AndIf} ${FileExists} "$0\${PRODUCT_EXECUTABLE}"
    ${AndIfNot} ${FileExists} "$0\uninstall.exe"
        DetailPrint "$(LATTIS_LEFTOVER)$0"
        Push $0
        Call lattis.cleanDir
    ${EndIf}
    Pop $0
FunctionEnd

; Restes des dossiers par defaut (Program Files\Lattis\Lattis,
; %LOCALAPPDATA%\Programs\Lattis), retires quand l'installeur en a le droit
; (Program Files : avec les droits d'administration seulement).
Function lattis.cleanLeftovers
    Push "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
    Call lattis.cleanLeftoverAt
    RMDir "$PROGRAMFILES64\${INFO_COMPANYNAME}"
    SetShellVarContext current
    Push "$LOCALAPPDATA\Programs\${INFO_PRODUCTNAME}"
    Call lattis.cleanLeftoverAt
    !insertmacro wails.setShellContext
FunctionEnd

; Retire l'installation de la portee sur la pile (user ou machine), dans ce
; processus (machine : il doit avoir les droits d'administration) : son
; desinstalleur, silencieux, sinon (cle orpheline, desinstalleur absent) sa
; cle et ses fichiers connus. Les donnees WebView2 de l'utilisateur
; (%APPDATA%\lattis.exe), communes a toutes ses installations, sont mises de
; cote pendant le retrait d'une installation pour l'utilisateur : ses
; desinstalleurs jusqu'a la 0.3.5-beta les effacent sans condition ; si
; c'est impossible (Lattis ouvert), rien n'est retire.
; Rend 1 si la portee n'a plus d'installation inscrite, 0 sinon.
Function lattis.removeInstall
    Exch $0 ; portee
    Push $1 ; dossier
    Push $2 ; donnees WebView2 mises de cote
    Push $3 ; resultat
    StrCpy $2 ""
    StrCpy $3 0
    ${If} $0 == machine
        StrCpy $1 $lattisMachineDir
    ${Else}
        StrCpy $1 $lattisUserDir
    ${EndIf}
    ${If} $1 == ""
        StrCpy $3 1
        Goto lattis_remove_done
    ${EndIf}
    DetailPrint "$(LATTIS_REMOVING)$1"
    Push $1
    Call lattis.waitFree
    ${If} $0 == user
        SetShellVarContext current
        ${If} ${FileExists} "$APPDATA\${PRODUCT_EXECUTABLE}\*.*"
            StrCpy $2 "$APPDATA\${PRODUCT_EXECUTABLE}"
            ClearErrors
            Rename "$2" "$2.lattis-keep"
            ${If} ${Errors}
                DetailPrint "$(LATTIS_REMOVE_KEPT)"
                StrCpy $2 ""
                Goto lattis_remove_done
            ${EndIf}
        ${EndIf}
    ${EndIf}
    ${If} ${FileExists} "$1\uninstall.exe"
        ; _?= : le desinstalleur tourne sur place, attendu ; il ne se retire
        ; pas lui-meme : retire ensuite s'il a abouti (cle partie).
        ExecWait '"$1\uninstall.exe" /S /KEEPDATA _?=$1'
        Call lattis.readInstalls
        ${If} $0 == machine
            StrCpy $3 $lattisMachineDir
        ${Else}
            StrCpy $3 $lattisUserDir
        ${EndIf}
        ${If} $3 == ""
            Delete "$1\uninstall.exe"
            RMDir "$1"
        ${EndIf}
        StrCpy $3 0
    ${EndIf}
    Call lattis.readInstalls
    ${If} $0 == machine
        StrCpy $1 $lattisMachineDir
    ${Else}
        StrCpy $1 $lattisUserDir
    ${EndIf}
    ${If} $1 != ""
    ${AndIfNot} ${FileExists} "$1\uninstall.exe"
        ; Cle orpheline (pas de desinstalleur) : la cle et les fichiers
        ; connus du dossier.
        Push $1
        Call lattis.cleanDir
        SetRegView 64
        ${If} $0 == machine
            DeleteRegKey HKLM "${UNINST_KEY}"
        ${Else}
            DeleteRegKey HKCU "${UNINST_KEY}"
        ${EndIf}
        Call lattis.readInstalls
        ${If} $0 == machine
            StrCpy $1 $lattisMachineDir
        ${Else}
            StrCpy $1 $lattisUserDir
        ${EndIf}
    ${EndIf}
    ${If} $1 == ""
        StrCpy $3 1
    ${EndIf}
    lattis_remove_done:
    ${If} $2 != ""
        ClearErrors
        Rename "$2.lattis-keep" "$2"
        ${If} ${Errors}
            DetailPrint "$(LATTIS_LEFTOVER)$2.lattis-keep"
        ${EndIf}
    ${EndIf}
    !insertmacro wails.setShellContext
    StrCpy $0 $3
    Pop $3
    Pop $2
    Pop $1
    Exch $0
FunctionEnd

; Retire l'installation pour tous depuis une instance sans droits : relance
; elevee de cet installeur avec /REMOVE=machine (invite UAC), attendue.
; Rend 1 si elle n'est plus inscrite, 0 sinon (refus de l'UAC compris).
Function lattis.removeMachineElevated
    Push $0
    StrCpy $lattisChildParams "/S /REMOVE=machine /LANGID=$LANGUAGE"
    Call lattis.runElevated
    ${If} $lattisChildProc != 0
        Call lattis.waitChild
    ${EndIf}
    Call lattis.readInstalls
    StrCpy $0 0
    ${If} $lattisMachineDir == ""
        StrCpy $0 1
    ${EndIf}
    Exch $0
FunctionEnd

; L'installation de l'autre portee que $lattisScope : $lattisOtherScope
; (vide : aucune) et $lattisOtherVer, pour les messages.
Function lattis.findOther
    StrCpy $lattisOtherScope ""
    StrCpy $lattisOtherVer ""
    ${If} $lattisScope == user
    ${AndIf} $lattisMachineDir != ""
        StrCpy $lattisOtherScope machine
        StrCpy $lattisOtherVer $lattisMachineVer
    ${ElseIf} $lattisScope == machine
    ${AndIf} $lattisUserDir != ""
        StrCpy $lattisOtherScope user
        StrCpy $lattisOtherVer $lattisUserVer
    ${EndIf}
    ; Le meme dossier inscrit des deux cotes : une seule installation.
    ${If} $lattisUserDir == $lattisMachineDir
        StrCpy $lattisOtherScope ""
    ${EndIf}
    ${If} $lattisOtherVer != ""
        StrCpy $lattisOtherVer " $lattisOtherVer"
    ${EndIf}
FunctionEnd

; Avec l'interface : « Lattis X est deja installe pour ... Il sera
; remplace. » (pas dans l'instance elevee : celle qui l'a lancee a demande).
; Rend 1 pour continuer, 0 si l'utilisateur annule.
Function lattis.confirmOther
    Push $0
    StrCpy $0 1
    ${If} $lattisOtherScope != ""
    ${AndIfNot} ${Silent}
    ${AndIf} $lattisElevated != 1
        ${If} $lattisOtherScope == machine
            MessageBox MB_OKCANCEL|MB_ICONEXCLAMATION "$(LATTIS_REPLACE_MACHINE)" IDOK lattis_confirm_ok
        ${Else}
            MessageBox MB_OKCANCEL|MB_ICONEXCLAMATION "$(LATTIS_REPLACE_USER)" IDOK lattis_confirm_ok
        ${EndIf}
        StrCpy $0 0
        lattis_confirm_ok:
    ${EndIf}
    Exch $0
FunctionEnd

; Installation pour l'utilisateur : l'installation pour tous est retiree
; avant la copie des fichiers. Sans droits, invite UAC (relance elevee),
; sauf en silencieux quand l'installation pour l'utilisateur existe deja
; (mise a jour sur place : l'application proposera de retirer l'autre).
; Refus ou echec : rien n'est installe si ce serait une seconde
; installation, sinon la mise a jour continue.
Function lattis.replaceMachine
    Push $0
    ${If} $lattisOtherScope == machine
        StrCpy $0 0
        ${If} $lattisIsAdmin == 1
            Push machine
            Call lattis.removeInstall
            Pop $0
            Call lattis.cleanLeftovers
        ${ElseIf} ${Silent}
        ${AndIf} $lattisUserDir != ""
            StrCpy $0 2
        ${Else}
            Call lattis.removeMachineElevated
            Pop $0
        ${EndIf}
        ${If} $0 == 0
            ${If} $lattisUserDir == ""
                MessageBox MB_OK|MB_ICONSTOP "$(LATTIS_REMOVE_REFUSED)" /SD IDOK
                StrCpy $lattisFailStatus elevation
                Pop $0
                Abort
            ${EndIf}
            DetailPrint "$(LATTIS_REMOVE_KEPT)"
        ${ElseIf} $0 == 1
            StrCpy $lattisOtherScope ""
        ${EndIf}
    ${EndIf}
    Pop $0
FunctionEnd

; Installation pour tous : l'installation pour l'utilisateur est retiree
; apres, si cette instance est dans le compte de l'utilisateur (pas
; l'instance elevee par l'installeur : lattis.afterElevatedInstall).
Function lattis.replaceUser
    Push $0
    ${If} $lattisOtherScope == user
    ${AndIf} $lattisElevated != 1
        Push user
        Call lattis.removeInstall
        Pop $0
        ${If} $0 == 1
            StrCpy $lattisOtherScope ""
            ; Liens lattis:// : les desinstalleurs jusqu'a la 0.3.5-beta
            ; retirent ceux de l'utilisateur, quelle que soit leur cible.
            !insertmacro wails.associateCustomProtocols
        ${Else}
            DetailPrint "$(LATTIS_REMOVE_KEPT)"
        ${EndIf}
    ${EndIf}
    Pop $0
FunctionEnd

; Instance non elevee, une fois l'installation pour tous faite par
; l'instance elevee : l'installation pour l'utilisateur de CE compte est
; retiree ici (l'instance elevee, peut-etre sous un autre compte, ne la
; voit pas toujours).
Function lattis.afterElevatedInstall
    Push $0
    Call lattis.readInstalls
    ${If} $lattisUserDir != ""
    ${AndIf} $lattisMachineDir != ""
    ${AndIf} $lattisUserDir != $lattisMachineDir
        StrCpy $lattisScope machine
        Push user
        Call lattis.removeInstall
        Pop $0
        ${If} $0 == 1
            StrCpy $INSTDIR $lattisMachineDir
            !insertmacro wails.associateCustomProtocols
        ${EndIf}
    ${EndIf}
    Pop $0
FunctionEnd

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
    ${If} $lattisChildErr == 0
        Call lattis.afterElevatedInstall
    ${EndIf}
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

    ; Instance elevee par lattis.removeMachineElevated : retire
    ; l'installation demandee, rend 0 si elle n'est plus inscrite.
    ClearErrors
    ${GetOptions} $R0 "/REMOVE=" $R1
    ${IfNot} ${Errors}
        StrCpy $R2 0
        ${If} $R1 == machine
        ${OrIf} $R1 == user
            Push $R1
            Call lattis.removeInstall
            Pop $R2
            Call lattis.cleanLeftovers
        ${EndIf}
        ${If} $R2 == 1
            SetErrorLevel 0
        ${Else}
            SetErrorLevel 1
        ${EndIf}
        Quit
    ${EndIf}

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

    ; Silencieux, sans portee imposee : l'installation existante est mise a
    ; jour sur place, jamais une seconde creee (meme avec un /D= nouveau).
    ${If} ${Silent}
    ${AndIf} $lattisScopeForced != 1
        ${If} $lattisScope == user
        ${AndIf} $lattisUserDir == ""
        ${AndIf} $lattisMachineDir != ""
            StrCpy $lattisScope machine
            StrCpy $lattisCmdDir ""
        ${ElseIf} $lattisScope == machine
        ${AndIf} $lattisMachineDir == ""
        ${AndIf} $lattisUserDir != ""
            StrCpy $lattisScope user
            StrCpy $lattisCmdDir ""
        ${EndIf}
    ${EndIf}

    ; L'installation de l'autre portee, remplacee ; portee imposee avec
    ; l'interface (pas de page de choix) : confirmation ici.
    Call lattis.findOther
    ${If} $lattisScopeForced == 1
        Call lattis.confirmOther
        Pop $R2
        ${If} $R2 != 1
            Quit
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
            MessageBox MB_OK|MB_ICONSTOP "$(LATTIS_ELEVATION_REFUSED)" /SD IDOK
        ${EndIf}
        Push $lattisChildErr
        Push elevation
        Call lattis.writeResult
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

    ; Jamais deux installations : celle de l'autre portee sera remplacee.
    Call lattis.findOther
    Call lattis.confirmOther
    Pop $0
    ${If} $0 != 1
        Abort
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
    ; /KEEPDATA : retrait par l'installeur ou l'application (autre
    ; installation gardee), les donnees WebView2 restent.
    StrCpy $lattisKeepData 0
    ClearErrors
    ${GetOptions} $R0 "/KEEPDATA" $R1
    ${IfNot} ${Errors}
        StrCpy $lattisKeepData 1
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

    ; Une autre installation reste (l'autre portee) : ses donnees WebView2,
    ; communes, sont gardees.
    ${If} $lattisScope == machine
    ${AndIf} $lattisUserDir != ""
        StrCpy $lattisKeepData 1
    ${ElseIf} $lattisScope == user
    ${AndIf} $lattisMachineDir != ""
        StrCpy $lattisKeepData 1
    ${EndIf}

    ${If} $lattisScope == machine
    ${AndIf} $lattisIsAdmin != 1
        ${If} $lattisElevated != 1
            ; _?= en dernier : le desinstalleur relance ne se recopie pas.
            StrCpy $lattisChildParams "/ELEVATED /LANGID=$LANGUAGE _?=$INSTDIR"
            ${If} $lattisKeepData == 1
                StrCpy $lattisChildParams "/KEEPDATA $lattisChildParams"
            ${EndIf}
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
            MessageBox MB_OK|MB_ICONSTOP "$(LATTIS_UNINSTALL_REFUSED)" /SD IDOK
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
