; NSIS installer for the Windows builds. Compiled by build/pack-windows.sh once
; per architecture.
;
; Required -D defines: ARCH, VERSION, VIVERSION, SRCEXE, OUTFILE. SRCEXE and
; OUTFILE are relative to the repository root, like every path here: makensis
; resolves relative paths against the script's own directory, so they all go
; through ${ROOT} instead and the caller's working directory is irrelevant.

Unicode true
ManifestDPIAware true
SetCompressor /SOLID lzma

!ifndef ARCH | VERSION | VIVERSION | SRCEXE | OUTFILE
	!error "ARCH, VERSION, VIVERSION, SRCEXE and OUTFILE must all be -D defined"
!endif

; Repository root, relative to this script's directory.
!define ROOT ".."

!define APP_NAME "Go HTTP File Server GUI"
!define APP_KEY "ghfs-gui"
!define APP_EXE "ghfs-gui.exe"
!define APP_PUBLISHER "MJ PC Lab"
!define APP_URL "https://github.com/mjpclab/go-http-file-server-gui"
!define UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP_KEY}"

; Per-user by default so the common path needs no elevation. Note that
; `Highest` makes an administrator account get one UAC prompt when the
; installer starts, whatever scope is picked afterwards — runtime elevation
; would need the third-party UAC plugin, which base NSIS does not ship.
!define MULTIUSER_EXECUTIONLEVEL Highest
!define MULTIUSER_MUI
!define MULTIUSER_INSTALLMODE_COMMANDLINE
!define MULTIUSER_INSTALLMODE_DEFAULT_CURRENTUSER
!define MULTIUSER_USE_PROGRAMFILES64
!define MULTIUSER_INSTALLMODE_INSTDIR "${APP_KEY}"
!define MULTIUSER_INSTALLMODE_INSTDIR_REGISTRY_KEY "Software\${APP_KEY}"
!define MULTIUSER_INSTALLMODE_INSTDIR_REGISTRY_VALUENAME "InstallDir"

; The same key doubles as the "which scope was used last" marker: the INSTDIR_
; pair only restores the directory *within* a scope, while the DEFAULT_ pair is
; what makes MultiUser.nsh preselect the scope itself. With
; MULTIUSER_INSTALLMODE_DEFAULT_CURRENTUSER set it probes HKCU first and falls
; back to HKLM, so a previous all-users install reopens as all-users and
; everything else stays per-user. It also fixes the uninstaller, which would
; otherwise always assume per-user and leave an all-users install's HKLM
; uninstall entry and common Start Menu shortcut behind.
!define MULTIUSER_INSTALLMODE_DEFAULT_REGISTRY_KEY "Software\${APP_KEY}"
!define MULTIUSER_INSTALLMODE_DEFAULT_REGISTRY_VALUENAME "InstallDir"

!include MultiUser.nsh
!include MUI2.nsh
!include LogicLib.nsh
!include nsDialogs.nsh
!include FileFunc.nsh

Name "${APP_NAME}"
OutFile "${ROOT}/${OUTFILE}"

VIProductVersion "${VIVERSION}"
VIAddVersionKey "ProductName" "${APP_NAME}"
VIAddVersionKey "FileDescription" "${APP_NAME} Setup (${ARCH})"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "ProductVersion" "${VERSION}"
VIAddVersionKey "CompanyName" "${APP_PUBLISHER}"
VIAddVersionKey "LegalCopyright" "Copyright (c) 2024 ${APP_PUBLISHER}"

!define MUI_ICON "${ROOT}/Icon.ico"
!define MUI_UNICON "${ROOT}/Icon.ico"
!define MUI_ABORTWARNING

; Whether the uninstaller should also drop %AppData%\ghfs-gui.
Var DeleteConfig
Var DeleteConfigCheckbox

; Pages. No license page: the app is MIT and the text adds a click for nothing.
!insertmacro MUI_PAGE_WELCOME
!insertmacro MULTIUSER_PAGE_INSTALLMODE
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES

; The finish page's "show readme" checkbox is NSIS's documented hook for an
; optional post-install action. There is deliberately no "run now" checkbox:
; an all-users install runs elevated, so the file server would inherit
; administrator rights.
!define MUI_FINISHPAGE_SHOWREADME ""
!define MUI_FINISHPAGE_SHOWREADME_NOTCHECKED
!define MUI_FINISHPAGE_SHOWREADME_TEXT "$(TEXT_DESKTOP_SHORTCUT)"
!define MUI_FINISHPAGE_SHOWREADME_FUNCTION CreateDesktopShortcut
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
UninstPage custom un.ConfigPageShow un.ConfigPageLeave
!insertmacro MUI_UNPAGE_INSTFILES

; English first so it is the fallback; NSIS picks the match for the user's
; system language at runtime, no language prompt.
!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "SimpChinese"

LangString TEXT_DESKTOP_SHORTCUT ${LANG_ENGLISH} "Create a desktop shortcut"
LangString TEXT_DESKTOP_SHORTCUT ${LANG_SIMPCHINESE} "创建桌面快捷方式"

LangString TEXT_APP_RUNNING ${LANG_ENGLISH} "${APP_NAME} is running. Please close it and run this again."
LangString TEXT_APP_RUNNING ${LANG_SIMPCHINESE} "${APP_NAME} 正在运行,请先关闭它,然后重新运行。"

