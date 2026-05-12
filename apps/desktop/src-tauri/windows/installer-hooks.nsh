!macro KillCialloClawProcess imageName
  !if "${INSTALLMODE}" == "currentUser"
    nsis_tauri_utils::FindProcessCurrentUser "${imageName}"
  !else
    nsis_tauri_utils::FindProcess "${imageName}"
  !endif
  Pop $R0

  ${If} $R0 = 0
    DetailPrint "Stopping ${imageName}"
    !if "${INSTALLMODE}" == "currentUser"
      nsis_tauri_utils::KillProcessCurrentUser "${imageName}"
    !else
      nsis_tauri_utils::KillProcess "${imageName}"
    !endif
    Pop $R0
    Sleep 500
  ${EndIf}
!macroend

!macro StopCialloClawProcesses
  !insertmacro KillCialloClawProcess "cialloclaw-desktop.exe"
  !insertmacro KillCialloClawProcess "cialloclaw-service.exe"
!macroend

!macro NSIS_HOOK_PREINSTALL
  !insertmacro StopCialloClawProcesses
!macroend

!macro NSIS_HOOK_PREUNINSTALL
  !insertmacro StopCialloClawProcesses
!macroend
