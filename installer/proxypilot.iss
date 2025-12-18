#define AppName "ProxyPilot"
#ifndef AppVersion
  #define AppVersion "0.0.0-dev"
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
SetupIconFile={#RepoRoot}\\bin\\ProxyPilot.ico
DefaultDirName={localappdata}\\{#AppName}
DefaultGroupName={#AppName}
DisableProgramGroupPage=yes
PrivilegesRequired=lowest
Compression=lzma2
SolidCompression=yes
WizardStyle=modern
ArchitecturesInstallIn64BitMode=x64
OutputDir={#OutDir}
OutputBaseFilename=ProxyPilot-Setup
UninstallDisplayIcon={app}\\ProxyPilot.ico

[Languages]
Name: "english"; MessagesFile: "compiler:Default.isl"

[Tasks]
Name: "desktopicon"; Description: "Create a &desktop icon"; GroupDescription: "Additional icons:"

[Files]
Source: "{#RepoRoot}\\bin\\ProxyPilot.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#RepoRoot}\\bin\\ProxyPilotUI.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#RepoRoot}\\bin\\proxypilot-engine.exe"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#RepoRoot}\\bin\\ProxyPilot.ico"; DestDir: "{app}"; Flags: ignoreversion
Source: "{#RepoRoot}\\config.example.yaml"; DestDir: "{app}"; Flags: ignoreversion

[Icons]
Name: "{autoprograms}\\{#AppName}"; Filename: "{app}\\ProxyPilot.exe"; WorkingDir: "{app}"; IconFilename: "{app}\\ProxyPilot.ico"
Name: "{autodesktop}\\{#AppName}"; Filename: "{app}\\ProxyPilot.exe"; Tasks: desktopicon; WorkingDir: "{app}"; IconFilename: "{app}\\ProxyPilot.ico"

[Run]
Filename: "{app}\\ProxyPilot.exe"; Description: "Launch {#AppName}"; Flags: nowait postinstall skipifsilent

[UninstallRun]
Filename: "cmd.exe"; Parameters: "/c reg delete HKCU\\Software\\Microsoft\\Windows\\CurrentVersion\\Run /v ProxyPilot /f >nul 2>nul"; Flags: runhidden

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
    ConfigYaml := ExpandConstant('{app}\\config.yaml');
    ExampleYaml := ExpandConstant('{app}\\config.example.yaml');
    if (not FileExists(ConfigYaml)) and FileExists(ExampleYaml) then
    begin
      FileCopy(ExampleYaml, ConfigYaml, False);
    end;
  end;
end;