LangString TEXT_UNCFG_TITLE ${LANG_ENGLISH} "Settings"
LangString TEXT_UNCFG_TITLE ${LANG_SIMPCHINESE} "配置文件"
LangString TEXT_UNCFG_SUBTITLE ${LANG_ENGLISH} "Choose what to do with your saved settings."
LangString TEXT_UNCFG_SUBTITLE ${LANG_SIMPCHINESE} "选择如何处理已保存的配置。"
LangString TEXT_UNCFG_LABEL ${LANG_ENGLISH} "Your settings are stored in $APPDATA\${APP_KEY}. They are kept by default so a later reinstall picks them up again."
LangString TEXT_UNCFG_LABEL ${LANG_SIMPCHINESE} "配置保存在 $APPDATA\${APP_KEY}。默认保留,以便将来重新安装时继续使用。"
LangString TEXT_UNCFG_CHECK ${LANG_ENGLISH} "Also delete my settings"
LangString TEXT_UNCFG_CHECK ${LANG_SIMPCHINESE} "同时删除我的配置"

; Overwriting a running .exe fails on Windows, so bail out early instead of
; half-installing. Defined for both the installer and the uninstaller.
!macro AbortIfRunning UN
Function ${UN}AbortIfRunning
	FindWindow $0 "" "${APP_NAME}"
	${If} $0 <> 0
		MessageBox MB_OK|MB_ICONSTOP "$(TEXT_APP_RUNNING)" /SD IDOK
		Quit
	${EndIf}
FunctionEnd
!macroend
!insertmacro AbortIfRunning ""
!insertmacro AbortIfRunning "un."

Function .onInit
	; The payload is 64-bit, so keep registry writes out of WOW6432Node —
	; otherwise the uninstall entry is invisible in Apps & features.
	!if "${ARCH}" != "386"
		SetRegView 64
	!endif
	Call AbortIfRunning
	!insertmacro MULTIUSER_INIT
FunctionEnd

Function un.onInit
	!if "${ARCH}" != "386"
		SetRegView 64
	!endif
	Call un.AbortIfRunning
	!insertmacro MULTIUSER_UNINIT
FunctionEnd

Function CreateDesktopShortcut
	CreateShortCut "$DESKTOP\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}"
FunctionEnd

Section "-Application"
	SetOutPath "$INSTDIR"
	File "/oname=${APP_EXE}" "${ROOT}/${SRCEXE}"
	WriteUninstaller "$INSTDIR\Uninstall.exe"
	CreateShortCut "$SMPROGRAMS\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}"

	; SHCTX is HKLM for an all-users install, HKCU for per-user; MultiUser.nsh
	; sets it along with the shell folder context.
	WriteRegStr SHCTX "Software\${APP_KEY}" "InstallDir" "$INSTDIR"

	WriteRegStr SHCTX "${UNINST_KEY}" "DisplayName" "${APP_NAME}"
	WriteRegStr SHCTX "${UNINST_KEY}" "DisplayVersion" "${VERSION}"
	WriteRegStr SHCTX "${UNINST_KEY}" "DisplayIcon" "$INSTDIR\${APP_EXE},0"
	WriteRegStr SHCTX "${UNINST_KEY}" "Publisher" "${APP_PUBLISHER}"
	WriteRegStr SHCTX "${UNINST_KEY}" "URLInfoAbout" "${APP_URL}"
	WriteRegStr SHCTX "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
	WriteRegStr SHCTX "${UNINST_KEY}" "UninstallString" '"$INSTDIR\Uninstall.exe"'
	WriteRegStr SHCTX "${UNINST_KEY}" "QuietUninstallString" '"$INSTDIR\Uninstall.exe" /S'
	WriteRegDWORD SHCTX "${UNINST_KEY}" "NoModify" 1
	WriteRegDWORD SHCTX "${UNINST_KEY}" "NoRepair" 1
	${GetSize} "$INSTDIR" "/S=0K" $0 $1 $2
	WriteRegDWORD SHCTX "${UNINST_KEY}" "EstimatedSize" $0
SectionEnd

Function un.ConfigPageShow
	!insertmacro MUI_HEADER_TEXT "$(TEXT_UNCFG_TITLE)" "$(TEXT_UNCFG_SUBTITLE)"
	nsDialogs::Create 1018
	Pop $0
	${If} $0 == error
		Abort
	${EndIf}
	${NSD_CreateLabel} 0 0 100% 32u "$(TEXT_UNCFG_LABEL)"
	Pop $1
	${NSD_CreateCheckbox} 0 36u 100% 12u "$(TEXT_UNCFG_CHECK)"
	Pop $DeleteConfigCheckbox
	nsDialogs::Show
FunctionEnd

Function un.ConfigPageLeave
	${NSD_GetState} $DeleteConfigCheckbox $DeleteConfig
FunctionEnd

Section "Uninstall"
	Delete "$INSTDIR\${APP_EXE}"
	Delete "$INSTDIR\Uninstall.exe"
	RMDir "$INSTDIR"

	Delete "$SMPROGRAMS\${APP_NAME}.lnk"
	Delete "$DESKTOP\${APP_NAME}.lnk"

	DeleteRegKey SHCTX "${UNINST_KEY}"
	DeleteRegKey SHCTX "Software\${APP_KEY}"

	; Skipped on a silent uninstall, where the custom page never runs and
	; $DeleteConfig stays empty.
	${If} $DeleteConfig == ${BST_CHECKED}
		; The preference file is always per-user (%AppData%\Roaming), even for
		; an all-users install, where the context would otherwise be
		; C:\ProgramData. Done last, since it leaves the context switched.
		SetShellVarContext current
		Delete "$APPDATA\${APP_KEY}\preference.json"
		RMDir "$APPDATA\${APP_KEY}"
	${EndIf}
SectionEnd
