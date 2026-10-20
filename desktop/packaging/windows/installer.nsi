; Installer of the desktop app for Windows: per user, no administrator rights.
; makensis -DVERSION=0.4.0 -DBINARY=…\wapp-debugger-desktop.exe -DICON=…\icon.ico -DOUTFILE=…\setup.exe installer.nsi
Unicode true
!include "LogicLib.nsh"

!define APP "wapp-debugger"
!define EXE "wapp-debugger-desktop.exe"
!define UNINSTALL_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${APP}"
; WebView2 runtime: the Evergreen client id
!define WEBVIEW2 "{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}"

Name "${APP} ${VERSION}"
OutFile "${OUTFILE}"
Icon "${ICON}"
UninstallIcon "${ICON}"
InstallDir "$LOCALAPPDATA\Programs\${APP}"
RequestExecutionLevel user
SetCompressor /SOLID lzma

Page directory
Page instfiles
UninstPage uninstConfirm
UninstPage instfiles

Section "Install"
  SetOutPath "$INSTDIR"
  File "/oname=${EXE}" "${BINARY}"
  WriteUninstaller "$INSTDIR\uninstall.exe"
  CreateShortCut "$SMPROGRAMS\${APP}.lnk" "$INSTDIR\${EXE}"

  ; wapp-debugger:// links open the app
  WriteRegStr HKCU "Software\Classes\wapp-debugger" "" "URL:wapp-debugger"
  WriteRegStr HKCU "Software\Classes\wapp-debugger" "URL Protocol" ""
  WriteRegStr HKCU "Software\Classes\wapp-debugger\DefaultIcon" "" "$INSTDIR\${EXE},0"
  WriteRegStr HKCU "Software\Classes\wapp-debugger\shell\open\command" "" '"$INSTDIR\${EXE}" "%1"'

  ; Apps and features
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayName" "${APP}"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayVersion" "${VERSION}"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "Publisher" "hightemp"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "DisplayIcon" "$INSTDIR\${EXE},0"
  WriteRegStr HKCU "${UNINSTALL_KEY}" "UninstallString" '"$INSTDIR\uninstall.exe"'
  WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoModify" 1
  WriteRegDWORD HKCU "${UNINSTALL_KEY}" "NoRepair" 1

  ; Windows 10 may lack WebView2, which draws the window
  ReadRegStr $0 HKLM "SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\${WEBVIEW2}" "pv"
  ${If} $0 == ""
    ReadRegStr $0 HKCU "Software\Microsoft\EdgeUpdate\Clients\${WEBVIEW2}" "pv"
  ${EndIf}
  ${If} $0 == ""
    MessageBox MB_YESNO "${APP} needs the Microsoft Edge WebView2 Runtime, which is not installed. Open its download page?" IDNO done
    ExecShell "open" "https://developer.microsoft.com/microsoft-edge/webview2/"
    done:
  ${EndIf}
SectionEnd

Section "Uninstall"
  Delete "$INSTDIR\${EXE}"
  Delete "$INSTDIR\uninstall.exe"
  RMDir "$INSTDIR"
  Delete "$SMPROGRAMS\${APP}.lnk"
  DeleteRegKey HKCU "Software\Classes\wapp-debugger"
  DeleteRegKey HKCU "${UNINSTALL_KEY}"
  ; Autostart set from the app's settings
  DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "${APP}"
SectionEnd
