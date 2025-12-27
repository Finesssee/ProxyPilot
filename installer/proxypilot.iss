#define AppName "ProxyPilot"
#ifndef AppVersion
  #define AppVersion "0.1.0"
#endif
#ifndef RepoRoot
  #define RepoRoot ".."
#endif
#ifndef OutDir
  #define OutDir "..\\dist"
#endif

[Setup]
AppId={{3D9053A0-3F6A-47D7-9D91-8BB1D1CC2A4E}
AppName={#AppName}
AppVersion={#AppVersion}
AppPublisher=ProxyPilot
AppPublisherURL=https://github.com/Finesssee/ProxyPilot
DefaultDirName={localappdata}\{#AppName}
DefaultGroupName={#AppName}
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
Compression=lzma2/ultra64
SolidCompression=yes
WizardStyle=modern
ArchitecturesInstallIn64BitMode=x64compatible
OutputDir={#OutDir}
OutputBaseFilename=ProxyPilot-{#AppVersion}-Setup
SetupIconFile={#RepoRoot}\static\icon.ico
UninstallDisplayIcon={app}\icon.ico

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "Create a &desktop icon"; GroupDescription: "Additional icons:"
Name: "startupicon"; Description: "Start ProxyPilot when Windows starts"; GroupDescription: "Startup:"

[Files]
; Main app (tray + dashboard UI)
Source: "{#OutDir}\ProxyPilot.exe"; DestDir: "{app}"; Flags: ignoreversion
; Config
Source: "{#RepoRoot}\config.example.yaml"; DestDir: "{app}"; Flags: ignoreversion
; Icons
Source: "{#RepoRoot}\static\icon.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#RepoRoot}\static\icon.png"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\{#AppName}"; Filename: "{app}\ProxyPilot.exe"; WorkingDir: "{app}"; IconFilename: "{app}\icon.ico"
Name: "{autodesktop}\{#AppName}"; Filename: "{app}\ProxyPilot.exe"; Tasks: desktopicon; WorkingDir: "{app}"; IconFilename: "{app}\icon.ico"

[Registry]
Root: HKCU; Subkey: "Software\Microsoft\Windows\CurrentVersion\Run"; ValueType: string; ValueName: "ProxyPilot"; ValueData: """{app}\ProxyPilot.exe"""; Flags: uninsdeletevalue; Tasks: startupicon

[Run]
Filename: "{app}\ProxyPilot.exe"; Description: "Launch {#AppName}"; Flags: nowait postinstall skipifsilent

[UninstallRun]
Filename: "taskkill"; Parameters: "/F /IM ProxyPilot.exe"; Flags: runhidden; RunOnceId: "KillTray"

[UninstallDelete]
Type: filesandordirs; Name: "{app}"

[Code]
procedure CurStepChanged(CurStep: TSetupStep);
var
  ConfigYaml: string;
  ExampleYaml: string;
begin
  if CurStep = ssPostInstall then
  begin
    ConfigYaml := ExpandConstant('{app}\config.yaml');
    ExampleYaml := ExpandConstant('{app}\config.example.yaml');
    if (not FileExists(ConfigYaml)) and FileExists(ExampleYaml) then
    begin
      CopyFile(ExampleYaml, ConfigYaml, False);
    end;
  end;
end;
