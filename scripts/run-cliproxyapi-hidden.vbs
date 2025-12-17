Option Explicit

Dim fso, shell, targetVbs, cmd
Set fso = CreateObject("Scripting.FileSystemObject")
Set shell = CreateObject("WScript.Shell")

targetVbs = fso.BuildPath(fso.GetParentFolderName(WScript.ScriptFullName), "windows\\run-cliproxyapi-hidden.vbs")
cmd = "wscript.exe """ & targetVbs & """"

shell.Run cmd, 0, False

