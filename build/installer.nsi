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
!include Sections.nsh

Name "${APP_NAME}"
OutFile "${ROOT}/${OUTFILE}"

VIProductVersion "${VIVERSION}"
VIAddVersionKey "ProductName" "${APP_NAME}"
VIAddVersionKey "FileDescription" "${APP_NAME} Setup (${ARCH})"
VIAddVersionKey "FileVersion" "${VERSION}"
VIAddVersionKey "ProductVersion" "${VERSION}"
VIAddVersionKey "CompanyName" "${APP_PUBLISHER}"
VIAddVersionKey "LegalCopyright" "Copyright (c) 2024 ${APP_PUBLISHER}"

; Icon.ico badged with NSIS's stock emblem, generated and committed by
; build/gen_setup_ico.sh.
!define MUI_ICON "${ROOT}/build/icons/setup.ico"
!define MUI_UNICON "${ROOT}/build/icons/uninstall.ico"
!define MUI_ABORTWARNING

; Whether the uninstaller should also drop %AppData%\ghfs-gui.
Var DeleteConfig
Var DeleteConfigCheckbox

; Pages. No license page: the app is MIT and the text adds a click for nothing.
; The components page carries the optional tasks and sits after the directory
; page, so both are decided on the last page before any file is copied.
!insertmacro MUI_PAGE_WELCOME
!insertmacro MULTIUSER_PAGE_INSTALLMODE
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_COMPONENTS
!insertmacro MUI_PAGE_INSTFILES

; There is deliberately no "run now" checkbox: an all-users install runs
; elevated, so the file server would inherit administrator rights.
!insertmacro MUI_PAGE_FINISH

!insertmacro MUI_UNPAGE_CONFIRM
UninstPage custom un.ConfigPageShow un.ConfigPageLeave
!insertmacro MUI_UNPAGE_INSTFILES

; English first so it is the fallback; NSIS picks the match for the user's
; system language at runtime, no language prompt.
!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "SimpChinese"

LangString TEXT_SEC_DESKTOP ${LANG_ENGLISH} "Create a desktop shortcut"
LangString TEXT_SEC_DESKTOP ${LANG_SIMPCHINESE} "创建桌面快捷方式"
LangString TEXT_SEC_DESKTOP_DESC ${LANG_ENGLISH} "Put a shortcut to ${APP_NAME} on the desktop."
LangString TEXT_SEC_DESKTOP_DESC ${LANG_SIMPCHINESE} "在桌面上创建 ${APP_NAME} 的快捷方式。"

LangString TEXT_SEC_FIREWALL ${LANG_ENGLISH} "Allow through Windows Firewall"
LangString TEXT_SEC_FIREWALL ${LANG_SIMPCHINESE} "添加 Windows 防火墙允许规则"
LangString TEXT_SEC_FIREWALL_NOADMIN ${LANG_ENGLISH} "Allow through Windows Firewall (requires administrator)"
LangString TEXT_SEC_FIREWALL_NOADMIN ${LANG_SIMPCHINESE} "添加 Windows 防火墙允许规则(需要管理员权限)"
LangString TEXT_SEC_FIREWALL_DESC ${LANG_ENGLISH} "Add an inbound rule for ${APP_EXE} covering private and domain networks, so other machines reach the server without Windows asking first. Public networks are deliberately left out."
LangString TEXT_SEC_FIREWALL_DESC ${LANG_SIMPCHINESE} "为 ${APP_EXE} 添加入站规则,覆盖专用网络和域网络,局域网内的其他设备无需再确认即可访问。公用网络不在其中。"

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

; Both optional tasks are unchecked by default (/o), matching what the finish
; page's checkbox used to do.
Section /o "$(TEXT_SEC_DESKTOP)" SecDesktop
	CreateShortCut "$DESKTOP\${APP_NAME}.lnk" "$INSTDIR\${APP_EXE}"
SectionEnd

Section /o "$(TEXT_SEC_FIREWALL)" SecFirewall
	; A program rule covers every port ghfs may be configured to listen on, so
	; the rule survives a port change in the GUI. The exit code is logged with
	; netsh's own output instead of aborting: the files are already in place by
	; now, and a missing rule only means Windows will ask on first listen.
	nsExec::ExecToLog 'netsh advfirewall firewall add rule name="${APP_NAME}" \
		dir=in action=allow program="$INSTDIR\${APP_EXE}" enable=yes \
		profile=domain,private'
	Pop $0
SectionEnd

!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
	!insertmacro MUI_DESCRIPTION_TEXT ${SecDesktop} "$(TEXT_SEC_DESKTOP_DESC)"
	!insertmacro MUI_DESCRIPTION_TEXT ${SecFirewall} "$(TEXT_SEC_FIREWALL_DESC)"
!insertmacro MUI_FUNCTION_DESCRIPTION_END

; Both init functions sit below the sections because .onInit references
; ${SecFirewall}: a section index only exists from its definition onwards.
Function .onInit
	; The payload is 64-bit, so keep registry writes out of WOW6432Node —
	; otherwise the uninstall entry is invisible in Apps & features.
	!if "${ARCH}" != "386"
		SetRegView 64
	!endif
	Call AbortIfRunning
	!insertmacro MULTIUSER_INIT

	; netsh writes machine-wide rules. An administrator account is already
	; elevated here (MULTIUSER_EXECUTIONLEVEL Highest), a standard user is not
	; and never will be, so grey the task out rather than let it fail silently
	; during the install. $MultiUser.Privileges is set by MULTIUSER_INIT above.
	${If} $MultiUser.Privileges != "Admin"
	${AndIf} $MultiUser.Privileges != "Power"
		SectionSetText ${SecFirewall} "$(TEXT_SEC_FIREWALL_NOADMIN)"
		!insertmacro SetSectionFlag ${SecFirewall} ${SF_RO}
	${EndIf}
FunctionEnd

Function un.onInit
	!if "${ARCH}" != "386"
		SetRegView 64
	!endif
	Call un.AbortIfRunning
	!insertmacro MULTIUSER_UNINIT
FunctionEnd

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
	; Unconditional: nothing records whether the rule was created, and deleting
	; a rule that does not exist is a no-op. Matched on the program path too, so
	; a same-named rule from elsewhere is left alone. Done before the .exe goes,
	; while $INSTDIR still spells what the rule holds.
	nsExec::ExecToLog 'netsh advfirewall firewall delete rule name="${APP_NAME}" \
		program="$INSTDIR\${APP_EXE}"'
	Pop $0

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
