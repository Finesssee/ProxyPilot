Option Explicit

' Runs CLIProxyAPI at logon without showing a console window.
' This script delegates to scripts\start-cliproxy.ps1.

Function ParentProcessName()
  On Error Resume Next
  Dim query, wmi, procs, proc
  ParentProcessName = ""
  Set wmi = GetObject("winmgmts:\\\\.\\root\\cimv2")
  query = "Select ProcessId, ParentProcessId, CommandLine From Win32_Process Where Name='wscript.exe'"
  Set procs = wmi.ExecQuery(query)
  For Each proc In procs
    If InStr(LCase(proc.CommandLine), LCase(WScript.ScriptFullName)) > 0 Then
      Dim parentPid, parentQuery, parentProcs, parentProc
      parentPid = proc.ParentProcessId
      parentQuery = "Select Name From Win32_Process Where ProcessId=" & parentPid
      Set parentProcs = wmi.ExecQuery(parentQuery)
      For Each parentProc In parentProcs
        ParentProcessName = LCase(parentProc.Name)
        Exit Function
      Next
    End If
  Next
End Function

Dim shell, fso, scriptDir, ps1Path, cmd
Set shell = CreateObject("WScript.Shell")
Set fso = CreateObject("Scripting.FileSystemObject")

scriptDir = fso.GetParentFolderName(WScript.ScriptFullName)
ps1Path = scriptDir & "\start-cliproxy.ps1"

Dim parentName
parentName = ParentProcessName()

' If this is invoked by Task Scheduler, exit early so we don't spawn an elevated
' server process (which is hard to stop/update). Prefer a HKCU Run entry instead.
If parentName = "taskeng.exe" Or parentName = "taskhostw.exe" Then
  WScript.Quit 0
End If

cmd = "powershell.exe -NoProfile -ExecutionPolicy Bypass -File """ & ps1Path & """"
shell.Run cmd, 0, False
